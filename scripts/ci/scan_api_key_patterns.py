#!/usr/bin/env python3
"""Fail CI when likely plaintext API keys are committed to source."""

from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path

SKIP_DIRS = {
    ".git",
    ".worktrees",
    ".venv",
    "node_modules",
    "__pycache__",
    "dist",
    "build",
}

SKIP_SUFFIXES = {
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".svg",
    ".pdf",
    ".lock",
}

TEXT_EXTENSIONS = {
    ".go",
    ".py",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".proto",
    ".sql",
    ".yaml",
    ".yml",
    ".toml",
    ".md",
    ".json",
}

SUSPICIOUS_PATTERNS: tuple[re.Pattern[str], ...] = (
    re.compile(r"\bsk-ant-[A-Za-z0-9_-]{16,}\b"),
    re.compile(r"\bsk-proj-[A-Za-z0-9_-]{16,}\b"),
    re.compile(r"\bsk-[A-Za-z0-9_-]{20,}\b"),
    re.compile(r"\bxoxb-[A-Za-z0-9-]{16,}\b"),
    re.compile(r"\bAIza[0-9A-Za-z\-_]{20,}\b"),
)


@dataclass(frozen=True)
class Finding:
    path: Path
    line_number: int
    line: str


def should_scan(path: Path) -> bool:
    if any(part in SKIP_DIRS for part in path.parts):
        return False
    if path.suffix.lower() in SKIP_SUFFIXES:
        return False
    return path.suffix.lower() in TEXT_EXTENSIONS


def scan_file(path: Path) -> list[Finding]:
    findings: list[Finding] = []
    try:
        content = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return findings
    except OSError:
        return findings

    allow_next_line = False
    for line_number, line in enumerate(content.splitlines(), start=1):
        stripped = line.strip()
        if allow_next_line:
            allow_next_line = False
            continue
        if stripped.startswith("# pragma: allowlist secret") or stripped.startswith("// pragma: allowlist secret"):
            allow_next_line = True
            continue
        for pattern in SUSPICIOUS_PATTERNS:
            if pattern.search(line):
                findings.append(Finding(path=path, line_number=line_number, line=line))
                break
    return findings


def scan_repo(root: Path) -> list[Finding]:
    findings: list[Finding] = []
    for path in root.rglob("*"):
        if not path.is_file() or not should_scan(path):
            continue
        findings.extend(scan_file(path))
    return findings


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    findings = scan_repo(root)
    if not findings:
        print("No plaintext API key patterns detected.")
        return 0

    print("Potential plaintext API keys detected:")
    for item in findings:
        rel = item.path.relative_to(root)
        print(f"- {rel}:{item.line_number}: {item.line.strip()}")
    print(
        "Use pragma allowlist comments only for documented fixtures: "
        "`# pragma: allowlist secret` or `// pragma: allowlist secret`."
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
