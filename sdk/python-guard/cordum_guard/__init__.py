"""Cordum Guard — Safety governance for Python AI agents."""

from importlib.metadata import PackageNotFoundError
from importlib.metadata import version as _dist_version

from .client import CordumClient
from .exceptions import (
    CordumApprovalRequiredError,
    CordumAuthError,
    CordumBlockedError,
    CordumConnectionError,
    CordumError,
    CordumTimeoutError,
)
from .guard import guard
from .mock import MockCordumClient
from .types import Decision, OnErrorCallback, OnErrorMode, SafetyDecision

try:
    __version__ = _dist_version("cordum-guard")
except PackageNotFoundError:  # imported from a source tree that was never installed
    __version__ = "0+unknown"

__all__ = [
    "CordumClient",
    "CordumApprovalRequiredError",
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
