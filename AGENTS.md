# AGENTS.md

## Commands

```bash
# Build (Go backend + Svelte frontend)
make build

# Build Go only (skip frontend)
go build ./cmd

# Test
go test ./...

# Run a single test
go test ./pkg/tools -run TestEditTool

# Lint (runs go vet + deadcode)
make lint

# Format Go
gofmt -w .

# Frontend dev server
cd web && bun run dev

# Frontend build
cd web && bun run build

# Frontend type check
cd web && bun run check

# Frontend format
cd web && bunx biome format --write .

# Frontend lint
cd web && bunx biome check .
```

## Architecture

localagent is a personal AI agent with a Go backend and SvelteKit frontend. It
uses the OpenAI-compatible chat completions API to talk to any LLM provider
(default: local Ollama).

### Two runtime modes

- **`agent`** - CLI mode. Processes a single message (`-m`) or runs an
  interactive REPL.
- **`gateway`** - Long-running daemon. Starts the agent loop, channels,
  heartbeat, cron, health server, and webchat server. This is the primary
  production mode.

### Core packages (`pkg/`)

- **`agent`** - The agent loop (`AgentLoop`). Receives messages from the bus,
  builds context (system prompt + session history + memory), calls the LLM,
  executes tool calls in a loop, and manages summarization. `ContextBuilder`
  assembles the system prompt from identity, tools, skills, bootstrap files, and
  memory.
- **`bus`** - `MessageBus` with inbound/outbound channels. All message routing
  goes through the bus. Channels publish inbound; the agent consumes inbound,
  produces outbound; the dispatcher routes outbound to channels.
- **`tools`** - Tool system. `Tool` interface (`Name`, `Description`,
  `Parameters`, `Execute`). Optional interfaces: `ContextualTool` (receives
  channel/chatID), `AsyncTool` (background execution with callback).
  `ToolRegistry` manages registration and execution. `RunToolLoop` is a reusable
  LLM-tool iteration loop used by subagents and memory flush.
- **`providers`** - `LLMProvider` interface and `HTTPProvider` implementation.
  Uses OpenAI-compatible `/v1/chat/completions` endpoint. `Message` type
  supports multimodal content (text + images via base64 data URLs).
- **`channels`** - Channel abstraction (`Channel` interface: `Start`, `Stop`,
  `Send`, `IsRunning`). `Manager` starts/stops channels and dispatches outbound
  messages. The webchat channel is always registered in gateway mode.
- **`session`** - JSONL-based session persistence. Stores messages, activity
  events, and summaries. Sessions are identified by keys like `web:default` or
  `cli:default`.
- **`webchat`** - HTTP server (Echo v5) serving the SvelteKit SPA and API
  endpoints (`/api/messages`, `/api/upload`, `/api/history`, `/api/events` SSE).
  Static files are embedded via `//go:embed`.
- **`prompts`** - All prompt templates loaded via `//go:embed` from `.txt` files
  in the same package.
- **`skills`** - Skill system. Skills are `SKILL.md` files with YAML frontmatter
  (name, description). Loaded from three sources with priority: workspace >
  global (`~/.localagent/skills`) > builtin (`skills/` in working directory).
- **`heartbeat`** - Periodic background task that reads `HEARTBEAT.md` from
  workspace, sends it through the agent, and delivers results to the last active
  channel.
- **`config`** - JSON config loaded from `~/.localagent/config.json`. Supports
  env var overrides (`LOCALAGENT_*`).
- **`state`** - Atomic file-based state persistence (last channel, last chat
  ID).
- **`cron`** - Cron job scheduling with persistent job storage.

### Tool result model

`ToolResult` has three content fields: `ForLLM` (fed back to the LLM), `ForUser`
(sent directly to the user), and `Silent` (suppresses user output). Constructor
helpers: `SuccessResult`, `ErrorResult`, `SilentResult`, `AsyncResult`.

### Subagent system

Two tool types for delegating work: `spawn` (async, runs in background
goroutine, reports via bus) and `subagent` (synchronous, blocks until complete).
Both use `SubagentManager` which creates a separate tool registry (without
spawn/subagent tools to prevent recursion).

### Frontend (`web/`)

SvelteKit 2 SPA with Svelte 5, Tailwind CSS 4, TypeScript. Uses
`@sveltejs/adapter-static` for static build. Package manager is **bun**.
Formatter/linter is **Biome** (tab indentation, double quotes). Built output is
embedded into the Go binary via `//go:embed static/*` in
`pkg/webchat/server.go`.

### Message flow (gateway mode)

1. Channel receives external message → publishes `InboundMessage` to bus
2. `AgentLoop.Run()` consumes from bus → `processMessage()` → `runAgentLoop()`
3. Context builder assembles system prompt + history + user message
4. LLM called in iteration loop (up to `max_tool_iterations`)
5. Tool calls executed, results appended to messages, loop continues
6. Final response saved to session, published as `OutboundMessage`
7. Channel manager's dispatcher routes outbound to correct channel

### Config (`~/.localagent/config.json`)

Configures: LLM provider (API base, key env var, proxy), agent defaults (model,
max tokens, temperature, tool iterations), gateway (host, port), tools (web
search, PDF), heartbeat, webchat.
