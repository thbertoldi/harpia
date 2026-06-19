"""Control-plane BudgetPolicyService client and LLM registry wrapper."""

from __future__ import annotations

import os
import uuid
from collections.abc import AsyncIterator, Sequence
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Protocol

from connectrpc.errors import ConnectError
from google.protobuf.timestamp_pb2 import Timestamp
from harpia.budget.v1.budget_connect import BudgetPolicyServiceClient
from harpia.budget.v1.budget_pb2 import (
    Money,
    RecordUsageRequest,
    ReserveBudgetRequest,
)

from harpia_agents.llm.provider import ChatMessage, CompletionChunk, CompletionResult
from harpia_agents.llm.registry import LLMRegistry, ModelInfo


def _budget_base_url() -> str:
    explicit = os.environ.get("HARPIA_CONTROL_PLANE_INTERNAL_URL", "").strip()
    if explicit:
        return explicit.rstrip("/")
    public = os.environ.get("HARPIA_CONTROL_PLANE_URL", "http://localhost:8080").rstrip("/")
    return f"{public}/internal"


def _default_internal_auth_token() -> str:
    configured = os.environ.get("HARPIA_INTERNAL_AUTH_TOKEN", "").strip()
    if configured:
        return configured
    if os.environ.get("HARPIA_ALLOW_DEV_AUTH", "").lower() in {"1", "true", "yes"}:
        return "dev-internal-token"
    return ""


def budget_policy_enabled() -> bool:
    return os.environ.get("HARPIA_BUDGET_POLICY_ENABLED", "").lower() in {"1", "true", "yes"}


def _timestamp(value: datetime) -> Timestamp:
    ts = Timestamp()
    ts.FromDatetime(value.astimezone(UTC))
    return ts


def _money_from_usd(value: float) -> Money:
    micros = max(0, round(value * 1_000_000))
    return Money(currency="USD", amount_micros=micros)


@dataclass(frozen=True, slots=True)
class BudgetContext:
    tenant_id: str
    agent_type: str
    task_id: str = ""
    subtask_id: str = ""
    plan_execution_id: str = ""
    step_execution_id: str = ""


@dataclass(frozen=True, slots=True)
class BudgetReservation:
    reservation_id: str
    idempotency_key: str


class _BudgetClient(Protocol):
    async def reserve(
        self,
        *,
        context: BudgetContext,
        provider: str,
        estimated_cost_usd: float,
        idempotency_key: str,
    ) -> BudgetReservation: ...

    async def record_usage(
        self,
        *,
        context: BudgetContext,
        reservation: BudgetReservation,
        provider: str,
        model_id: str,
        input_tokens: int,
        output_tokens: int,
        cost_usd: float,
    ) -> None: ...


