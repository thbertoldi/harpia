"""Agent runner registry keyed by manifest id."""

from __future__ import annotations

import os
from collections.abc import Awaitable, Callable, Mapping

from harpia.artifacts.v1.artifacts_pb2 import LinkedInPostDraft, NewsList, TextDraft

from harpia_agents.agents.linkedin_voice import MANIFEST as LINKEDIN_MANIFEST
from harpia_agents.agents.linkedin_voice import (
    run as run_linkedin_voice,
)
from harpia_agents.agents.newsletter_writer import MANIFEST as NEWSLETTER_MANIFEST
from harpia_agents.agents.newsletter_writer import (
    AgentRunResult as NewsletterAgentRunResult,
)
from harpia_agents.agents.newsletter_writer import (
    run as run_newsletter_writer,
)
from harpia_agents.budget import (
    BudgetClient,
    BudgetContext,
    BudgetedLLMRegistry,
    budget_policy_enabled,
)
from harpia_agents.llm import LLMRegistry
from harpia_agents.llm.errors import ModelNotFoundError
from harpia_agents.llm.provider import LLMProvider
from harpia_agents.llm.providers.anthropic import AnthropicProvider
from harpia_agents.llm.providers.deepseek import DeepSeekProvider
from harpia_agents.llm.providers.ollama import OllamaProvider
from harpia_agents.llm.providers.openai import OpenAIProvider
from harpia_agents.llm.resolver import ResolvedProviderCredentials, TenantLLMResolver

type AgentInput = NewsList | TextDraft | Mapping[str, object]
type AgentRunResult = NewsletterAgentRunResult | LinkedInPostDraft

AgentRunner = Callable[
    [AgentInput, LLMRegistry, str, Mapping[str, str] | None],
    Awaitable[AgentRunResult],
]


def _default_provider_for_manifest(manifest_id: str) -> str | None:
    providers = {
        "newsletter-writer-senior": "deepseek",
        "linkedin-voice-senior": "deepseek",
    }
    return providers.get(manifest_id)


def _resolve_tenant_model_hint(elicitation_responses: Mapping[str, str] | None) -> str:
    if not elicitation_responses:
        return ""
    return str(elicitation_responses.get("requested_model", "")).strip()


def _provider_for_model_id(model_id: str) -> str | None:
    provider_models = {
        AnthropicProvider.name: AnthropicProvider.supported_models,
        DeepSeekProvider.name: DeepSeekProvider.supported_models,
        OpenAIProvider.name: OpenAIProvider.supported_models,
        OllamaProvider.name: OllamaProvider.supported_models,
    }
    for provider_name, model_ids in provider_models.items():
        if model_id in model_ids:
            return provider_name
    return None


def _model_id_for_resolved_provider(
    model_id: str,
    resolved: ResolvedProviderCredentials,
) -> str:
    provider_name = _provider_for_model_id(model_id)
    if provider_name is None:
        raise ModelNotFoundError(f"unknown model_id: {model_id}")
    if provider_name != resolved.provider:
        raise ModelNotFoundError(
            f"model_id {model_id} is routed to provider {provider_name}, "
            f"not resolved provider {resolved.provider}"
        )
    return model_id


def _allow_local_llm_fallback() -> bool:
    return os.environ.get("HARPIA_ALLOW_DEV_AUTH", "").lower() in {"1", "true", "yes"}


def _provider_env_api_key(provider: str | None) -> str:
    provider_name = (provider or "").strip().upper()
    if not provider_name:
        return ""
    return os.environ.get(f"{provider_name}_API_KEY", "").strip()


def _build_registry_with_tenant_credentials(
    resolved: ResolvedProviderCredentials,
) -> LLMRegistry:
    api_key = resolved.api_key.reveal()
    providers: list[LLMProvider] = [
        DeepSeekProvider(api_key=api_key if resolved.provider == "deepseek" else None),
        OpenAIProvider(api_key=api_key if resolved.provider == "openai" else None),
        AnthropicProvider(api_key=api_key if resolved.provider == "anthropic" else None),
        OllamaProvider(),
    ]
    return LLMRegistry(providers)


def _model_id_for_run(
    manifest_id: str,
    *,
    resolved: ResolvedProviderCredentials | None,
    elicitation_responses: Mapping[str, str] | None,
) -> str:
    model_hint = _resolve_tenant_model_hint(elicitation_responses)
    if model_hint:
        if resolved is not None:
            return _model_id_for_resolved_provider(model_hint, resolved)
        return model_hint
    if resolved is not None and resolved.default_model:
        return _model_id_for_resolved_provider(resolved.default_model, resolved)
    return _MANIFEST_MODEL_IDS[manifest_id]


async def _run_newsletter(
    input_payload: AgentInput,
    llm_registry: LLMRegistry,
    model_id: str,
    elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_newsletter_writer(
        input_payload,
        llm_registry=llm_registry,
        model_id=model_id,
        elicitation_responses=elicitation_responses,
    )


async def _run_linkedin(
    input_payload: AgentInput,
    llm_registry: LLMRegistry,
    model_id: str,
    _elicitation_responses: Mapping[str, str] | None,
) -> AgentRunResult:
    return await run_linkedin_voice(
        input_payload,
        llm_registry=llm_registry,
        model_id=model_id,
    )


_RUNNERS: dict[str, AgentRunner] = {
    "newsletter-writer-senior": _run_newsletter,
    "linkedin-voice-senior": _run_linkedin,
}

_MANIFEST_MODEL_IDS: dict[str, str] = {
    "newsletter-writer-senior": NEWSLETTER_MANIFEST.model_id,
    "linkedin-voice-senior": LINKEDIN_MANIFEST.model_id,
}


def has_runner(manifest_id: str) -> bool:
    return manifest_id in _RUNNERS


async def run_registered_agent(
    manifest_id: str,
    *,
    tenant_id: str | None = None,
    input_payload: AgentInput | None = None,
    input_news_list: AgentInput | None = None,
    llm_registry: LLMRegistry | None = None,
    elicitation_responses: Mapping[str, str] | None = None,
) -> AgentRunResult:
    try:
        runner = _RUNNERS[manifest_id]
    except KeyError as exc:
        raise ValueError(f"unsupported manifest id: {manifest_id}") from exc

    resolved_input = input_payload if input_payload is not None else input_news_list
    if resolved_input is None:
        raise ValueError("input_payload is required")

    registry = llm_registry or LLMRegistry.default()
    resolved_credentials: ResolvedProviderCredentials | None = None

    if tenant_id:
        provider = _default_provider_for_manifest(manifest_id)
        if provider:
            try:
                resolved_credentials = await TenantLLMResolver().resolve(
                    tenant_id=tenant_id,
                    provider=provider,
                    requested_model=_resolve_tenant_model_hint(elicitation_responses),
                )
                if not resolved_credentials.api_key.is_empty():
                    registry = _build_registry_with_tenant_credentials(resolved_credentials)
            except Exception:
                if not _allow_local_llm_fallback() or not _provider_env_api_key(provider):
                    raise

    model_id = _model_id_for_run(
        manifest_id,
        resolved=resolved_credentials,
        elicitation_responses=elicitation_responses,
    )
    registry.resolve(model_id)
    if tenant_id and budget_policy_enabled():
        registry = BudgetedLLMRegistry(
            registry,
            budget_client=BudgetClient(),
            context=BudgetContext(
                tenant_id=tenant_id,
                agent_type=manifest_id,
            ),
        )

    return await runner(
        resolved_input,
        registry,
        model_id,
        elicitation_responses,
    )
