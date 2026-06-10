#!/usr/bin/env python3
"""Detect dependency pin and runtime version changes in PR diffs."""

from __future__ import annotations

import json
import re
import tomllib
from fnmatch import fnmatch
from pathlib import PurePosixPath
from typing import Any

from guardlib import changed_files, common_parser, file_at, path_line, write_comment, write_json


MARKER = "<!-- harpia-guard:runtime-and-deps -->"
LABEL = "requires-human-approval"

DEPENDENCY_PATTERNS = (
    "mise.toml",
    "**/go.mod",
    "**/go.sum",
    "**/pyproject.toml",
    "**/uv.lock",
    "**/package.json",
    "**/bun.lock",
    "**/Cargo.toml",
    "go.mod",
    "go.sum",
    "Cargo.toml",
)

BOT_AUTHORS = {
    "dependabot[bot]",
    "renovate[bot]",
    "renovate-bot",
    "renovate",
}


def is_dependency_path(path: str) -> bool:
    return any(fnmatch(path, pattern) for pattern in DEPENDENCY_PATTERNS)


def parse_toml(text: str | None) -> dict[str, Any]:
    if not text:
        return {}
    try:
        return tomllib.loads(text)
    except tomllib.TOMLDecodeError:
        return {}


def parse_json(text: str | None) -> dict[str, Any]:
    if not text:
        return {}
    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        return {}
    return parsed if isinstance(parsed, dict) else {}


def string_map(value: Any) -> dict[str, str]:
    if not isinstance(value, dict):
        return {}
    return {str(key): str(item) for key, item in value.items() if isinstance(item, (str, int, float))}


def compare_maps(
    path: str,
    section: str,
    before: dict[str, str],
    after: dict[str, str],
) -> list[dict[str, object]]:
    changes: list[dict[str, object]] = []
    for name in sorted(before.keys() | after.keys()):
        old = before.get(name)
        new = after.get(name)
        if old == new:
            continue
        changes.append(
            {
                "path": path,
                "section": section,
                "name": name,
                "before": old,
                "after": new,
            }
        )
    return changes


def nested_mapping(root: dict[str, Any], path: tuple[str, ...]) -> dict[str, str]:
    value = nested_value(root, path)
    return string_map(value)


def nested_value(root: dict[str, Any], path: tuple[str, ...]) -> Any:
    current: Any = root
    for part in path:
        if not isinstance(current, dict):
            return None
        current = current.get(part, {})
    return current


def flatten_optional_dependencies(value: Any) -> dict[str, str]:
    if not isinstance(value, dict):
        return {}
    flattened: dict[str, str] = {}
    for group, deps in value.items():
        if isinstance(deps, list):
            for dep in deps:
                if isinstance(dep, str):
                    flattened[f"{group}:{dep}"] = dep
    return flattened


def list_dependencies(value: Any) -> dict[str, str]:
    if not isinstance(value, list):
        return {}
    return {str(item): str(item) for item in value if isinstance(item, str)}


def go_mod_pins(text: str | None) -> dict[str, str]:
    pins: dict[str, str] = {}
    if not text:
        return pins

    in_require = False
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("//"):
            continue
        if line.startswith("go "):
            pins["runtime:go"] = line.split(maxsplit=1)[1]
            continue
        if line.startswith("toolchain "):
            pins["runtime:toolchain"] = line.split(maxsplit=1)[1]
            continue
        if line == "require (":
            in_require = True
            continue
        if in_require and line == ")":
            in_require = False
            continue
        if line.startswith("require "):
            parts = line.split()
            if len(parts) >= 3:
                pins[parts[1]] = parts[2]
            continue
        if in_require:
            parts = line.split()
            if len(parts) >= 2:
                pins[parts[0]] = parts[1]
    return pins


def go_sum_pins(text: str | None) -> dict[str, str]:
    pins: dict[str, str] = {}
    if not text:
        return pins
    for line in text.splitlines():
        parts = line.split()
        if len(parts) >= 2:
            module = parts[0]
            version = parts[1].removesuffix("/go.mod")
            pins[f"{module} {version}"] = version
    return pins


