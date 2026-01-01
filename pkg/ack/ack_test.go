package ack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestAccept(t *testing.T) {
	// Create temp directory with .beads structure
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Write test issue
	issue := model.Issue{
		ID:        "test-123",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	// Run accept
	svc := NewService(tmpDir)
	result, err := svc.Accept("test-123", "test-agent")
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	// Verify issue was updated
	issues := loadTestIssues(t, beadsDir)
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	updated := issues[0]
	if updated.Status != model.StatusInProgress {
		t.Errorf("Expected status in_progress, got %s", updated.Status)
	}
	if updated.AckStatus != model.AckStatusAccepted {
		t.Errorf("Expected ack_status accepted, got %s", updated.AckStatus)
	}
	if updated.Assignee != "test-agent" {
		t.Errorf("Expected assignee test-agent, got %s", updated.Assignee)
	}
}

func TestDecline(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	issue := model.Issue{
		ID:          "test-456",
		Title:       "Test Issue",
		Status:      model.StatusOpen,
		IssueType:   model.TypeTask,
		BounceCount: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	svc := NewService(tmpDir)
	result, err := svc.Decline("test-456", "test-agent", "Not my expertise")
	if err != nil {
		t.Fatalf("Decline failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	issues := loadTestIssues(t, beadsDir)
	updated := issues[0]
	if updated.BounceCount != 1 {
		t.Errorf("Expected bounce_count 1, got %d", updated.BounceCount)
	}
	if updated.AckStatus != model.AckStatusDeclined {
		t.Errorf("Expected ack_status declined, got %s", updated.AckStatus)
	}
	if len(updated.Comments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(updated.Comments))
	}
}

func TestDeclineAutoEscalate(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Start with bounce_count at 2 (one below threshold)
	issue := model.Issue{
		ID:          "test-789",
		Title:       "Test Issue",
		Status:      model.StatusOpen,
		IssueType:   model.TypeTask,
		BounceCount: 2,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	svc := NewService(tmpDir)
	result, err := svc.Decline("test-789", "test-agent", "Still not my expertise")
	if err != nil {
		t.Fatalf("Decline failed: %v", err)
	}

	if !result.Escalated {
		t.Error("Expected escalation at bounce count 3")
	}

	issues := loadTestIssues(t, beadsDir)
	updated := issues[0]
	if updated.BounceCount != 3 {
		t.Errorf("Expected bounce_count 3, got %d", updated.BounceCount)
	}
	if !updated.Escalated {
		t.Error("Expected issue to be marked as escalated")
	}
}

func TestDefer(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	issue := model.Issue{
		ID:        "test-def",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		Assignee:  "agent-a",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	svc := NewService(tmpDir)
	result, err := svc.Defer("test-def", "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("Defer failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success: %s", result.Message)
	}

	issues := loadTestIssues(t, beadsDir)
	updated := issues[0]
	if updated.Assignee != "agent-b" {
		t.Errorf("Expected assignee agent-b, got %s", updated.Assignee)
	}
	if updated.DeferredFrom != "agent-a" {
		t.Errorf("Expected deferred_from agent-a, got %s", updated.DeferredFrom)
	}
	if updated.AckStatus != model.AckStatusDeferred {
		t.Errorf("Expected ack_status deferred, got %s", updated.AckStatus)
	}
}

func TestImpossible(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	issue := model.Issue{
		ID:        "test-imp",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	svc := NewService(tmpDir)
	result, err := svc.Impossible("test-imp", "test-agent", "Requires external API access")
	if err != nil {
		t.Fatalf("Impossible failed: %v", err)
	}

	if !result.Escalated {
		t.Error("Expected immediate escalation for impossible")
	}

	issues := loadTestIssues(t, beadsDir)
	updated := issues[0]
	if !updated.Escalated {
		t.Error("Expected issue to be marked as escalated")
	}
	if updated.AckStatus != model.AckStatusImpossible {
		t.Errorf("Expected ack_status impossible, got %s", updated.AckStatus)
	}
}

func TestIssueNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	issue := model.Issue{
		ID:        "test-existing",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	writeTestIssue(t, beadsDir, issue)

	svc := NewService(tmpDir)
	_, err := svc.Accept("nonexistent-id", "test-agent")
	if err == nil {
		t.Error("Expected error for nonexistent issue")
	}
}

func writeTestIssue(t *testing.T, beadsDir string, issue model.Issue) {
	t.Helper()
	path := filepath.Join(beadsDir, "issues.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create issues.jsonl: %v", err)
	}
	defer f.Close()

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Failed to marshal issue: %v", err)
	}
	f.Write(data)
	f.WriteString("\n")
}

func loadTestIssues(t *testing.T, beadsDir string) []model.Issue {
	t.Helper()
	path := filepath.Join(beadsDir, "issues.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read issues.jsonl: %v", err)
	}

	var issues []model.Issue
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var issue model.Issue
		if err := json.Unmarshal(line, &issue); err != nil {
			t.Fatalf("Failed to unmarshal issue: %v", err)
		}
		issues = append(issues, issue)
	}
	return issues
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
