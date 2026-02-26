// Package kubernetes provides a SandboxAcquirer implementation that manages
// sandbox pods through agent-sandbox SandboxClaim CRDs.
package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sandboxv1alpha1 "sigs.k8s.io/agent-sandbox/api/v1alpha1"
	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"

	"github.com/rhuss/antwort/pkg/tools/builtins/codeinterpreter"
)

// Ensure claimAcquirer implements SandboxAcquirer.
var _ codeinterpreter.SandboxAcquirer = (*ClaimAcquirer)(nil)

// ClaimAcquirer implements SandboxAcquirer by creating and deleting SandboxClaim CRDs.
// Each call to Acquire creates a SandboxClaim, waits for the corresponding Sandbox
// to become ready, and returns the Sandbox's serviceFQDN as the sandbox URL.
type ClaimAcquirer struct {
	client    client.Client
	template  string
	namespace string
	timeout   time.Duration
}

// NewClaimAcquirer creates a ClaimAcquirer from configuration.
func NewClaimAcquirer(c client.Client, template, namespace string, timeout time.Duration) *ClaimAcquirer {
	return &ClaimAcquirer{
		client:    c,
		template:  template,
		namespace: namespace,
		timeout:   timeout,
	}
}

// NewScheme returns a runtime.Scheme with the agent-sandbox types registered.
func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := sandboxv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register sandbox types: %w", err)
	}
	if err := extensionsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register extensions types: %w", err)
	}
	return scheme, nil
}

// Acquire creates a SandboxClaim, waits for the Sandbox to become ready,
// and returns the sandbox URL (http://<serviceFQDN>:8080) along with a
// release function that deletes the claim.
func (a *ClaimAcquirer) Acquire(ctx context.Context) (string, func(), error) {
	claimName := generateClaimNameFn()

	claim := &extensionsv1alpha1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: a.namespace,
		},
		Spec: extensionsv1alpha1.SandboxClaimSpec{
			TemplateRef: extensionsv1alpha1.SandboxTemplateRef{
				Name: a.template,
			},
		},
	}

	if err := a.client.Create(ctx, claim); err != nil {
		return "", nil, fmt.Errorf("create SandboxClaim %q: %w", claimName, err)
	}

	slog.Debug("created SandboxClaim", "name", claimName, "namespace", a.namespace, "template", a.template)

	// Wait for the Sandbox to become ready.
	serviceFQDN, err := a.waitForReady(ctx, claimName)
	if err != nil {
		// Clean up the claim on error.
		a.deleteClaim(context.Background(), claimName)
		return "", nil, err
	}

	sandboxURL := fmt.Sprintf("http://%s:8080", serviceFQDN)

	release := func() {
		a.deleteClaim(context.Background(), claimName)
	}

	slog.Debug("sandbox acquired", "name", claimName, "url", sandboxURL)
	return sandboxURL, release, nil
}

// waitForReady polls the Sandbox resource until its Ready condition is True
// or the timeout expires.
func (a *ClaimAcquirer) waitForReady(ctx context.Context, sandboxName string) (string, error) {
	deadline := time.After(a.timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled waiting for Sandbox %q: %w", sandboxName, ctx.Err())
		case <-deadline:
			return "", fmt.Errorf("timeout waiting for Sandbox %q to become ready (waited %s)", sandboxName, a.timeout)
		case <-ticker.C:
			sandbox := &sandboxv1alpha1.Sandbox{}
			key := types.NamespacedName{Name: sandboxName, Namespace: a.namespace}
			if err := a.client.Get(ctx, key, sandbox); err != nil {
				// Sandbox may not exist yet (controller hasn't created it). Keep polling.
				slog.Debug("waiting for Sandbox", "name", sandboxName, "error", err.Error())
				continue
			}

			if isReady(sandbox) {
				if sandbox.Status.ServiceFQDN == "" {
					continue // Ready but FQDN not yet populated.
				}
				return sandbox.Status.ServiceFQDN, nil
			}
		}
	}
}

// isReady checks if the Sandbox has a Ready condition set to True.
func isReady(sandbox *sandboxv1alpha1.Sandbox) bool {
	for _, c := range sandbox.Status.Conditions {
		if c.Type == string(sandboxv1alpha1.SandboxConditionReady) && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// deleteClaim deletes a SandboxClaim. Errors are logged but not returned
// since this is called from release functions and cleanup paths.
func (a *ClaimAcquirer) deleteClaim(ctx context.Context, name string) {
	claim := &extensionsv1alpha1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: a.namespace,
		},
	}
	if err := a.client.Delete(ctx, claim); err != nil {
		slog.Warn("failed to delete SandboxClaim", "name", name, "namespace", a.namespace, "error", err.Error())
		return
	}
	slog.Debug("deleted SandboxClaim", "name", name, "namespace", a.namespace)
}

// generateClaimNameFn creates a unique name for a SandboxClaim.
// Replaceable in tests for deterministic naming.
var generateClaimNameFn = func() string {
	return fmt.Sprintf("antwort-ci-%d", time.Now().UnixNano())
}
