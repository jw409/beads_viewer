// Command bd-ack implements task acknowledgment for the beads issue tracker.
//
// Usage:
//
//	bd-ack <id> accept                    Mark task as accepted
//	bd-ack <id> decline "reason"          Decline with reason (bounces, auto-escalates at 3)
//	bd-ack <id> defer <other-agent>       Reassign to another agent
//	bd-ack <id> impossible "reason"       Mark as impossible (immediate escalation)
//
// Environment:
//
//	BD_ACTOR - Agent name for the action (default: $USER)
//	BEADS_DIR - Custom beads directory (default: .beads)
//
// Exit codes:
//
//	0 - Success
//	1 - Usage error or invalid arguments
//	2 - Issue not found
//	3 - Action failed
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Dicklesworthstone/beads_viewer/pkg/ack"
)

var (
	jsonOutput bool
	actor      string
)

func main() {
	flag.BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	flag.StringVar(&actor, "actor", getDefaultActor(), "Actor name for audit trail")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		printUsage()
		os.Exit(1)
	}

	issueID := args[0]
	action := args[1]

	// Get repo path (current working directory)
	repoPath, err := os.Getwd()
	if err != nil {
		exitWithError("failed to get working directory", err, 3)
	}

	svc := ack.NewService(repoPath)

	var result *ack.Result

	switch action {
	case "accept":
		result, err = svc.Accept(issueID, actor)

	case "decline":
		if len(args) < 3 {
			exitWithError("decline requires a reason", nil, 1)
		}
		reason := args[2]
		result, err = svc.Decline(issueID, actor, reason)

	case "defer":
		if len(args) < 3 {
			exitWithError("defer requires a target agent", nil, 1)
		}
		toAgent := args[2]
		result, err = svc.Defer(issueID, actor, toAgent)

	case "impossible":
		if len(args) < 3 {
			exitWithError("impossible requires a reason", nil, 1)
		}
		reason := args[2]
		result, err = svc.Impossible(issueID, actor, reason)

	default:
		exitWithError(fmt.Sprintf("unknown action: %s", action), nil, 1)
	}

	if err != nil {
		if err.Error() == fmt.Sprintf("issue %s not found", issueID) {
			exitWithError(err.Error(), nil, 2)
		}
		exitWithError("action failed", err, 3)
	}

	printResult(result)

	if result.Escalated {
		// Exit with special code to indicate escalation happened
		os.Exit(10)
	}
}

func getDefaultActor() string {
	if actor := os.Getenv("BD_ACTOR"); actor != "" {
		return actor
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `bd-ack - Task acknowledgment for beads

Usage:
  bd-ack [options] <id> accept                    Accept the task
  bd-ack [options] <id> decline "reason"          Decline with reason
  bd-ack [options] <id> defer <other-agent>       Reassign to another agent
  bd-ack [options] <id> impossible "reason"       Mark as impossible

Options:
  --json            Output in JSON format
  --actor <name>    Actor name for audit trail (default: $BD_ACTOR or $USER)

Environment:
  BD_ACTOR          Agent name for actions
  BEADS_DIR         Custom beads directory path

Exit Codes:
  0   Success
  1   Usage error
  2   Issue not found
  3   Action failed
  10  Success with escalation

Examples:
  bd-ack game1-abc123 accept
  bd-ack game1-abc123 decline "Outside my expertise"
  bd-ack game1-abc123 defer backend-agent
  bd-ack game1-abc123 impossible "Requires external API access"
`)
}

func printResult(result *ack.Result) {
	if jsonOutput {
		output := map[string]interface{}{
			"success": result.Success,
			"message": result.Message,
		}
		if result.Escalated {
			output["escalated"] = true
			output["escalate_info"] = result.EscalateInfo
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else {
		if result.Success {
			fmt.Printf("OK: %s\n", result.Message)
		} else {
			fmt.Printf("FAILED: %s\n", result.Message)
		}
		if result.Escalated {
			fmt.Printf("ESCALATED: %s\n", result.EscalateInfo)
		}
	}
}

func exitWithError(msg string, err error, code int) {
	if jsonOutput {
		output := map[string]interface{}{
			"success": false,
			"error":   msg,
		}
		if err != nil {
			output["details"] = err.Error()
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
		}
	}
	os.Exit(code)
}