def lockfile_versions(text: str | None) -> dict[str, str]:
    pins: dict[str, str] = {}
    if not text:
        return pins
    package_name: str | None = None
    for raw_line in text.splitlines():
        line = raw_line.strip().strip(",")
        name_match = re.match(r'name\s*=\s*"([^"]+)"', line)
        if name_match:
            package_name = name_match.group(1)
            continue
        version_match = re.match(r'version\s*=\s*"([^"]+)"', line)
        if version_match and package_name:
            pins[package_name] = version_match.group(1)
            package_name = None
            continue
        bun_match = re.match(r'"?([^"@\s][^"]*?)@([^"]+)"?:\s*$', line)
        if bun_match:
            pins[bun_match.group(1)] = bun_match.group(2)
    return pins


def cargo_target_dependencies(root: dict[str, Any]) -> dict[str, str]:
    result: dict[str, str] = {}
    targets = root.get("target", {})
    if not isinstance(targets, dict):
        return result
    for target_name, target in targets.items():
        if not isinstance(target, dict):
            continue
        for section in ("dependencies", "dev-dependencies", "build-dependencies"):
            for name, version in string_map(target.get(section, {})).items():
                result[f"{target_name}:{section}:{name}"] = version
    return result


def package_json_changes(path: str, before: str | None, after: str | None) -> list[dict[str, object]]:
    before_json = parse_json(before)
    after_json = parse_json(after)
    changes: list[dict[str, object]] = []
    for section in (
        "dependencies",
        "devDependencies",
        "peerDependencies",
        "optionalDependencies",
        "engines",
    ):
        changes.extend(
            compare_maps(
                path,
                section,
                string_map(before_json.get(section, {})),
                string_map(after_json.get(section, {})),
            )
        )
    return changes


def pyproject_changes(path: str, before: str | None, after: str | None) -> list[dict[str, object]]:
    before_toml = parse_toml(before)
    after_toml = parse_toml(after)
    changes: list[dict[str, object]] = []
    changes.extend(
        compare_maps(
            path,
            "project.dependencies",
            list_dependencies(nested_value(before_toml, ("project", "dependencies"))),
            list_dependencies(nested_value(after_toml, ("project", "dependencies"))),
        )
    )
    changes.extend(
        compare_maps(
            path,
            "project.optional-dependencies",
            flatten_optional_dependencies(nested_value(before_toml, ("project", "optional-dependencies"))),
            flatten_optional_dependencies(nested_value(after_toml, ("project", "optional-dependencies"))),
        )
    )
    changes.extend(
        compare_maps(
            path,
            "dependency-groups",
            flatten_optional_dependencies(before_toml.get("dependency-groups", {})),
            flatten_optional_dependencies(after_toml.get("dependency-groups", {})),
        )
    )
    changes.extend(
        compare_maps(
            path,
            "build-system.requires",
            list_dependencies(nested_value(before_toml, ("build-system", "requires"))),
            list_dependencies(nested_value(after_toml, ("build-system", "requires"))),
        )
    )
    requires_python_before = nested_value(before_toml, ("project", "requires-python"))
    requires_python_after = nested_value(after_toml, ("project", "requires-python"))
    if requires_python_before != requires_python_after:
        changes.append(
            {
                "path": path,
                "section": "project.requires-python",
                "name": "python",
                "before": requires_python_before,
                "after": requires_python_after,
            }
        )
    return changes


def toml_map_changes(path: str, section: str, before: str | None, after: str | None) -> list[dict[str, object]]:
    before_toml = parse_toml(before)
    after_toml = parse_toml(after)
    return compare_maps(
        path,
        section,
        nested_mapping(before_toml, tuple(section.split("."))),
        nested_mapping(after_toml, tuple(section.split("."))),
    )


def cargo_changes(path: str, before: str | None, after: str | None) -> list[dict[str, object]]:
    before_toml = parse_toml(before)
    after_toml = parse_toml(after)
    changes: list[dict[str, object]] = []
    for section in ("dependencies", "dev-dependencies", "build-dependencies"):
        changes.extend(compare_maps(path, section, string_map(before_toml.get(section, {})), string_map(after_toml.get(section, {}))))
    changes.extend(compare_maps(path, "target.dependencies", cargo_target_dependencies(before_toml), cargo_target_dependencies(after_toml)))
    rust_before = nested_value(before_toml, ("package", "rust-version"))
    rust_after = nested_value(after_toml, ("package", "rust-version"))
    if rust_before != rust_after:
        changes.append(
            {
                "path": path,
                "section": "package.rust-version",
                "name": "rust",
                "before": rust_before,
                "after": rust_after,
            }
        )
    return changes


