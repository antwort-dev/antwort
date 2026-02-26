package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sandboxv1alpha1 "sigs.k8s.io/agent-sandbox/api/v1alpha1"
	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme, err := NewScheme()
	if err != nil {
		t.Fatalf("NewScheme: %v", err)
	}
	return scheme
}

// simulateReady creates a Sandbox resource with Ready=True for the given claim name.
// This simulates what the agent-sandbox controller does when a SandboxClaim is created.
func simulateReady(t *testing.T, c client.Client, name, namespace, fqdn string) {
	t.Helper()
	sandbox := &sandboxv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: sandboxv1alpha1.SandboxStatus{
			ServiceFQDN: fqdn,
			Conditions: []metav1.Condition{
				{
					Type:               string(sandboxv1alpha1.SandboxConditionReady),
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "Ready",
				},
			},
		},
	}
	if err := c.Create(context.Background(), sandbox); err != nil {
		t.Fatalf("simulateReady: create sandbox: %v", err)
	}
	sandbox.Status.ServiceFQDN = fqdn
	sandbox.Status.Conditions = []metav1.Condition{
		{
			Type:               string(sandboxv1alpha1.SandboxConditionReady),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Ready",
		},
	}
	if err := c.Status().Update(context.Background(), sandbox); err != nil {
		t.Fatalf("simulateReady: update status: %v", err)
	}
}

func TestClaimAcquirer_AcquireAndRelease(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sandboxv1alpha1.Sandbox{}).Build()

	acquirer := NewClaimAcquirer(c, "test-template", "default", 5*time.Second)

	// Simulate the controller creating a ready Sandbox in the background.
	// We need to know the claim name, so we override generateClaimName for determinism.
	origGen := generateClaimNameFn
	generateClaimNameFn = func() string { return "test-claim-001" }
	defer func() { generateClaimNameFn = origGen }()

	go func() {
		time.Sleep(200 * time.Millisecond)
		simulateReady(t, c, "test-claim-001", "default", "sandbox-001.default.svc.cluster.local")
	}()

	url, release, err := acquirer.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	if url != "http://sandbox-001.default.svc.cluster.local:8080" {
		t.Errorf("url = %q, want http://sandbox-001.default.svc.cluster.local:8080", url)
	}

	// Verify SandboxClaim was created.
	claim := &extensionsv1alpha1.SandboxClaim{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "test-claim-001", Namespace: "default"}, claim); err != nil {
		t.Fatalf("SandboxClaim not found: %v", err)
	}
	if claim.Spec.TemplateRef.Name != "test-template" {
		t.Errorf("templateRef = %q, want %q", claim.Spec.TemplateRef.Name, "test-template")
	}

	// Release should delete the claim.
	release()

	err = c.Get(context.Background(), client.ObjectKey{Name: "test-claim-001", Namespace: "default"}, claim)
	if err == nil {
		t.Error("SandboxClaim still exists after release, expected deletion")
	}
}

func TestClaimAcquirer_Timeout(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sandboxv1alpha1.Sandbox{}).Build()

	acquirer := NewClaimAcquirer(c, "test-template", "default", 1*time.Second)

	origGen := generateClaimNameFn
	generateClaimNameFn = func() string { return "test-claim-timeout" }
	defer func() { generateClaimNameFn = origGen }()

	// Don't create a Sandbox, so the acquirer will timeout.
	_, _, err := acquirer.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Verify the claim was cleaned up despite the timeout.
	claim := &extensionsv1alpha1.SandboxClaim{}
	getErr := c.Get(context.Background(), client.ObjectKey{Name: "test-claim-timeout", Namespace: "default"}, claim)
	if getErr == nil {
		t.Error("SandboxClaim still exists after timeout, expected cleanup")
	}
}

func TestClaimAcquirer_ContextCancelled(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sandboxv1alpha1.Sandbox{}).Build()

	acquirer := NewClaimAcquirer(c, "test-template", "default", 30*time.Second)

	origGen := generateClaimNameFn
	generateClaimNameFn = func() string { return "test-claim-cancel" }
	defer func() { generateClaimNameFn = origGen }()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	_, _, err := acquirer.Acquire(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	// Verify the claim was cleaned up.
	claim := &extensionsv1alpha1.SandboxClaim{}
	getErr := c.Get(context.Background(), client.ObjectKey{Name: "test-claim-cancel", Namespace: "default"}, claim)
	if getErr == nil {
		t.Error("SandboxClaim still exists after context cancel, expected cleanup")
	}
}

func TestClaimAcquirer_ConcurrentAcquisitions(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&sandboxv1alpha1.Sandbox{}).Build()

	acquirer := NewClaimAcquirer(c, "test-template", "default", 5*time.Second)

	// Use a counter for deterministic names.
	var mu sync.Mutex
	counter := 0
	origGen := generateClaimNameFn
	generateClaimNameFn = func() string {
		mu.Lock()
		defer mu.Unlock()
		counter++
		return fmt.Sprintf("concurrent-claim-%d", counter)
	}
	defer func() { generateClaimNameFn = origGen }()

	const n = 3
	var wg sync.WaitGroup
	errors := make([]error, n)
	urls := make([]string, n)
	releases := make([]func(), n)

	// Simulate controller creating sandboxes for each claim.
	go func() {
		time.Sleep(200 * time.Millisecond)
		for i := 1; i <= n; i++ {
			name := fmt.Sprintf("concurrent-claim-%d", i)
			fqdn := fmt.Sprintf("sandbox-%d.default.svc.cluster.local", i)
			simulateReady(t, c, name, "default", fqdn)
		}
	}()

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			url, release, err := acquirer.Acquire(context.Background())
			urls[idx] = url
			releases[idx] = release
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < n; i++ {
		if errors[i] != nil {
			t.Errorf("goroutine %d: Acquire failed: %v", i, errors[i])
			continue
		}
		if urls[i] == "" {
			t.Errorf("goroutine %d: got empty URL", i)
		}
		// Release each.
		if releases[i] != nil {
			releases[i]()
		}
	}
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       bool
	}{
		{
			name:       "no conditions",
			conditions: nil,
			want:       false,
		},
		{
			name: "ready true",
			conditions: []metav1.Condition{
				{Type: string(sandboxv1alpha1.SandboxConditionReady), Status: metav1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "ready false",
			conditions: []metav1.Condition{
				{Type: string(sandboxv1alpha1.SandboxConditionReady), Status: metav1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "other condition only",
			conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox := &sandboxv1alpha1.Sandbox{
				Status: sandboxv1alpha1.SandboxStatus{
					Conditions: tt.conditions,
				},
			}
			if got := isReady(sandbox); got != tt.want {
				t.Errorf("isReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
