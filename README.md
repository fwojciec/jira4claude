# jira4claude

A minimal Jira CLI designed for AI coding agents.

![Demo of Claude Code using j4c to list and view Jira tasks](assets/demo.gif)

## Why?

AI agents need CLI tools that:
- Never prompt for input or hang waiting for user interaction
- Produce predictable, parseable output
- Have explicit flags instead of interactive menus

This CLI is built specifically for that use case.

## Design Principles

- **Agent-first** - never prompt, never hang, always explicit
- **Minimal scope** - only the commands agents actually need
- **Clean output** - human-readable tables, optional JSON for parsing

## Status

Alpha. Built to solve a specific workflow problem.

## License

MIT
