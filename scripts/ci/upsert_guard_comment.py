#!/usr/bin/env python3
"""Create or update a PR comment identified by an HTML marker."""

from __future__ import annotations

import argparse
import json
import os
import urllib.error
import urllib.request


def request(method: str, url: str, token: str, body: dict[str, object] | None = None) -> dict[str, object] | list[dict[str, object]]:
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(
        url,
        data=data,
        method=method,
        headers={
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode() or "{}")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode()
        raise SystemExit(f"GitHub API request failed: {exc.code} {detail}") from exc


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True, help="owner/repo")
    parser.add_argument("--issue-number", required=True, help="Pull request number")
    parser.add_argument("--comment-file", required=True, help="Comment body file")
    parser.add_argument("--marker", required=True, help="Marker used to identify an existing comment")
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        raise SystemExit("GITHUB_TOKEN is required")

    with open(args.comment_file, encoding="utf-8") as handle:
        body = handle.read()

    base_url = f"https://api.github.com/repos/{args.repo}/issues/{args.issue_number}/comments"
    comments = request("GET", f"{base_url}?per_page=100", token)
    if not isinstance(comments, list):
        raise SystemExit("Unexpected comments response from GitHub API")

    for comment in comments:
        if args.marker in str(comment.get("body", "")):
            request("PATCH", str(comment["url"]), token, {"body": body})
            return 0

    request("POST", base_url, token, {"body": body})
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
