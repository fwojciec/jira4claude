---
description: Run a self-learning retrospective after milestone completion - analyze logs, extract learnings, update memory
allowed-tools: Bash(gh:*), Bash(git:*), Bash(cat:*), Bash(wc:*)
argument-hint: "<milestone-name> [log-file-1] [log-file-2] ..."
---

# Milestone Retrospective

Structured self-learning loop after a milestone completes. Analyzes what happened, extracts process improvements, and updates persistent memory so future loops compound.

**Design principle**: The 1-10-100 rule applies to learning too. Insights captured immediately after a milestone are cheap and high-fidelity. Insights reconstructed weeks later are expensive and lossy.

## Arguments

Milestone name: first argument
Ralph loop logs: remaining arguments (file paths, optional)

Example: `/retrospective "API Simplification" ralph-20260206.log ralph-20260207.log`

## Phase 1: Gather Evidence

### 1.1 Milestone Summary

```bash
# Get all issues in milestone (open and closed)
gh issue list --milestone "$MILESTONE" --state all --json number,title,state,closedAt

# Get milestone metadata
gh api repos/{owner}/{repo}/milestones --jq '.[] | select(.title == "'"$MILESTONE"'")'
```

### 1.2 Commit History

```bash
# Find the commit range for this milestone's work
# Look for "Closes #<first-issue>" through "Closes #<last-issue>"
git log --oneline --all --grep="Closes #" | head -30
```

### 1.3 Analyze Loop Logs (if provided)

For each log file, launch a **parallel** explore subagent:

```
Task: subagent_type="Explore"
  prompt: "Analyze this ralph loop log. Use grep strategically (file is large).
           Extract:
           1. Iterations: count, duration, issues processed
           2. Failures: compilation errors, test failures, tool errors
           3. Friction patterns: what slowed the agent down?
           4. Smooth patterns: what went well?
           5. Scope adherence: did the agent stay within issue boundaries?
           6. Review outcomes: APPROVE/REJECT counts and reasons
           File: <log-path>"
```

Do NOT read the full log files yourself -- they're too large for main context. Always delegate to subagents.

### 1.4 Diff Analysis

```bash
# Total lines changed across the milestone
git diff <first-commit>^..<last-commit> --stat | tail -1

# Files changed
git diff <first-commit>^..<last-commit> --stat
```

---

## Phase 2: Analyze Each Pipeline Stage

Evaluate each stage of the design->issues->loop->review pipeline:

### 2.1 Design Phase

- How many review rounds did the design doc go through?
- What did each round catch? (structural, factual, contradictions)
- Were there claims in the doc that turned out to be wrong?
- Did the design doc accurately predict the implementation?

### 2.2 Issue Creation Phase

- Did issues have precise validation criteria?
- Which issues went smoothly? Which had friction? Why?
- Were there dependency/ordering problems?
- Was the granularity right? (too big, too small, just right)

### 2.3 Loop Execution Phase

- What was the success rate? (issues completed / attempted)
- What were the main sources of friction? (imports, tool errors, stale context)
- How fast was error recovery?
- Did the agent stay within scope?

### 2.4 Post-Loop Review Phase

- What did the post-loop review catch that per-iteration reviews missed?
- Were findings genuine bugs or style issues?
- How many fix issues were created?

---

## Phase 3: Extract Learnings

### 3.1 Read Current Memory

Read the existing learnings file (if any):

```bash
cat .claude/projects/-Users-filip-code-go-jira4claude/memory/ralph-loop-learnings.md 2>/dev/null || echo "No existing learnings"
```

### 3.2 Identify New Insights

For each finding from Phase 2, ask:
- Is this already captured in memory? -> Skip
- Is this a refinement of an existing insight? -> Update
- Is this genuinely new? -> Add

**Categories to evaluate:**
- Process improvements (how we work)
- Technical patterns (what works in this codebase)
- Tool/workflow friction (what to avoid)
- Metrics (baselines for comparison)

### 3.3 Identify Anti-Patterns

Look for repeated mistakes across milestones:
- Same type of error appearing in multiple loops
- Process steps that are consistently skipped
- Friction that was identified but not fixed

---

## Phase 4: Update Memory

### 4.1 Update Learnings File

Write or update `ralph-loop-learnings.md` in the memory directory:
- Add new insights under the appropriate section
- Update metrics with new data points
- Refine existing insights if this milestone provided better evidence
- Remove or correct insights that turned out to be wrong

### 4.2 Update MEMORY.md

If any project-level facts changed (new patterns, new types, new architecture), update MEMORY.md.

### 4.3 Update Process Improvements

The "Process Improvements for Next Time" section is the most important output. It should be:
- Numbered and prioritized
- Actionable (not vague)
- Cumulative (keep working improvements, drop ones that didn't help)

---

## Phase 5: Report

Present a structured summary to the user:

```
## Milestone Retrospective: <name>

### Metrics
- Issues: <completed>/<total> (<success-rate>%)
- Duration: <total-time> (~<per-issue> per issue)
- Fix issues created post-review: <count>
- Lines changed: +<added> / -<removed>

### What Went Well
- <2-3 bullet points with evidence>

### What Didn't Go Well
- <2-3 bullet points with evidence>

### New Learnings Captured
- <list of new/updated insights added to memory>

### Process Changes for Next Milestone
- <numbered list of concrete changes>
```

---

## Phase 6: Suggest Ralph Loop Improvements (Optional)

If the retrospective reveals friction that could be fixed in the ralph-iterate command itself:

Present the potential improvements to the user with options:
- <specific change 1>
- <specific change 2>
- Skip for now

Only suggest changes backed by evidence from this milestone. Don't speculate.
