"""Type-check sentinel for the Subtask domain contract.

This file is intentionally not a pytest module. Run it with mypy alongside
graph.py so field renames or removals fail at type-check time.
"""

from harpia_agents.graph import Subtask


def reference_every_subtask_field(subtask: Subtask) -> tuple[str, str, str, bool]:
    return (
        subtask.title,
        subtask.description,
        subtask.suggested_agent_type,
        subtask.needs_human_input,
    )
