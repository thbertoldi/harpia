"""Typed exception hierarchy for LLM providers."""

from __future__ import annotations


class LLMError(RuntimeError):
    """Base class for all LLM abstraction errors."""


class AuthenticationError(LLMError):
    """Provider credentials are missing or invalid."""


class RateLimitError(LLMError):
    """Provider rate limit was exceeded."""


class ProviderUnavailableError(LLMError):
    """Provider is unavailable or returned transient server errors."""


class ModelNotFoundError(LLMError):
    """Requested model is unknown to the registry/provider."""


class BadRequestError(LLMError):
    """Provider rejected the request due to invalid payload."""
