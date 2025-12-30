# Agent Instructions

This project uses **Jira** for issue tracking. Project key: `J4C`

## Quick Reference

```bash
# List open tasks
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search?jql=project=J4C+AND+status!=Done&fields=key,summary,status' | jq '.issues[] | {key, summary: .fields.summary, status: .fields.status.name}'

# View issue details
curl -s -n https://fwojciec.atlassian.net/rest/api/3/issue/J4C-1 | jq '{key, summary: .fields.summary, status: .fields.status.name, description: .fields.description}'

# Create issue
curl -s -n -X POST -H "Content-Type: application/json" https://fwojciec.atlassian.net/rest/api/3/issue -d '{"fields": {"project": {"key": "J4C"}, "summary": "Issue title", "issuetype": {"id": "10005"}}}'

# Transition issue (get transitions first, then apply)
curl -s -n https://fwojciec.atlassian.net/rest/api/3/issue/J4C-1/transitions | jq '.transitions[] | {id, name}'
curl -s -n -X POST -H "Content-Type: application/json" https://fwojciec.atlassian.net/rest/api/3/issue/J4C-1/transitions -d '{"transition": {"id": "31"}}'
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create Jira issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Transition finished work to Done
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
