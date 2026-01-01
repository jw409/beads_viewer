// Package ack implements task acknowledgment actions for the beads issue tracker.
// It supports accept, decline, defer, and impossible actions as part of the
// Task Accountability System.
//
// Key behaviors:
//   - accept: marks bead in_progress with assignee
//   - decline: increments bounce_count, adds comment, auto-escalates at 3 bounces
//   - defer: reassigns bead to another agent
//   - impossible: immediate escalation with reason
package ack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// MaxBounces is the threshold after which a task auto-escalates
const MaxBounces = 3

// Result represents the outcome of an ack action
type Result struct {
	Success      bool
	Message      string
	Escalated    bool
	EscalateInfo string
}

// Service provides task acknowledgment operations
type Service struct {
	repoPath string
}

// NewService creates a new ack service for the given repository path
func NewService(repoPath string) *Service {
	return &Service{repoPath: repoPath}
}

// Accept marks a task as accepted by the agent
func (s *Service) Accept(issueID, agentID string) (*Result, error) {
	return s.modifyIssue(issueID, func(issue *model.Issue) (*Result, error) {
		now := time.Now()

		issue.Status = model.StatusInProgress
		issue.AckStatus = model.AckStatusAccepted
		issue.Assignee = agentID
		issue.UpdatedAt = now

		return &Result{
			Success: true,
			Message: fmt.Sprintf("Task %s accepted by %s", issueID, agentID),
		}, nil
	})
}

// Decline marks a task as declined and increments bounce count
func (s *Service) Decline(issueID, agentID, reason string) (*Result, error) {
	if reason == "" {
		return nil, fmt.Errorf("decline reason is required")
	}

	return s.modifyIssue(issueID, func(issue *model.Issue) (*Result, error) {
		now := time.Now()

		issue.BounceCount++
		issue.AckStatus = model.AckStatusDeclined
		issue.UpdatedAt = now

		// Add decline comment
		commentID := int64(len(issue.Comments) + 1)
		issue.Comments = append(issue.Comments, &model.Comment{
			ID:        commentID,
			IssueID:   issueID,
			Author:    agentID,
			Text:      fmt.Sprintf("[DECLINED] %s (bounce %d/%d)", reason, issue.BounceCount, MaxBounces),
			CreatedAt: now,
		})

		result := &Result{
			Success: true,
			Message: fmt.Sprintf("Task %s declined by %s (bounce %d/%d)", issueID, agentID, issue.BounceCount, MaxBounces),
		}

		// Auto-escalate if max bounces reached
		if issue.BounceCount >= MaxBounces {
			issue.Escalated = true
			result.Escalated = true
			result.EscalateInfo = fmt.Sprintf("Task bounced %d times - escalated. Last reason: %s", issue.BounceCount, reason)
			result.Message = fmt.Sprintf("Task %s auto-escalated after %d declines", issueID, MaxBounces)
		}

		return result, nil
	})
}

// Defer reassigns a task to another agent
func (s *Service) Defer(issueID, fromAgent, toAgent string) (*Result, error) {
	if toAgent == "" {
		return nil, fmt.Errorf("target agent is required for defer")
	}

	return s.modifyIssue(issueID, func(issue *model.Issue) (*Result, error) {
		now := time.Now()

		issue.DeferredFrom = fromAgent
		issue.Assignee = toAgent
		issue.AckStatus = model.AckStatusDeferred
		issue.UpdatedAt = now

		// Add defer comment
		commentID := int64(len(issue.Comments) + 1)
		issue.Comments = append(issue.Comments, &model.Comment{
			ID:        commentID,
			IssueID:   issueID,
			Author:    fromAgent,
			Text:      fmt.Sprintf("[DEFERRED] Task deferred from %s to %s", fromAgent, toAgent),
			CreatedAt: now,
		})

		return &Result{
			Success: true,
			Message: fmt.Sprintf("Task %s deferred from %s to %s", issueID, fromAgent, toAgent),
		}, nil
	})
}

// Impossible marks a task as impossible and escalates immediately
func (s *Service) Impossible(issueID, agentID, reason string) (*Result, error) {
	if reason == "" {
		return nil, fmt.Errorf("impossible reason is required")
	}

	return s.modifyIssue(issueID, func(issue *model.Issue) (*Result, error) {
		now := time.Now()

		issue.AckStatus = model.AckStatusImpossible
		issue.Escalated = true
		issue.UpdatedAt = now

		// Add impossible comment
		commentID := int64(len(issue.Comments) + 1)
		issue.Comments = append(issue.Comments, &model.Comment{
			ID:        commentID,
			IssueID:   issueID,
			Author:    agentID,
			Text:      fmt.Sprintf("[IMPOSSIBLE] %s - Escalated for strategic decision", reason),
			CreatedAt: now,
		})

		return &Result{
			Success:      true,
			Message:      fmt.Sprintf("Task %s marked impossible by %s - escalated", issueID, agentID),
			Escalated:    true,
			EscalateInfo: reason,
		}, nil
	})
}

// modifyIssue loads the issue, applies the modification function, and saves it back
func (s *Service) modifyIssue(issueID string, modifyFn func(*model.Issue) (*Result, error)) (*Result, error) {
	beadsDir, err := loader.GetBeadsDir(s.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get beads directory: %w", err)
	}

	jsonlPath, err := loader.FindJSONLPath(beadsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find JSONL file: %w", err)
	}

	// Load all issues
	issues, err := loader.LoadIssuesFromFile(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load issues: %w", err)
	}

	// Find and modify the target issue
	var result *Result
	found := false
	for i := range issues {
		if issues[i].ID == issueID {
			result, err = modifyFn(&issues[i])
			if err != nil {
				return nil, err
			}
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("issue %s not found", issueID)
	}

	// Write back all issues atomically
	if err := writeIssuesAtomic(jsonlPath, issues); err != nil {
		return nil, fmt.Errorf("failed to write issues: %w", err)
	}

	return result, nil
}

// writeIssuesAtomic writes all issues to the JSONL file atomically
func writeIssuesAtomic(filePath string, issues []model.Issue) error {
	// Get file info to preserve permissions
	var mode os.FileMode = 0644
	if info, err := os.Stat(filePath); err == nil {
		mode = info.Mode()
	}

	// Create temp file in same directory (required for atomic rename)
	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, ".bd-ack-atomic-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Cleanup temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write each issue as a JSON line
	writer := bufio.NewWriter(tmp)
	for _, issue := range issues {
		line, err := json.Marshal(issue)
		if err != nil {
			tmp.Close()
			return fmt.Errorf("marshal issue %s: %w", issue.ID, err)
		}
		if _, err := writer.Write(line); err != nil {
			tmp.Close()
			return fmt.Errorf("write issue %s: %w", issue.ID, err)
		}
		if _, err := writer.WriteString("\n"); err != nil {
			tmp.Close()
			return fmt.Errorf("write newline: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush buffer: %w", err)
	}

	// Ensure data is flushed to disk
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	success = true
	return nil
}
