#!/usr/bin/env python3
"""Detect newly added lint/type/security suppressions in PR diffs."""

from __future__ import annotations

import re
from fnmatch import fnmatch

from guardlib import common_parser, diff_text, file_at, iter_added_lines, path_line, write_comment, write_json


MARKER = "<!-- harpia-guard:rule-disablement -->"
LABEL = "requires-human-approval"

SUPPRESSION_FILES = {
    ".trivyignore",
    ".gitleaksignore",
}

ESLINT_OFF_RE = re.compile(r"""["'](?P<rule>[^"']+)["']\s*:\s*["']off["']""")
RUFF_IGNORE_RE = re.compile(r"""(?P<key>extend-ignore|ignore)\s*=""")
TSCONFIG_LOOSEN_RE = re.compile(
    r""""(?P<option>strict|noImplicitAny|strictNullChecks|noUncheckedIndexedAccess)"\s*:\s*false"""
)
GOLANGCI_DISABLE_RE = re.compile(r"^\s*-\s*(?P<linter>[a-zA-Z0-9_-]+)\s*(?:#.*)?$")


def is_suppression_file(path: str) -> bool:
    name = path.rsplit("/", 1)[-1]
    lowered = path.lower()
    return (
        name in SUPPRESSION_FILES
        or "suppression" in lowered
        or "suppressions" in lowered
    )


def relevant(path: str) -> bool:
    return any(
        fnmatch(path, pattern)
        for pattern in (
            "frontend/eslint.config.js",
            "frontend/.eslintrc*",
            "frontend/tsconfig.json",
            "frontend/**/*.ts",
            "frontend/**/*.js",
            "frontend/**/*.svelte",
            "agent-runtime/pyproject.toml",
            "agent-runtime/**/*.py",
            "control-plane/.golangci.yml",
            "control-plane/**/*.go",
            "**/.trivyignore",
            "**/.gitleaksignore",
            ".trivyignore",
            ".gitleaksignore",
            "**/*suppression*",
            "**/*suppressions*",
        )
    )


def finding(kind: str, path: str, line_number: int, detail: str) -> dict[str, object]:
    return {
        "kind": kind,
        "path": path,
        "line": line_number,
        "detail": detail,
    }


def in_yaml_list_block(text: str | None, line_number: int, key: str) -> bool:
    if not text:
        return False

    lines = text.splitlines()
    index = line_number - 1
    if index < 0 or index >= len(lines):
        return False

    target = lines[index]
    target_indent = len(target) - len(target.lstrip(" "))
    for prior in reversed(lines[:index]):
        stripped = prior.strip()
        if not stripped or stripped.startswith("#"):
            continue

        indent = len(prior) - len(prior.lstrip(" "))
        if indent < target_indent:
            return stripped == f"{key}:"
        if indent == target_indent and not stripped.startswith("-"):
            return False

    return False


