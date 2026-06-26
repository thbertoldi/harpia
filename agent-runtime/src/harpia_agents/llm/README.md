# harpia_agents.llm

This module defines Harpia's canonical LLM abstraction for agent orchestration: providers implement one protocol (`LLMProvider`), while callers route all model usage through `LLMRegistry` so switching providers is a manifest/config change instead of node-level SDK rewrites.

## Provider Matrix

| Provider | Models (current) |
|---|---|
| Anthropic | `claude-3-5-sonnet-20241022`, `claude-3-opus-20240229`, `claude-3-haiku-20240307` |
| DeepSeek | `deepseek-v4-flash`, `deepseek-v4-pro` |
| OpenAI | `gpt-4o`, `gpt-4o-mini`, `openai-gpt-4o-mini` |
| Ollama | `llama3:70b` |

## Add A Provider (5 Steps)

1. Create `harpia_agents/llm/providers/<provider>.py` implementing `LLMProvider`.
2. Add model pricing constants in `harpia_agents/llm/pricing.py`.
3. Export the provider in `harpia_agents/llm/providers/__init__.py`.
4. Register the provider in `LLMRegistry.default()`.
5. Add provider tests under `agent-runtime/tests/llm/` (complete, stream, errors, token counting, cost math).

Tenant-scoped API key resolution is intentionally out of scope for this PR; #33 will provide per-call credential wiring at registry construction/use time.
