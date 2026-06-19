from __future__ import annotations

from dataclasses import dataclass

import pytest

from harpia_agents.budget.client import BudgetContext, BudgetedLLMRegistry, BudgetReservation
from harpia_agents.llm import ChatMessage, LLMRegistry


@dataclass
class FakeBudgetClient:
    reserves: list[dict]
    records: list[dict]

    async def reserve(self, **kwargs) -> BudgetReservation:
        self.reserves.append(kwargs)
        return BudgetReservation(reservation_id="reservation-1", idempotency_key=kwargs["idempotency_key"])

    async def record_usage(self, **kwargs) -> None:
        self.records.append(kwargs)


@pytest.mark.asyncio
async def test_budgeted_registry_reserves_and_records_completion_usage() -> None:
    budget = FakeBudgetClient(reserves=[], records=[])
    registry = BudgetedLLMRegistry(
        LLMRegistry.for_testing(model_ids=["gpt-test"], responses=["hello world"]),
        budget_client=budget,
        context=BudgetContext(tenant_id="tenant-1", agent_type="newsletter-writer-senior"),
    )

    result = await registry.complete(
        "gpt-test",
        [ChatMessage(role="user", content="write")],
        max_tokens=32,
    )

    assert result.content == "hello world"
    assert len(budget.reserves) == 1
    assert len(budget.records) == 1
    assert budget.reserves[0]["provider"] == "fake"
    assert budget.records[0]["provider"] == "fake"
    assert budget.records[0]["model_id"] == "gpt-test"
    assert budget.records[0]["reservation"].reservation_id == "reservation-1"
