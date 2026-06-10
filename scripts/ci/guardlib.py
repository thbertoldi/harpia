#!/usr/bin/env python3
"""Shared helpers for PR guard scripts."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


@dataclass(frozen=True)
class AddedLine:
    path: str
    line_number: int
    text: str


def run_git(args: list[str]) -> str:
    return subprocess.check_output(["git", *args], text=True)


def changed_files(base: str, head: str) -> list[str]:
    output = run_git(
        ["diff", "--name-only", "--diff-filter=ACMRT", base, head, "--"]
    )
    return sorted(line for line in output.splitlines() if line)


def diff_text(base: str, head: str, paths: Iterable[str] | None = None) -> str:
    args = ["diff", "--find-renames", "--unified=0", base, head, "--"]
    if paths:
        args.extend(paths)
    return run_git(args)


def file_at(rev: str, path: str) -> str | None:
    try:
        return run_git(["show", f"{rev}:{path}"])
    except subprocess.CalledProcessError:
        return None


def iter_added_lines(diff: str) -> Iterable[AddedLine]:
    current_path: str | None = None
    new_line_number: int | None = None

    for line in diff.splitlines():
        if line.startswith("+++ "):
            raw_path = line[4:]
            current_path = None if raw_path == "/dev/null" else raw_path.removeprefix("b/")
            continue
        if line.startswith("@@ "):
            match = re.search(r"\+(\d+)(?:,\d+)?", line)
            new_line_number = int(match.group(1)) if match else None
            continue
        if current_path is None or new_line_number is None:
            continue
        if line.startswith("+"):
            yield AddedLine(current_path, new_line_number, line[1:])
            new_line_number += 1
        elif line.startswith("-"):
            continue
        else:
            new_line_number += 1


def write_json(path: str | os.PathLike[str], data: object) -> None:
    Path(path).write_text(json.dumps(data, indent=2, sort_keys=True) + "\n")


def write_comment(path: str | os.PathLike[str], marker: str, body: str) -> None:
    Path(path).write_text(f"{marker}\n{body.rstrip()}\n")


def common_parser(description: str) -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument("--base", required=True, help="Base git revision")
    parser.add_argument("--head", required=True, help="Head git revision")
    parser.add_argument("--comment-file", required=True, help="Markdown output path")
    parser.add_argument("--result-file", required=True, help="JSON output path")
    return parser


def path_line(path: str, line_number: int | None = None) -> str:
    if line_number is None:
        return f"`{path}`"
    return f"`{path}:{line_number}`"
