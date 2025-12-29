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

### 5. Respond to Comments

For each comment:
- If fixed: Reply with what was done
- If won't fix: Explain reasoning clearly
- If needs discussion: Ask clarifying questions

```bash
gh pr comment <pr-number> --body "..."
```

### 6. Request Re-review

After addressing all feedback:
```bash
gh pr ready
```

Summarize changes made to the user.
