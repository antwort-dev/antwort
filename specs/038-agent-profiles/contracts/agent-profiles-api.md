# API Contract: Agent Profiles

**Version**: 1.0

## POST /v1/responses (extended)

The existing create response endpoint gains two new optional fields:

### `agent` field

```json
{
  "agent": "devops-helper",
  "input": "Check the status of my Kubernetes cluster"
}
```

Resolves the named profile and merges its settings with the request.

### `prompt` field (OpenAI compatibility)

```json
{
  "prompt": {
    "id": "devops-helper",
    "version": "1",
    "variables": {
      "project_name": "antwort"
    }
  },
  "input": "Help me deploy the new version"
}
```

Resolves the profile by ID, substitutes `{{variables}}` in instructions.

### `variables` field (alternative)

```json
{
  "agent": "devops-helper",
  "variables": {
    "project_name": "antwort"
  },
  "input": "Help me deploy"
}
```

---

## GET /v1/agents

List available agent profiles.

**Response** (200 OK):
```json
{
  "object": "list",
  "data": [
    {
      "name": "devops-helper",
      "description": "DevOps assistant for Kubernetes",
      "model": "qwen-2.5-72b"
    },
    {
      "name": "code-reviewer",
      "description": "Code review assistant",
      "model": "deepseek-r1"
    }
  ]
}
```

**Errors**:
- 501: Agents not configured (no profiles in config)
