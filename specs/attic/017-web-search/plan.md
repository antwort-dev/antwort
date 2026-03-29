# Implementation Plan: Web Search Provider

**Branch**: `017-web-search` | **Date**: 2026-02-20 | **Spec**: [spec.md](spec.md)

## Project Structure

```text
pkg/tools/builtins/websearch/
├── provider.go        # WebSearchProvider implementing FunctionProvider
├── adapter.go         # SearchAdapter interface
├── searxng.go         # SearXNG adapter
├── provider_test.go   # Tests with mock search backend
```