def detect(base: str, head: str) -> list[dict[str, object]]:
    findings: list[dict[str, object]] = []
    golangci_disable_block = False
    golangci_head_text: str | None = None
    last_path: str | None = None

    for added in iter_added_lines(diff_text(base, head)):
        path = added.path
        if path != last_path:
            golangci_disable_block = False
            last_path = path
        if not relevant(path):
            continue

        stripped = added.text.strip()
        if not stripped:
            continue

        if path.startswith("frontend/") and (
            path.endswith("eslint.config.js") or "/.eslintrc" in path or path.endswith(".eslintrc")
        ):
            match = ESLINT_OFF_RE.search(added.text)
            if match:
                findings.append(
                    finding(
                        "eslint-rule-off",
                        path,
                        added.line_number,
                        f"ESLint rule disabled: `{match.group('rule')}`",
                    )
                )

        if path == "frontend/tsconfig.json":
            match = TSCONFIG_LOOSEN_RE.search(added.text)
            if match:
                findings.append(
                    finding(
                        "typescript-check-loosened",
                        path,
                        added.line_number,
                        f"TypeScript compiler check loosened: `{match.group('option')}` set to `false`",
                    )
                )

        if path.startswith("agent-runtime/") and path.endswith(".py"):
            if re.search(r"#\s*noqa\b", added.text):
                findings.append(
                    finding("python-noqa", path, added.line_number, "Python `# noqa` suppression added")
                )
            if re.search(r"#\s*type:\s*ignore\b", added.text):
                findings.append(
                    finding(
                        "python-type-ignore",
                        path,
                        added.line_number,
                        "Python `# type: ignore` suppression added",
                    )
                )

        if path.startswith("agent-runtime/") and path.endswith("pyproject.toml"):
            if RUFF_IGNORE_RE.search(added.text):
                findings.append(
                    finding(
                        "ruff-ignore",
                        path,
                        added.line_number,
                        "Ruff ignore list changed; verify the rule disablement is justified",
                    )
                )

        if path.startswith("frontend/") and path.endswith((".ts", ".js", ".svelte")):
            if re.search(r"//\s*eslint-disable-next-line\b", added.text):
                findings.append(
                    finding(
                        "eslint-disable-next-line",
                        path,
                        added.line_number,
                        "Inline `eslint-disable-next-line` suppression added",
                    )
                )
            if re.search(r"//\s*@ts-ignore\b", added.text):
                findings.append(
                    finding("ts-ignore", path, added.line_number, "Inline `@ts-ignore` suppression added")
                )

        if path.startswith("control-plane/") and path.endswith(".go"):
            if re.search(r"//\s*nolint:", added.text):
                findings.append(
                    finding("go-nolint", path, added.line_number, "Go `//nolint:` suppression added")
                )

        if path == "control-plane/.golangci.yml":
            if re.match(r"^\s*disable:\s*$", added.text):
                golangci_disable_block = True
            elif golangci_disable_block:
                match = GOLANGCI_DISABLE_RE.match(added.text)
                if match:
                    findings.append(
                        finding(
                            "golangci-disable",
                            path,
                            added.line_number,
                            f"golangci-lint rule disabled: `{match.group('linter')}`",
                        )
                    )
                elif stripped and not stripped.startswith("#"):
                    golangci_disable_block = False
            else:
                match = GOLANGCI_DISABLE_RE.match(added.text)
                if match:
                    if golangci_head_text is None:
                        golangci_head_text = file_at(head, path)
                    if in_yaml_list_block(golangci_head_text, added.line_number, "disable"):
                        findings.append(
                            finding(
                                "golangci-disable",
                                path,
                                added.line_number,
                                f"golangci-lint rule disabled: `{match.group('linter')}`",
                            )
                        )

        if is_suppression_file(path) and not stripped.startswith("#"):
            findings.append(
                finding(
                    "suppression-file-entry",
                    path,
                    added.line_number,
                    "Suppression/ignore file entry added",
                )
            )

    return findings


def comment_body(findings: list[dict[str, object]]) -> str:
    rows = "\n".join(
        f"- {path_line(str(item['path']), int(item['line']))}: {item['detail']}" for item in findings
    )
    return f"""### Rule-Disablement Guard

This PR adds lint, type, or security suppressions. I added `{LABEL}` so a human can verify the suppression is intentional before merge.

Detected suppressions:
{rows}

Please leave a written justification in the PR discussion for each suppression, or remove the suppression and fix the underlying issue.
"""


def main() -> int:
    parser = common_parser("Detect new rule disablements in a PR diff")
    args = parser.parse_args()

    findings = detect(args.base, args.head)
    result = {
        "detected": bool(findings),
        "label_required": bool(findings),
        "label": LABEL,
        "findings": findings,
    }
    write_json(args.result_file, result)
    if findings:
        write_comment(args.comment_file, MARKER, comment_body(findings))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
