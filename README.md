# jira4claude

Minimal Jira CLI for AI agents - zero interactivity, readable flags, clean output.

## Why?

The official jira-cli hangs in non-interactive contexts (like Claude Code) because it prompts for optional fields. AI agents need predictable, non-interactive commands with structured output.

## Design Principles

- **Agent-first** - never prompt, never hang, always explicit
- **Minimal scope** - only commands agents actually need (~8 endpoints, not 417)
- **Clean output** - structured JSON, no ANSI colors, no spinners

## Status

Under development.

## License

MIT
