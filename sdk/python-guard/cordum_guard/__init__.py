"""Cordum Guard — Safety governance for Python AI agents."""

__version__ = "0.1.0"

from .client import CordumClient
from .exceptions import (
    CordumAuthError,
    CordumBlockedError,
    CordumConnectionError,
    CordumError,
    CordumTimeoutError,
)
from .guard import guard
from .mock import MockCordumClient
from .types import Decision, SafetyDecision

__all__ = [
    "CordumClient",
    "CordumAuthError",
    "CordumBlockedError",
    "CordumConnectionError",
    "CordumError",
    "CordumTimeoutError",
    "Decision",
    "MockCordumClient",
    "SafetyDecision",
    "guard",
]
