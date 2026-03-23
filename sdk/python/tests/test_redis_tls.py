"""Tests for redis_ssl_context_from_env() TLS configuration."""

import datetime
import os
import ssl
import tempfile

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


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch):
    """Remove all REDIS_TLS_* and SSL_CERT_FILE env vars before each test."""
    for var in ("REDIS_TLS_CA", "REDIS_TLS_CERT", "REDIS_TLS_KEY",
                "REDIS_TLS_SERVER_NAME", "REDIS_TLS_INSECURE", "SSL_CERT_FILE"):
        monkeypatch.delenv(var, raising=False)


def _import_fn():
    """Import redis_ssl_context_from_env avoiding protobuf issues.

    Extracts just the function source from runtime.py and compiles it
    in an isolated module to avoid triggering protobuf imports.
    """
    import re
    import types

    source_path = os.path.join(os.path.dirname(__file__), "..", "cap", "runtime.py")
    with open(source_path) as f:
        source = f.read()

    # Extract the function body between its def and the next class/def at module level
    match = re.search(
        r"(def redis_ssl_context_from_env\(.*?\n(?:(?:    .*|[ \t]*)\n)*)",
        source,
    )
    if not match:
        raise RuntimeError("could not find redis_ssl_context_from_env in runtime.py")

    fn_source = "import os, ssl, logging\nfrom typing import Optional\n\n" + match.group(0)
    mod = types.ModuleType("_tls_test")
    exec(compile(fn_source, "redis_tls_extract", "exec"), mod.__dict__)
    return mod.redis_ssl_context_from_env


def test_returns_none_when_no_env_vars():
    """No TLS env vars set -> returns None."""
    fn = _import_fn()
    assert fn() is None


def test_creates_context_with_ca(monkeypatch, tmp_path):
    """REDIS_TLS_CA set -> returns SSLContext with CA loaded."""
    ca_file = tmp_path / "ca.pem"
    ca_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("REDIS_TLS_CA", str(ca_file))

    fn = _import_fn()
    ctx = fn()
    assert ctx is not None
    assert isinstance(ctx, ssl.SSLContext)


def test_raises_when_cert_without_key(monkeypatch, tmp_path):
    """REDIS_TLS_CERT without REDIS_TLS_KEY -> ValueError."""
    cert_file = tmp_path / "cert.pem"
    cert_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("REDIS_TLS_CERT", str(cert_file))

    fn = _import_fn()
    with pytest.raises(ValueError, match="must be set together"):
        fn()


def test_insecure_skips_verification(monkeypatch):
    """REDIS_TLS_INSECURE=true -> check_hostname=False, CERT_NONE."""
    monkeypatch.setenv("REDIS_TLS_INSECURE", "true")

    fn = _import_fn()
    ctx = fn()
    assert ctx is not None
    assert ctx.check_hostname is False
    assert ctx.verify_mode == ssl.CERT_NONE


def test_falls_back_to_ssl_cert_file(monkeypatch, tmp_path):
    """SSL_CERT_FILE used when REDIS_TLS_CA not set."""
    ca_file = tmp_path / "system-ca.pem"
    ca_file.write_text(_make_self_signed_pem())
    monkeypatch.setenv("SSL_CERT_FILE", str(ca_file))

    fn = _import_fn()
    ctx = fn()
    assert ctx is not None
    assert isinstance(ctx, ssl.SSLContext)


def test_raises_when_ca_file_missing(monkeypatch):
    """REDIS_TLS_CA points to non-existent file -> FileNotFoundError."""
    monkeypatch.setenv("REDIS_TLS_CA", "/nonexistent/ca.pem")

    fn = _import_fn()
    with pytest.raises(FileNotFoundError, match="REDIS_TLS_CA"):
        fn()
