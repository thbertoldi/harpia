from __future__ import annotations

import ast
from pathlib import Path

FORBIDDEN = ("anthropic", "openai", "ollama")
ALLOWED_SEGMENT = "harpia_agents/llm/providers"


def _iter_python_files() -> list[Path]:
    src_root = Path(__file__).parents[2] / "src" / "harpia_agents"
    return [path for path in src_root.rglob("*.py") if "/gen/" not in path.as_posix()]


def test_no_direct_sdk_imports_outside_llm_providers() -> None:
    violations: list[str] = []
    for file_path in _iter_python_files():
        rel = file_path.relative_to(Path(__file__).parents[2]).as_posix()
        if ALLOWED_SEGMENT in rel:
            continue
        module = ast.parse(file_path.read_text(encoding="utf-8"), filename=str(file_path))
        for node in ast.walk(module):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    if alias.name.split(".")[0] in FORBIDDEN:
                        violations.append(f"{rel}: import {alias.name}")
            elif isinstance(node, ast.ImportFrom) and node.module:
                if node.module.split(".")[0] in FORBIDDEN:
                    violations.append(f"{rel}: from {node.module} import ...")
    assert not violations, "Forbidden SDK imports outside llm/providers:\n" + "\n".join(violations)
