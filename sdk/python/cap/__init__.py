import sys
import types

try:
    from google.protobuf import runtime_version as _runtime_version  # noqa: F401
except Exception:
    try:
        import google.protobuf as _protobuf
    except Exception:
        _protobuf = None

    _shim = types.SimpleNamespace()

    class _Domain:
        PUBLIC = 0

    def _validate(*_args, **_kwargs):
        return None

    _shim.Domain = _Domain
    _shim.ValidateProtobufRuntimeVersion = _validate
    sys.modules["google.protobuf.runtime_version"] = _shim
    if _protobuf is not None:
        setattr(_protobuf, "runtime_version", _shim)

from .client import submit_job
from .worker import run_worker
from .bus import connect_nats

__all__ = ["submit_job", "run_worker", "connect_nats"]
