"""Installed-artifact contract for the public worker-trust API."""

import importlib.util
import json
import subprocess
import sys
from pathlib import Path
from types import ModuleType
from typing import Tuple
from unittest.mock import patch

import pytest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "verify_artifacts.py"


def _load_verifier() -> ModuleType:
    spec = importlib.util.spec_from_file_location("cap_artifact_trust_verifier", SCRIPT)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


verifier = _load_verifier()


@pytest.mark.parametrize(
    ("worker_trust", "message"),
    (
        (None, "worker trust smoke is missing"),
        ({"challenge_bytes": 321, "mode": "off", "protocol_version": 1,
          "sender_id": "artifact-worker"}, "mode mismatch"),
        ({"challenge_bytes": 321, "mode": "enforce", "protocol_version": 2,
          "sender_id": "artifact-worker"}, "protocol_version mismatch"),
        ({"challenge_bytes": 321, "mode": "enforce", "protocol_version": 1,
          "sender_id": "other-worker"}, "sender_id mismatch"),
        ({"challenge_bytes": True, "mode": "enforce", "protocol_version": 1,
          "sender_id": "artifact-worker"}, "challenge size is invalid"),
    ),
)
def test_consumer_rejects_invalid_worker_trust_smoke(
    tmp_path: Path, worker_trust: object, message: str
) -> None:
    def fake_run(args: Tuple[str, ...], cwd: Path, timeout: int) -> object:
        del cwd, timeout
        evidence = {
            "imports": len(verifier.EXPECTED_IMPORTS),
            "protobuf_version": "6.31.1",
            "grpcio_version": "1.76.0",
            "worker_trust": worker_trust,
        }
        stdout = json.dumps(evidence) if str(verifier.SMOKE_SCRIPT) in args else ""
        return subprocess.CompletedProcess(args, 0, stdout=stdout, stderr="")

    with patch.object(verifier, "_run", side_effect=fake_run), patch.object(
        verifier, "_venv_python", return_value=Path(sys.executable)
    ), pytest.raises(verifier.VerificationError, match=message):
        verifier._consumer_check(tmp_path / "artifact.whl", Path(sys.executable), 10)
