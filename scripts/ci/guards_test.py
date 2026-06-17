#!/usr/bin/env python3
"""Synthetic PR diff tests for the guard detectors."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import contextmanager
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import guard_rule_disablement
import guard_runtime_and_deps
import scan_api_key_patterns
import guard_scope_inflation


def git(args: list[str], cwd: Path) -> str:
    return subprocess.check_output(["git", *args], cwd=cwd, text=True)


def write(repo: Path, path: str, content: str) -> None:
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content)


def commit(repo: Path, message: str) -> str:
    git(["add", "."], repo)
    git(["commit", "-m", message], repo)
    return git(["rev-parse", "HEAD"], repo).strip()


@contextmanager
def synthetic_repo():
    old_cwd = Path.cwd()
    with tempfile.TemporaryDirectory() as raw_dir:
        repo = Path(raw_dir)
        git(["init"], repo)
        git(["config", "user.email", "ci@example.test"], repo)
        git(["config", "user.name", "CI Test"], repo)
        os.chdir(repo)
        try:
            yield repo
        finally:
            os.chdir(old_cwd)


class GuardTests(unittest.TestCase):
    def test_rule_disablement_detects_inline_eslint_suppression(self) -> None:
        with synthetic_repo() as repo:
            write(repo, "frontend/src/example.ts", "export const value = 1;\n")
            base = commit(repo, "base")
            write(
                repo,
                "frontend/src/example.ts",
                "export const value = 1;\n// eslint-disable-next-line no-console\nconsole.log(value);\n",
            )
            head = commit(repo, "add suppression")

            findings = guard_rule_disablement.detect(base, head)

        self.assertTrue(any(item["kind"] == "eslint-disable-next-line" for item in findings))

    def test_rule_disablement_detects_security_ignore_entry(self) -> None:
        with synthetic_repo() as repo:
            write(repo, ".trivyignore", "# accepted entries\n")
            base = commit(repo, "base")
            write(repo, ".trivyignore", "# accepted entries\nCVE-2026-0001\n")
            head = commit(repo, "add trivy ignore")

            findings = guard_rule_disablement.detect(base, head)

        self.assertTrue(any(item["kind"] == "suppression-file-entry" for item in findings))

    def test_rule_disablement_detects_added_golangci_disable_entry(self) -> None:
        with synthetic_repo() as repo:
            write(
                repo,
                "control-plane/.golangci.yml",
                "linters:\n  disable:\n    - errcheck\n",
            )
            base = commit(repo, "base")
            write(
                repo,
                "control-plane/.golangci.yml",
                "linters:\n  disable:\n    - errcheck\n    - govet\n",
            )
            head = commit(repo, "add golangci disable")

            findings = guard_rule_disablement.detect(base, head)

        self.assertTrue(any(item["kind"] == "golangci-disable" for item in findings))

    def test_runtime_guard_detects_dependency_and_runtime_versions(self) -> None:
        with synthetic_repo() as repo:
            write(repo, "mise.toml", "[tools]\nnode = \"22\"\n")
            write(
                repo,
                "frontend/package.json",
                json.dumps({"devDependencies": {"prettier-plugin-svelte": "3.3.0"}}, indent=2),
            )
            base = commit(repo, "base")
            write(repo, "mise.toml", "[tools]\nnode = \"24\"\n")
            write(
                repo,
                "frontend/package.json",
                json.dumps({"devDependencies": {"prettier-plugin-svelte": "4.0.0"}}, indent=2),
            )
            head = commit(repo, "change versions")

            findings, files = guard_runtime_and_deps.detect(base, head)

        names = {item["name"] for item in findings}
        self.assertIn("node", names)
        self.assertIn("prettier-plugin-svelte", names)
        self.assertEqual(files, ["frontend/package.json", "mise.toml"])

    def test_runtime_guard_exempts_chore_deps_when_only_deps_changed(self) -> None:
        self.assertTrue(
            guard_runtime_and_deps.chore_deps_commit_type(
                "feat(ui): update dependencies",
                ["chore(deps): bump frontend deps"],
            )
        )

    def test_scope_guard_warns_without_requiring_label(self) -> None:
        with synthetic_repo() as repo:
            write(repo, "src/a.txt", "a\n")
            base = commit(repo, "base")
            for index in range(4):
                write(repo, f"src/file-{index}.txt", f"{index}\n")
            head = commit(repo, "touch too many files")

            issue_body = """## Scope
- [ ] `src/a.txt`

Only src/a.txt is expected.
"""
            labels = [{"name": "complexity:trivial"}]
            result = guard_scope_inflation.detect(base, head, issue_body, labels)

        self.assertTrue(result["detected"])
        self.assertFalse(result["label_required"])
        self.assertTrue(any(item["kind"] == "trivial-complexity-file-count" for item in result["warnings"]))

    def test_api_key_scanner_detects_realistic_key_pattern(self) -> None:
        with tempfile.TemporaryDirectory() as raw_dir:
            repo = Path(raw_dir)
            (repo / "src").mkdir(parents=True, exist_ok=True)
            (repo / "src" / "sample.py").write_text('token = "sk-ant-thisshouldtrigger123456789"\n')

            findings = scan_api_key_patterns.scan_repo(repo)

        self.assertEqual(len(findings), 1)
        self.assertIn("sample.py", str(findings[0].path))

    def test_api_key_scanner_respects_allowlist_pragma(self) -> None:
        with tempfile.TemporaryDirectory() as raw_dir:
            repo = Path(raw_dir)
            (repo / "src").mkdir(parents=True, exist_ok=True)
            (repo / "src" / "fixture.py").write_text(
                "# pragma: allowlist secret\n"
                'token = "sk-ant-thisisfixturevalue123456"\n'
            )

            findings = scan_api_key_patterns.scan_repo(repo)

        self.assertEqual(findings, [])


if __name__ == "__main__":
    unittest.main()
