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
from .types import Decision, OnErrorCallback, OnErrorMode, SafetyDecision

__all__ = [
    "CordumClient",
    "CordumAuthError",
    "CordumBlockedError",
    "CordumConnectionError",
    "CordumError",
    "CordumTimeoutError",
    "Decision",
    "OnErrorCallback",
    "OnErrorMode",
    "SafetyDecision",
    "guard",
]
