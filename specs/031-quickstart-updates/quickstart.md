# Quickstart: Quickstart Updates Feature

## What This Feature Adds

Two new quickstarts to the progressive series:

- **05-code-interpreter**: Deploy antwort with a Python sandbox for LLM code execution
- **06-responses-proxy**: Deploy two antwort instances in a proxy chain architecture

Plus structured output and reasoning examples added to all existing quickstarts (01-04).

## Quick Verification

After deploying any quickstart, verify the new features work:

### Structured Output (works on all quickstarts 01-06)

```bash
export URL=http://localhost:8080

curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [{"type": "message", "role": "user",
      "content": [{"type": "input_text", "text": "Name 3 colors"}]}],
    "text": {"format": {"type": "json_schema", "name": "colors",
      "schema": {"type": "object", "properties": {"colors": {"type": "array", "items": {"type": "string"}}}, "required": ["colors"]}}}
  }' | jq '.output[] | select(.type == "message") | .content[0].text' -r | jq .
```

### Code Execution (05-code-interpreter only)

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [{"type": "message", "role": "user",
      "content": [{"type": "input_text", "text": "Calculate the first 10 Fibonacci numbers"}]}]
  }' | jq '.output[] | select(.type == "code_interpreter_call") | .code'
```

### Proxy Chain (06-responses-proxy only)

```bash
# Send to frontend, verify response comes from backend LLM
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "input": [{"type": "message", "role": "user",
      "content": [{"type": "input_text", "text": "Hello, world!"}]}]
  }' | jq '.status'
```
