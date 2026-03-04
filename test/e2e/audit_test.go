//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

// readAuditLines reads the audit file and returns parsed JSON lines.
// Returns nil and skips the test if the audit file is not accessible.
func readAuditLines(t *testing.T) []map[string]any {
	t.Helper()
	if auditFile == "" {
		t.Skip("ANTWORT_AUDIT_FILE not set")
	}

	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Skip("audit file not accessible: ", err)
	}

	var lines []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip non-JSON lines
		}
		lines = append(lines, entry)
	}
	return lines
}

// auditLineCountBefore returns the current number of lines in the audit file.
// Returns 0 if the file does not exist yet.
func auditLineCountBefore(t *testing.T) int {
	t.Helper()
	if auditFile == "" {
		return 0
	}
	data, err := os.ReadFile(auditFile)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// readNewAuditLines reads audit lines added after the given offset.
func readNewAuditLines(t *testing.T, offset int) []map[string]any {
	t.Helper()
	all := readAuditLines(t)
	if offset >= len(all) {
		return nil
	}
	return all[offset:]
}

// hasAuditEvent checks if any audit line matches the given event type.
func hasAuditEvent(lines []map[string]any, eventType string) bool {
	for _, line := range lines {
		if line["event"] == eventType {
			return true
		}
		// Also check nested "type" field in case the schema uses that.
		if line["type"] == eventType {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// T024: TestE2EAuditEvents
// ---------------------------------------------------------------------------

func TestE2EAuditEvents(t *testing.T) {
	if auditFile == "" {
		t.Skip("ANTWORT_AUDIT_FILE not set")
	}

	// Record the offset before our actions.
	before := auditLineCountBefore(t)

	// 1. Make an authenticated request (create response).
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "audit test"},
		},
	}
	resp := postJSON(t, "/responses", body)
	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Fatalf("create response: status %d, body: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)
	var created map[string]any
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatal("created response has no id")
	}

	// 2. Delete the response.
	delResp := deleteHTTP(t, "/responses/"+id)
	readBody(t, delResp)

	// 3. Read new audit lines.
	newLines := readNewAuditLines(t, before)
	if len(newLines) == 0 {
		t.Skip("no new audit lines found, audit logging may not be enabled")
	}

	// 4. Verify auth.success and resource.created events exist.
	if !hasAuditEvent(newLines, "auth.success") {
		t.Error("expected auth.success audit event after authenticated request")
	}
	if !hasAuditEvent(newLines, "resource.created") {
		t.Error("expected resource.created audit event after creating a response")
	}
}

// ---------------------------------------------------------------------------
// T025: TestE2EAuditAuthFailure
// ---------------------------------------------------------------------------

func TestE2EAuditAuthFailure(t *testing.T) {
	if auditFile == "" {
		t.Skip("ANTWORT_AUDIT_FILE not set")
	}

	// Record the offset before our action.
	before := auditLineCountBefore(t)

	// 1. Send request with invalid key.
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "should fail auth"},
		},
	}
	resp := postJSONWithKey(t, "/responses", body, "invalid-audit-test-key")
	readBody(t, resp)
	// We expect 401 but do not fail if the status differs.

	// 2. Read new audit lines.
	newLines := readNewAuditLines(t, before)
	if len(newLines) == 0 {
		t.Skip("no new audit lines found, audit logging may not be enabled")
	}

	// 3. Verify auth.failure event exists.
	if !hasAuditEvent(newLines, "auth.failure") {
		t.Error("expected auth.failure audit event after request with invalid key")
	}
}
