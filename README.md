# agentsdk

A Go SDK for building AI coding agents with multi-provider support.

## Features

- **Multi-provider support**: Anthropic (Claude), OpenAI (GPT), Google (Gemini)
- **Interactive TUI**: Terminal UI for interactive coding sessions
- **Print mode**: Non-interactive mode for scripting and automation
- **Built-in tools**: File operations, code search, bash execution
- **Extended thinking**: Configurable reasoning levels for supported models
- **Read-only mode**: Safe exploration without write capabilities

## Installation

```bash
go install agentsdk@latest
```

Or build from source:

```bash
git clone https://github.com/yourusername/agentsdk.git
cd agentsdk
go build -o agentsdk .
```

## Usage

### Interactive Mode

```bash
agentsdk                                    # Auto-detect provider from env
agentsdk --model gpt-4o                     # Use specific model
agentsdk --provider anthropic               # Use specific provider
```

### Print Mode (Non-interactive)

```bash
agentsdk -p "explain this codebase"
agentsdk --print "fix the bug in main.go"
```

### Flags

| Flag | Description |
|------|-------------|
| `--model <id>` | Model ID (e.g., claude-sonnet-4-5-20250929, gpt-4o) |
| `--provider <name>` | Provider (anthropic, openai, google) |
| `--thinking <level>` | Thinking level (off, minimal, low, medium, high, xhigh) |
| `-p, --print <prompt>` | Non-interactive mode |
| `--api-key <key>` | API key (overrides environment variable) |
| `--cwd <dir>` | Working directory |
| `--read-only` | Use read-only tools only |
| `--max-turns <n>` | Maximum LLM round-trips (0 = unlimited) |
| `--list-models` | List available models |
| `--version` | Print version |
| `--help` | Print help |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GOOGLE_API_KEY` | Google AI API key |

## Available Models

```
anthropic:
  claude-sonnet-4-5-20250929
  claude-3-5-sonnet-20241022
  claude-3-5-haiku-20241022 (reasoning)

openai:
  gpt-4o
  gpt-4o-mini
  o1 (reasoning)
  o1-mini (reasoning)

google:
  gemini-2.5-flash
  gemini-2.5-pro
```

## SDK Usage

```go
package main

import (
    "context"
    "fmt"
    
    "agentsdk/ai"
    "agentsdk/agent"
)

func main() {
    model, _ := ai.GetModel(ai.ProviderAnthropic, "claude-sonnet-4-5-20250929")
    
    ag := agent.New(model, agent.Config{
        SystemPrompt: "You are a helpful coding assistant.",
    })
    
    response, _ := ag.Run(context.Background(), "Explain Go interfaces")
    fmt.Println(response)
}
```

## Project Structure

```
agentsdk/
├── agent/          # Core agent loop and types
├── ai/             # AI provider abstractions
│   └── providers/  # Provider implementations
├── coding/         # Coding session management
│   └── tools/      # Built-in tools (bash, edit, find, grep, ls, read, write)
├── tui/            # Terminal UI components
├── main.go         # CLI entrypoint
└── go.mod
```

## Development

```bash
# Run tests
go test ./...

# Run linter
go vet ./...

# Build
go build -o agentsdk .
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
