#!/bin/bash
# Ralph Wiggum Loop - autonomous milestone execution
#
# Usage: ./ralph.sh "<milestone-name>"
# Example: ./ralph.sh "API Simplification"
#
# The script works through all open issues in the milestone, respecting dependencies.
# Each iteration picks the first ready issue, implements it, and commits.
# Loop continues until all issues are complete or blocked.

set -e

if [ $# -eq 0 ]; then
    echo "Usage: ./ralph.sh \"<milestone-name>\""
    echo ""
    echo "Available milestones:"
    gh api 'repos/:owner/:repo/milestones?per_page=100' --jq '.[] | "  - \(.title) (\(.open_issues) open)"'
    exit 1
fi

MILESTONE="$1"
CLAUDE="${CLAUDE:-claude}"
LOGFILE="ralph-$(date +%Y%m%d-%H%M%S).log"
COMPLETE_MARKER=".ralph-complete"

# Verify milestone exists
if ! gh api 'repos/:owner/:repo/milestones?per_page=100' --jq '.[].title' | grep -qx "$MILESTONE"; then
    echo "Error: Milestone '$MILESTONE' not found"
    echo ""
    echo "Available milestones:"
    gh api 'repos/:owner/:repo/milestones?per_page=100' --jq '.[] | "  - \(.title)"'
    exit 1
fi

# Show initial state
open_count=$(gh issue list --milestone "$MILESTONE" --state open --json number --jq 'length')
echo "Starting Ralph loop"
echo "Milestone: $MILESTONE"
echo "Open issues: $open_count"
echo "Logging to: $LOGFILE"
echo ""

# Clean up any stale marker
rm -f "$COMPLETE_MARKER"

iteration=0
while :; do
    iteration=$((iteration + 1))

    # Check remaining open issues
    remaining=$(gh issue list --milestone "$MILESTONE" --state open --json number --jq 'length')

    echo ""
    echo "=============================================="
    echo "Iteration $iteration - $(date '+%Y-%m-%d %H:%M:%S')"
    echo "Remaining issues: $remaining"
    echo "=============================================="

    if [ "$remaining" -eq 0 ]; then
        echo ""
        echo "All issues in milestone complete!"
        break
    fi

    # Run one iteration
    "$CLAUDE" -p "/ralph-iterate $MILESTONE" \
        --dangerously-skip-permissions \
        --output-format stream-json 2>&1 | \
        tee -a "$LOGFILE" | \
        jq -r --unbuffered '
            if .type == "assistant" and .message.content then
                .message.content[] |
                if .type == "text" then .text
                elif .type == "tool_use" then
                    ">>> " + .name + ": " + (
                        .input |
                        if .command then .command
                        elif .file_path then .file_path
                        elif .pattern then .pattern
                        elif .query then .query
                        elif .prompt then (.prompt | split("\n")[0])
                        elif .skill then .skill
                        else (tostring | .[0:80])
                        end
                    )
                else empty
                end
            elif .type == "result" then
                "\n--- Iteration complete ---"
            else empty
            end
        ' 2>/dev/null || true

    # Check if signaled complete (all blocked or done)
    if [ -f "$COMPLETE_MARKER" ]; then
        rm -f "$COMPLETE_MARKER"
        remaining=$(gh issue list --milestone "$MILESTONE" --state open --json number --jq 'length')
        if [ "$remaining" -eq 0 ]; then
            echo ""
            echo "=============================================="
            echo "Milestone '$MILESTONE' complete!"
            echo "Total iterations: $iteration"
            echo "=============================================="
        else
            echo ""
            echo "=============================================="
            echo "Stopping: $remaining issues remain but all are blocked"
            echo "Check dependencies and re-run when unblocked"
            echo "=============================================="
        fi
        break
    fi

    # Safety check: don't run forever
    if [ $iteration -ge 50 ]; then
        echo ""
        echo "ERROR: Reached iteration limit (50). Stopping."
        echo "Check $LOGFILE for details."
        exit 1
    fi

    # Brief pause between iterations
    echo ""
    echo "Pausing before next iteration..."
    sleep 3
done

echo ""
echo "Log file: $LOGFILE"
