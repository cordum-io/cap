"""Ensures sdk/python is importable as the ``tests`` package root regardless
of the working directory pytest is invoked from.

test_production_admission_nats.py does ``from tests.docker_nats_support
import DockerNATSServer``, which only resolves when sdk/python happens to be
on sys.path. That's true when a developer runs ``cd sdk/python && pytest
tests/``, but not when CI (publish-python.yml's mandatory real-NATS gate)
runs ``python -m pytest -q sdk/python/tests`` from the repository root.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
