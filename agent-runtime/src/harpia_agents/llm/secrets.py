"""Redacted secret wrapper for runtime credentials."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class RedactedSecret:
    """String-backed secret that masks itself in logs/tracebacks."""

    _value: str

    def reveal(self) -> str:
        return self._value

    def is_empty(self) -> bool:
        return self._value == ""

    def __str__(self) -> str:
        return "***REDACTED***"

    def __repr__(self) -> str:
        return "RedactedSecret(***)"