class BudgetClient:
    """Small wrapper around the generated BudgetPolicyService client."""

    def __init__(
        self,
        client: BudgetPolicyServiceClient | None = None,
        *,
        base_url: str | None = None,
        auth_token: str | None = None,
        timeout_ms: int = 5000,
    ) -> None:
        self._client = client or BudgetPolicyServiceClient(base_url=base_url or _budget_base_url())
        self._auth_token = auth_token if auth_token is not None else _default_internal_auth_token()
        self._timeout_ms = timeout_ms

    def _headers(self, tenant_id: str) -> dict[str, str]:
        if not self._auth_token:
            raise ConnectError("HARPIA_INTERNAL_AUTH_TOKEN is required for budget policy")
        return {
            "Authorization": f"Bearer {self._auth_token}",
            "X-Tenant-ID": tenant_id,
        }

    async def reserve(
        self,
        *,
        context: BudgetContext,
        provider: str,
        estimated_cost_usd: float,
        idempotency_key: str,
    ) -> BudgetReservation:
        expires_at = datetime.now(UTC) + timedelta(minutes=15)
        response = await self._client.reserve_budget(
            ReserveBudgetRequest(
                tenant_id=context.tenant_id,
                provider=provider,
                task_id=context.task_id,
                subtask_id=context.subtask_id,
                plan_execution_id=context.plan_execution_id,
                step_execution_id=context.step_execution_id,
                agent_type=context.agent_type,
                estimated_cost=_money_from_usd(estimated_cost_usd),
                idempotency_key=idempotency_key,
                expires_at=_timestamp(expires_at),
            ),
            headers=self._headers(context.tenant_id),
            timeout_ms=self._timeout_ms,
        )
        return BudgetReservation(
            reservation_id=response.reservation_id,
            idempotency_key=idempotency_key,
        )

    async def record_usage(
        self,
        *,
        context: BudgetContext,
        reservation: BudgetReservation,
        provider: str,
        model_id: str,
        input_tokens: int,
        output_tokens: int,
        cost_usd: float,
    ) -> None:
        await self._client.record_usage(
            RecordUsageRequest(
                tenant_id=context.tenant_id,
                reservation_id=reservation.reservation_id,
                idempotency_key=reservation.idempotency_key,
                task_id=context.task_id,
                subtask_id=context.subtask_id,
                plan_execution_id=context.plan_execution_id,
                step_execution_id=context.step_execution_id,
                agent_type=context.agent_type,
                provider=provider,
                model=model_id,
                input_tokens=input_tokens,
                output_tokens=output_tokens,
                cost=_money_from_usd(cost_usd),
                occurred_at=_timestamp(datetime.now(UTC)),
            ),
            headers=self._headers(context.tenant_id),
            timeout_ms=self._timeout_ms,
        )


class BudgetedLLMRegistry:
    """LLMRegistry-compatible wrapper that reserves and records per-call usage."""

    def __init__(
        self,
        registry: LLMRegistry,
        *,
        budget_client: _BudgetClient,
        context: BudgetContext,
    ) -> None:
        self._registry = registry
        self._budget_client = budget_client
        self._context = context

    def resolve(self, model_id: str):
        return self._registry.resolve(model_id)

    def list_models(self) -> list[ModelInfo]:
        return self._registry.list_models()

    async def complete(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> CompletionResult:
        provider = self._registry.resolve(model_id)
        input_tokens = provider.count_tokens(model_id, messages)
        estimated_output_tokens = max_tokens if max_tokens is not None else 1024
        estimated_cost = provider.estimate_cost(model_id, input_tokens, estimated_output_tokens)
        reservation = await self._budget_client.reserve(
            context=self._context,
            provider=provider.name,
            estimated_cost_usd=estimated_cost,
            idempotency_key=str(uuid.uuid4()),
        )
        result = await self._registry.complete(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )
        await self._budget_client.record_usage(
            context=self._context,
            reservation=reservation,
            provider=result.provider,
            model_id=result.model_id,
            input_tokens=result.usage.input_tokens,
            output_tokens=result.usage.output_tokens,
            cost_usd=result.usage.cost_usd,
        )
        return result

    async def stream(
        self,
        model_id: str,
        messages: Sequence[ChatMessage],
        *,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> AsyncIterator[CompletionChunk]:
        provider = self._registry.resolve(model_id)
        input_tokens = provider.count_tokens(model_id, messages)
        estimated_output_tokens = max_tokens if max_tokens is not None else 1024
        estimated_cost = provider.estimate_cost(model_id, input_tokens, estimated_output_tokens)
        reservation = await self._budget_client.reserve(
            context=self._context,
            provider=provider.name,
            estimated_cost_usd=estimated_cost,
            idempotency_key=str(uuid.uuid4()),
        )
        recorded = False
        async for chunk in self._registry.stream(
            model_id=model_id,
            messages=messages,
            max_tokens=max_tokens,
            temperature=temperature,
        ):
            if chunk.usage is not None and not recorded:
                await self._budget_client.record_usage(
                    context=self._context,
                    reservation=reservation,
                    provider=provider.name,
                    model_id=model_id,
                    input_tokens=chunk.usage.input_tokens,
                    output_tokens=chunk.usage.output_tokens,
                    cost_usd=chunk.usage.cost_usd,
                )
                recorded = True
            yield chunk
