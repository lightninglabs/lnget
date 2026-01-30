#!/bin/bash
# check-tasks-complete.sh
# Checks if all tasks for this project are completed.
# Outputs JSON for Claude Code Stop hook decision control.
#
# Hook Input (stdin JSON):
#   session_id: The current session UUID
#   stop_hook_active: true if this is a retry after previous block
#
# Output (stdout JSON):
#   decision: "block" to prevent stopping, null/omit to allow
#   reason: Message shown to Claude when blocked
#
# Behavior: Continuously blocks exit until ALL tasks are completed.
# The agent must complete all tasks before being allowed to stop.

# Read hook input from stdin.
input=$(cat)

# Extract session_id from hook input.
SESSION_ID=$(echo "$input" | jq -r '.session_id // empty' 2>/dev/null)

# If no session_id, allow stop (can't find tasks).
if [ -z "$SESSION_ID" ]; then
    exit 0
fi

TASKS_DIR="$HOME/.claude/tasks/$SESSION_ID"

# If no tasks directory, allow stop.
if [ ! -d "$TASKS_DIR" ]; then
    exit 0
fi

# Count incomplete tasks.
incomplete=0
incomplete_list=""

for task_file in "$TASKS_DIR"/*.json; do
    if [ ! -f "$task_file" ]; then
        continue
    fi

    status=$(jq -r '.status' "$task_file" 2>/dev/null)
    id=$(jq -r '.id' "$task_file" 2>/dev/null)

    if [ "$status" != "completed" ]; then
        incomplete=$((incomplete + 1))
        if [ -n "$incomplete_list" ]; then
            incomplete_list="$incomplete_list, #$id [$status]"
        else
            incomplete_list="#$id [$status]"
        fi
    fi
done

if [ $incomplete -gt 0 ]; then
    # Block stopping - keep blocking until ALL tasks are complete.
    cat << EOF
{
  "decision": "block",
  "reason": "$incomplete incomplete task(s): $incomplete_list. Complete ALL tasks before stopping."
}
EOF
    exit 0
fi

# All tasks complete - allow stop.
exit 0
