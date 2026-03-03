# Data Model: 038-agent-profiles

**Date**: 2026-03-03

## Entities

### AgentProfile

| Field          | Type              | Description                              |
|----------------|-------------------|------------------------------------------|
| Name           | string            | Unique profile identifier (config key)   |
| Description    | string            | Human-readable summary (for listing)     |
| Model          | string            | Default model for this profile           |
| Instructions   | string            | System prompt template (supports `{{var}}`) |
| Tools          | []ToolDefinition  | Default tools for this profile           |
| Temperature    | *float64          | Default temperature                      |
| TopP           | *float64          | Default top_p                            |
| MaxOutputTokens| *int              | Default max output tokens                |
| MaxToolCalls   | *int              | Default max tool calls                   |
| Reasoning      | *ReasoningConfig  | Default reasoning configuration          |
| Metadata       | map[string]any    | Arbitrary metadata                       |

### PromptReference (OpenAI compatibility)

| Field     | Type              | Description                            |
|-----------|-------------------|----------------------------------------|
| ID        | string            | Profile name or ID to resolve          |
| Version   | string            | Accepted but ignored in v1             |
| Variables | map[string]string | Template variable substitutions        |

## Interfaces

### ProfileResolver (1 method)

| Method  | Input           | Output                    |
|---------|-----------------|---------------------------|
| Resolve | name string     | *AgentProfile, error      |

**Implementations**: ConfigResolver (loads from config file)

## Request Extensions

### CreateResponseRequest (extended)

| Field     | Type             | Description                          |
|-----------|------------------|--------------------------------------|
| Agent     | string           | Profile name to resolve              |
| Prompt    | *PromptReference | OpenAI prompt parameter              |
| Variables | map[string]string| Template variables (alternative to prompt.variables) |

## Config Schema

```yaml
agents:
  devops-helper:
    description: "DevOps assistant for Kubernetes"
    model: qwen-2.5-72b
    instructions: |
      You are a DevOps assistant for {{project_name}}.
      Investigate issues methodically.
    tools:
      - type: mcp
        server: kubernetes-tools
      - type: builtin
        name: web_search
    temperature: 0.3
    max_tool_calls: 15
    reasoning:
      effort: medium
```
