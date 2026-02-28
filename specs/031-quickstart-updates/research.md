# Research: Quickstart Updates

## Decision: Sandbox Container Image Name

- **Decision**: Use `quay.io/rhuss/antwort-sandbox:latest`
- **Rationale**: Follows naming convention of main image (`quay.io/rhuss/antwort:latest`), built from `Containerfile.sandbox`
- **Alternatives**: Using same image with different tag rejected because sandbox has a different binary and runtime

## Decision: Responses Provider Config Key

- **Decision**: Use `engine.provider: vllm-responses` for the frontend
- **Rationale**: This is the existing provider name registered in `cmd/server/main.go` (line 248)
- **Alternatives**: None, this is the only supported provider name

## Decision: Code Interpreter Config Structure

- **Decision**: Use `providers.code_interpreter.settings.sandbox_url`
- **Rationale**: Matches the `createFunctionRegistry()` switch case in `cmd/server/main.go` (line 213) and `codeinterpreter.New()` settings parsing
- **Alternatives**: Direct env vars rejected because config.yaml is the standard pattern since Spec 012

## Decision: Naming Backend/Frontend in 06-responses-proxy

- **Decision**: Use `antwort-backend` and `antwort-frontend` as resource name prefixes
- **Rationale**: Clear, self-documenting names that indicate the proxy chain direction
- **Alternatives**: `antwort-upstream`/`antwort-gateway` considered but "backend/frontend" is more universally understood
