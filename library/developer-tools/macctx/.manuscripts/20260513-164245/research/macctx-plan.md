# macctx CLI

macctx turns the current state of a Mac into structured context for AI agents. It is a privacy-conscious, local-first wrapper around Peekaboo, optimized for developer workflows and agent runtimes.

## Commands

- `active` - Show the current foreground app and frontmost window using Peekaboo window/app inspection.
- `screenshot` - Capture a full-screen or frontmost-window screenshot with optional output path and JSON metadata.
- `see` - Capture a Peekaboo UI inspection snapshot and return snapshot metadata plus a path to an annotated image when requested.
- `clipboard` - Show a privacy-safe clipboard preview by default, with --full for explicit full text output.
- `windows` - List open windows, optionally filtered by app.
- `apps` - List running apps.
- `dump` - Produce an agent-friendly context bundle as markdown or JSON, optionally including screenshot and UI inspection.
- `handoff` - Generate a concise markdown handoff describing the visible/active Mac context for pasting into an AI agent.