def file_changes(path: str, before: str | None, after: str | None) -> list[dict[str, object]]:
    name = PurePosixPath(path).name
    if path == "mise.toml":
        return toml_map_changes(path, "tools", before, after)
    if name == "package.json":
        return package_json_changes(path, before, after)
    if name == "pyproject.toml":
        return pyproject_changes(path, before, after)
    if name == "go.mod":
        return compare_maps(path, "go.mod", go_mod_pins(before), go_mod_pins(after))
    if name == "go.sum":
        return compare_maps(path, "go.sum", go_sum_pins(before), go_sum_pins(after))
    if name in {"uv.lock", "bun.lock"}:
        changes = compare_maps(path, name, lockfile_versions(before), lockfile_versions(after))
        if changes:
            return changes
        if before != after:
            return [{"path": path, "section": name, "name": name, "before": "changed", "after": "changed"}]
    if name == "Cargo.toml":
        return cargo_changes(path, before, after)
    return []


def chore_deps_title(title: str) -> bool:
    return bool(re.match(r"^chore\(deps\)(?:!?:|!)", title.strip(), re.IGNORECASE))


def chore_deps_commit_type(pr_title: str, commit_subjects: list[str]) -> bool:
    subjects = [subject for subject in commit_subjects if subject.strip()]
    return chore_deps_title(pr_title) or (
        bool(subjects) and all(chore_deps_title(subject) for subject in subjects)
    )


def exempt_author(author: str) -> bool:
    return author.strip().lower() in BOT_AUTHORS


def detect(base: str, head: str) -> tuple[list[dict[str, object]], list[str]]:
    files = changed_files(base, head)
    changes: list[dict[str, object]] = []
    for path in files:
        if not is_dependency_path(path):
            continue
        changes.extend(file_changes(path, file_at(base, path), file_at(head, path)))
    return changes, files


def comment_body(changes: list[dict[str, object]]) -> str:
    rows = "\n".join(
        "- {path} `{section}` `{name}`: `{before}` -> `{after}`".format(
            path=path_line(str(change["path"])),
            section=change["section"],
            name=change["name"],
            before=change["before"],
            after=change["after"],
        )
        for change in changes
    )
    return f"""### Runtime And Dependency Guard

This PR changes runtime versions or dependency pins. I added `{LABEL}` so a human can explicitly review the version movement before merge.

Detected version changes:
{rows}

Please confirm these changes are intentional, or split them into a dedicated `chore(deps)` PR when they are not part of this issue's requested scope.
"""


def main() -> int:
    parser = common_parser("Detect dependency and runtime version changes in a PR diff")
    parser.add_argument("--pr-title", default="", help="Pull request title")
    parser.add_argument("--author", default="", help="Pull request author login")
    parser.add_argument(
        "--commit-subjects-file",
        default="",
        help="Optional newline-delimited commit subjects for conventional commit checks",
    )
    args = parser.parse_args()

    changes, files = detect(args.base, args.head)
    commit_subjects: list[str] = []
    if args.commit_subjects_file:
        with open(args.commit_subjects_file, encoding="utf-8") as handle:
            commit_subjects = handle.read().splitlines()
    only_dependency_files = bool(files) and all(is_dependency_path(path) for path in files)
    exempt = bool(changes) and (
        exempt_author(args.author)
        or (chore_deps_commit_type(args.pr_title, commit_subjects) and only_dependency_files)
    )
    result = {
        "detected": bool(changes),
        "label_required": bool(changes) and not exempt,
        "exempt": exempt,
        "label": LABEL,
        "only_dependency_files": only_dependency_files,
        "changed_files": files,
        "findings": changes,
    }
    write_json(args.result_file, result)
    if changes and not exempt:
        write_comment(args.comment_file, MARKER, comment_body(changes))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
