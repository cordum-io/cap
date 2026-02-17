"""Cordum Guard — Safety governance for Python AI agents."""

__version__ = "0.1.1"

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
from .types import Decision, OnErrorCallback, OnErrorMode, SafetyDecision

__all__ = [
    "CordumClient",
    "CordumAuthError",
    "CordumBlockedError",
    "CordumConnectionError",
    "CordumError",
    "CordumTimeoutError",
    "Decision",
    "MockCordumClient",
    "OnErrorCallback",
    "OnErrorMode",
    "SafetyDecision",
    "guard",
]
