"""Tests for redis_ssl_context_from_env() TLS configuration."""

import datetime
import importlib.util
import os
import ssl
import sys

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID


def _make_self_signed_pem() -> str:
    """Generate a valid self-signed CA certificate PEM for testing."""
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    now = datetime.datetime.now(datetime.timezone.utc)
    cert = (
        x509.CertificateBuilder()
        .subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "testca")]))
        .issuer_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "testca")]))
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now)
        .not_valid_after(now + datetime.timedelta(days=365))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .sign(key, hashes.SHA256())
    )
    return cert.public_bytes(serialization.Encoding.PEM).decode()


def _load_redis_ssl_context_from_env():
    """Import redis_ssl_context_from_env from runtime.py directly.

    Uses importlib to load the module file directly, bypassing the cap
    package __init__.py which triggers protobuf imports that may fail
    due to version mismatches in dev environments.
    """
    runtime_path = os.path.join(
        os.path.dirname(__file__), "..", "cap", "runtime.py",
    )
    spec = importlib.util.spec_from_file_location(
        "_cap_runtime_isolated", runtime_path,
        submodule_search_locations=[],
    )
    mod = importlib.util.module_from_spec(spec)

    # Stub out protobuf imports that runtime.py needs but may fail
    # due to version mismatches in dev environments.
    import types
    from unittest.mock import MagicMock

    # Discover all cap.* modules to stub
    cap_dir = os.path.join(os.path.dirname(__file__), "..", "cap")
    stub_mods = ["cap"]
    for entry in os.listdir(cap_dir):
        if entry.endswith(".py") and entry != "__init__.py":
            stub_mods.append(f"cap.{entry[:-3]}")
        elif os.path.isdir(os.path.join(cap_dir, entry)) and not entry.startswith("_"):
            stub_mods.append(f"cap.{entry}")
    # Add deep protobuf paths
    stub_mods.extend([
        "cap.pb.cordum", "cap.pb.cordum.agent",
        "cap.pb.cordum.agent.v1", "cap.pb.cordum.agent.v1.buspacket_pb2",
        "cap.pb.cordum.agent.v1.job_pb2", "cap.pb.cordum.agent.v1.heartbeat_pb2",
    ])

    saved = {}
    for name in stub_mods:
        saved[name] = sys.modules.get(name)
        if name not in sys.modules:
            sys.modules[name] = MagicMock()

    try:
        spec.loader.exec_module(mod)
    finally:
        for name, orig in saved.items():
            if orig is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = orig

    return mod.redis_ssl_context_from_env


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch):
    """Remove all REDIS_TLS_* and SSL_CERT_FILE env vars before each test."""
    for var in ("REDIS_TLS_CA", "REDIS_TLS_CERT", "REDIS_TLS_KEY",
                "REDIS_TLS_SERVER_NAME", "REDIS_TLS_INSECURE", "SSL_CERT_FILE"):
        monkeypatch.delenv(var, raising=False)


@pytest.fixture
def fn():
    """Provide the redis_ssl_context_from_env function."""
    return _load_redis_ssl_context_from_env()


def test_returns_none_when_no_env_vars(fn):
    """No TLS env vars set -> returns None."""
    assert fn() is None


def test_creates_context_with_ca(fn, monkeypatch, tmp_path):
    """REDIS_TLS_CA set -> returns SSLContext with CA loaded."""
    ca_file = tmp_path / "ca.pem"
    ca_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("REDIS_TLS_CA", str(ca_file))

    ctx = fn()
    assert ctx is not None
    assert isinstance(ctx, ssl.SSLContext)


def test_raises_when_cert_without_key(fn, monkeypatch, tmp_path):
    """REDIS_TLS_CERT without REDIS_TLS_KEY -> ValueError."""
    cert_file = tmp_path / "cert.pem"
    cert_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("REDIS_TLS_CERT", str(cert_file))

    with pytest.raises(ValueError, match="must be set together"):
        fn()


def test_insecure_skips_verification(fn, monkeypatch):
    """REDIS_TLS_INSECURE=true -> check_hostname=False, CERT_NONE."""
    monkeypatch.setenv("REDIS_TLS_INSECURE", "true")

    ctx = fn()
    assert ctx is not None
    assert ctx.check_hostname is False
    assert ctx.verify_mode == ssl.CERT_NONE


def test_falls_back_to_ssl_cert_file(fn, monkeypatch, tmp_path):
    """SSL_CERT_FILE used when REDIS_TLS_CA not set."""
    ca_file = tmp_path / "system-ca.pem"
    ca_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("SSL_CERT_FILE", str(ca_file))

    ctx = fn()
    assert ctx is not None
    assert isinstance(ctx, ssl.SSLContext)


def test_raises_when_ca_file_missing(fn, monkeypatch):
    """REDIS_TLS_CA points to non-existent file -> FileNotFoundError."""
    monkeypatch.setenv("REDIS_TLS_CA", "/nonexistent/ca.pem")

    with pytest.raises(FileNotFoundError, match="REDIS_TLS_CA"):
        fn()
