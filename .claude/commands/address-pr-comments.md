---
description: Fetch, evaluate, and respond to PR feedback
allowed-tools: Bash(gh:*), Bash(git:*), Bash(make:*)
---

## Current State

Branch: !`git branch --show-current`
Git status: !`git status --porcelain`

## Your Workflow

### 1. Fetch PR Comments

Get the PR number for the current branch:
```bash
gh pr view --json number,title,url
```

Fetch all comments and reviews:
```bash
gh pr view --comments
gh api repos/{owner}/{repo}/pulls/{pr_number}/reviews
```

### 2. Categorize Feedback

For each piece of feedback, categorize as:
- **Must fix**: Bugs, correctness issues, security concerns
- **Should fix**: Valid improvements that align with project standards
- **Discuss**: Disagreements that need clarification
- **Won't fix**: Stylistic preferences that don't improve the code

### 3. Evaluate with Rigor

Use `superpowers:receiving-code-review` to evaluate feedback:
- Don't blindly agree - verify claims are accurate
- Push back on suggestions that don't improve correctness
- Accept structural improvements even if "not strictly necessary"

### 4. Implement Changes

For accepted feedback:
1. Make the changes
2. Run `make validate`
3. Commit with clear message referencing the feedback

### 5. Respond Inline

For EVERY inline code review comment, reply using the `/replies` endpoint.

**Critical syntax rules for `gh api`:**
- `{owner}/{repo}` is a **literal placeholder** - type it exactly as shown, `gh api` auto-substitutes it
- Do NOT replace `{owner}/{repo}` with the actual repo name
- DO replace `$PR_NUM` and `$COMMENT_ID` with actual numeric values

```bash
# CORRECT - {owner}/{repo} is literal, 10 and 2652398654 are actual values
gh api repos/{owner}/{repo}/pulls/10/comments/2652398654/replies \
  -f body="Done - description of change"

# WRONG - don't hardcode the repo path
gh api repos/fwojciec/jira4claude/pulls/10/comments/2652398654/replies ...
```

The `$COMMENT_ID` is the numeric `id` field from the comment JSON (e.g., `2652398654`).

For general PR comments (not inline code comments):
```bash
gh pr comment <PR_NUMBER> --body "Your response"
```

Response format for each comment:
- **If implemented**: "Done - [brief description of change]"
- **If partially implemented**: "Partially addressed - [what was done and why]"
- **If not implemented**: "Not changing - [technical rationale]"

Be professional and constructive. Explain reasoning when declining suggestions.

### 6. Push Updates

After all changes are made:
1. Push the updated branch
2. Summarize actions taken for the user
