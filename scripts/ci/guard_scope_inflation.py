#!/usr/bin/env python3
"""Warn when a PR appears broader than its linked issue scope."""

from __future__ import annotations

import json
import re
from fnmatch import fnmatch
from pathlib import PurePosixPath

from guardlib import changed_files, common_parser, path_line, write_comment, write_json


MARKER = "<!-- harpia-guard:scope-inflation -->"

CHECKBOX_RE = re.compile(r"^\s*-\s+\[[ xX]\]\s+(?P<text>.+)$")
PATH_TOKEN_RE = re.compile(
    r"`(?P<backtick>[^`]+\.[A-Za-z0-9][^`]*)`|(?P<bare>(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.*/{}-]+)"
)


def issue_section(body: str, heading: str) -> str:
    lines = body.splitlines()
    collecting = False
    collected: list[str] = []
    target = heading.strip().lower()
    for line in lines:
        if line.startswith("## "):
            current = line.lstrip("#").strip().lower()
            if collecting and current != target:
                break
            collecting = current == target
            continue
        if collecting:
            collected.append(line)
    return "\n".join(collected)


def path_tokens(text: str) -> set[str]:
    paths: set[str] = set()
    for match in PATH_TOKEN_RE.finditer(text):
        token = (match.group("backtick") or match.group("bare") or "").strip()
        if not token or " " in token:
            continue
        token = token.strip(".,;:()[]")
        if token.startswith(("http://", "https://")):
            continue
        paths.add(token)
    return paths


def scope_items(scope: str) -> list[str]:
    items: list[str] = []
    for line in scope.splitlines():
        match = CHECKBOX_RE.match(line)
        if match:
            items.append(match.group("text").strip())
    return items


def implied_file_count(scope: str) -> int:
    items = scope_items(scope)
    explicit_paths = path_tokens("\n".join(items))
    return max(len(items), len(explicit_paths), 1)


def mentioned_patterns(body: str) -> set[str]:
    patterns: set[str] = set()
    for token in path_tokens(body):
        if "*" in token:
            patterns.add(token)
            continue
        if token.endswith("/"):
            patterns.add(token)
            continue
        path = PurePosixPath(token)
        if path.suffix or "/" not in token:
            parent = str(path.parent)
            if parent != ".":
                patterns.add(parent + "/")
            patterns.add(token)
        else:
            patterns.add(token.rstrip("/") + "/")
    return {pattern for pattern in patterns if pattern and pattern != "/"}


def path_is_mentioned(path: str, patterns: set[str]) -> bool:
    for pattern in patterns:
        if "*" in pattern and fnmatch(path, pattern):
            return True
        if pattern.endswith("/") and path.startswith(pattern):
            return True
        if path == pattern:
            return True
    return False


def complexity_label(labels: list[dict[str, object]]) -> str | None:
    for label in labels:
        name = str(label.get("name", ""))
        if name.startswith("complexity:"):
            return name
    return None


def load_issue(path: str) -> tuple[str, list[dict[str, object]]]:
    with open(path, encoding="utf-8") as handle:
        issue = json.load(handle)
    return str(issue.get("body") or ""), list(issue.get("labels") or [])


def detect(base: str, head: str, issue_body: str, labels: list[dict[str, object]]) -> dict[str, object]:
    files = changed_files(base, head)
    scope = issue_section(issue_body, "Scope")
    implied_count = implied_file_count(scope) if scope else 1
    complexity = complexity_label(labels)
    patterns = mentioned_patterns(issue_body)
    out_of_scope = [path for path in files if patterns and not path_is_mentioned(path, patterns)]

    warnings: list[dict[str, object]] = []
    if len(files) > implied_count * 2:
        warnings.append(
            {
                "kind": "file-count",
                "message": f"PR touches {len(files)} files; linked issue scope implies about {implied_count}.",
            }
        )
    if out_of_scope:
        warnings.append(
            {
                "kind": "paths-not-mentioned",
                "message": "PR touches paths not mentioned in the linked issue body.",
                "paths": out_of_scope,
            }
        )
    if complexity == "complexity:trivial" and len(files) > 3:
        warnings.append(
            {
                "kind": "trivial-complexity-file-count",
                "message": f"`complexity:trivial` issue touches {len(files)} files.",
            }
        )

    return {
        "detected": bool(warnings),
        "label_required": False,
        "changed_files": files,
        "implied_scope_file_count": implied_count,
        "complexity": complexity,
        "warnings": warnings,
    }


def comment_body(result: dict[str, object]) -> str:
    lines = [
        "### Scope-Inflation Guard",
        "",
        "This PR may be broader than its linked issue. This is a warning only; no merge-blocking label was added.",
        "",
        "Signals:",
    ]
    for warning in result["warnings"]:
        lines.append(f"- {warning['message']}")
        for path in warning.get("paths", []):
            lines.append(f"  - {path_line(path)}")
    lines.extend(
        [
            "",
            "Please confirm the extra scope is intentional, or split unrelated changes before review.",
        ]
    )
    return "\n".join(lines)


def main() -> int:
    parser = common_parser("Warn on possible PR scope inflation")
    parser.add_argument("--issue-json", required=True, help="JSON from gh issue view")
    args = parser.parse_args()

    body, labels = load_issue(args.issue_json)
    result = detect(args.base, args.head, body, labels)
    write_json(args.result_file, result)
    if result["detected"]:
        write_comment(args.comment_file, MARKER, comment_body(result))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
