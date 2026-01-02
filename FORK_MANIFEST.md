# jw409/beads_viewer Fork Manifest

This fork of [Dicklesworthstone/beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) adds TalentOS-specific features.

## Binaries

| Binary | Purpose | Upstream? |
|--------|---------|-----------|
| `bd` | Beads CRUD operations | From jw409/beads fork |
| `bv` | Graph analysis TUI | ✅ Yes |
| `bd-ack` | Task acknowledgment workflow | ❌ Fork only |

## bd-ack Usage

```bash
bd-ack <id> accept              # Accept/claim the task (sets assignee)
bd-ack <id> decline "reason"    # Reject with reason
bd-ack <id> defer <agent>       # Reassign to another agent
bd-ack <id> impossible "reason" # Mark as impossible to complete
```

## Fork-Only Features

1. **bd-ack CLI** - Task accountability workflow
2. **close_reason field** - Store why beads were closed
3. **Light terminal support** - AdaptiveColor
4. **TUI improvements** - Disabled mouse capture, footer hints

## Schema Extensions (in jw409/beads)

- `hook_bead`, `role_bead`, `agent_state`, `role_type`, `close_reason`

## Keeping Up with Upstream

```bash
git fetch origin && git merge origin/main && git push jw409 main
```
