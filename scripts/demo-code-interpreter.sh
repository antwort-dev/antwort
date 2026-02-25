#!/usr/bin/env bash
# Demo: Code Interpreter with Qwen on ROSA
#
# Sends a request to the antwort gateway with the code_interpreter tool.
# The model writes Python code, antwort executes it in the sandbox pod,
# and the result flows back through the agentic loop.
#
# Prerequisites:
#   - oc logged in to the ROSA cluster
#   - antwort and sandbox-server deployed in the antwort namespace
#
# Usage:
#   ./scripts/demo-code-interpreter.sh "Calculate fibonacci(20)"
#   ./scripts/demo-code-interpreter.sh "Create a list of prime numbers up to 50"
#   ./scripts/demo-code-interpreter.sh  # uses default prompt

set -euo pipefail

NAMESPACE="${ANTWORT_NAMESPACE:-antwort}"
LOCAL_PORT="${ANTWORT_PORT:-8081}"
MODEL="${ANTWORT_MODEL:-/mnt/models}"
PROMPT="${1:-Calculate the sum of squares from 1 to 100 and print the result.}"

echo "=== Antwort Code Interpreter Demo ==="
echo "Namespace: $NAMESPACE"
echo "Model:     $MODEL"
echo "Prompt:    $PROMPT"
echo ""

# Start port-forward in background.
echo "Starting port-forward..."
oc port-forward -n "$NAMESPACE" svc/antwort "$LOCAL_PORT":8080 > /dev/null 2>&1 &
PF_PID=$!
trap "kill $PF_PID 2>/dev/null; wait $PF_PID 2>/dev/null" EXIT
sleep 3

# Build the request using jq for safe JSON escaping.
REQUEST=$(jq -n \
  --arg model "$MODEL" \
  --arg prompt "$PROMPT" \
  '{
    model: $model,
    instructions: "When using code_interpreter, always use print() to output results. The sandbox captures stdout only.",
    input: [{
      type: "message",
      role: "user",
      content: [{type: "input_text", text: $prompt}]
    }],
    tools: [{
      type: "function",
      name: "code_interpreter",
      description: "Execute Python code in an isolated sandbox. Use print() for output.",
      parameters: {
        type: "object",
        properties: {
          code: {type: "string", description: "Python code to execute. Always use print() to show results."},
          requirements: {type: "array", items: {type: "string"}, description: "Packages to install"}
        },
        required: ["code"]
      }
    }],
    max_tool_calls: 3
  }')

echo "Sending request..."
echo ""

RESPONSE=$(curl -s -X POST "http://localhost:$LOCAL_PORT/v1/responses" \
  -H "Content-Type: application/json" \
  -d "$REQUEST")

# Parse and display results.
STATUS=$(echo "$RESPONSE" | jq -r '.status')
echo "=== Response Status: $STATUS ==="
echo ""

# Show each output item.
echo "$RESPONSE" | jq -r '.output[] |
  if .type == "function_call" then
    "🔧 Tool Call: \(.name)\n   Code: \(.arguments | fromjson | .code)"
  elif .type == "function_call_output" then
    "📤 Result: \(.output | fromjson | .outputs[]? | select(.type == "logs") | .logs // "(no output)")"
  elif .type == "message" then
    "💬 Answer: \(.content[]?.text // "(no text)")"
  else
    "   [\(.type)]: \(.status)"
  end'

echo ""
echo "=== Usage ==="
echo "$RESPONSE" | jq '{input_tokens: .usage.input_tokens, output_tokens: .usage.output_tokens, total: .usage.total_tokens}'
