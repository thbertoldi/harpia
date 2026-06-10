"""Namespace shim for Buf-generated Harpia protobuf modules."""

from pathlib import Path

_CURRENT_ROOT = Path(__file__).resolve().parent
_GENERATED_ROOT = _CURRENT_ROOT.parent / "harpia_agents" / "gen" / "harpia"

__path__ = [str(_CURRENT_ROOT), str(_GENERATED_ROOT)]
