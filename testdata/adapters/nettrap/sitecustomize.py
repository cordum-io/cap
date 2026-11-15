"""Test-only network trap for the CAP framework adapters. Never shipped.

Loaded through PYTHONPATH (or by path in the wheel smoke). Loopback, localhost,
AF_UNIX and passive getaddrinfo pass; every other connect or lookup is recorded
in socket._cap_nettrap_blocked and refused with OSError, and the process exits
97 at exit when anything was blocked. Python 3.10 compatible (task-c82d5007).
"""
import atexit
import ipaddress
import os
import socket
import sys

_blocked = []
socket._cap_nettrap_blocked = _blocked
_connect = socket.socket.connect
_connect_ex = socket.socket.connect_ex
_create_connection = socket.create_connection
_getaddrinfo = socket.getaddrinfo


def _loopback(host):
    if isinstance(host, bytes):
        host = host.decode("ascii", "replace")
    if not isinstance(host, str):
        return False
    if host.lower() == "localhost":
        return True
    try:
        ip = ipaddress.ip_address(host.split("%", 1)[0])
    except ValueError:
        return False
    mapped = getattr(ip, "ipv4_mapped", None)
    return ip.is_loopback or (mapped is not None and mapped.is_loopback)


def _refuse(call, host, port):
    _blocked.append((call, str(host), port))
    raise OSError("nettrap: non-loopback %s blocked" % call)


def _check_address(call, sock, address):
    if sock is not None and getattr(socket, "AF_UNIX", None) == sock.family:
        return
    host, port = (address[0], address[1]) if isinstance(address, tuple) else (address, None)
    if not _loopback(host):
        _refuse(call, host, port)


def connect(self, address):
    _check_address("connect", self, address)
    return _connect(self, address)


def connect_ex(self, address):
    _check_address("connect_ex", self, address)
    return _connect_ex(self, address)


def create_connection(address, *args, **kwargs):
    _check_address("create_connection", None, address)
    return _create_connection(address, *args, **kwargs)


def getaddrinfo(host, port, *args, **kwargs):
    if host is not None and not _loopback(host):
        _refuse("getaddrinfo", host, port)
    return _getaddrinfo(host, port, *args, **kwargs)


def _report():
    if _blocked:
        sys.stderr.write("NETTRAP: %d blocked attempt(s)\n" % len(_blocked))
        for record in _blocked:
            sys.stderr.write("NETTRAP: %r\n" % (record,))
        sys.stderr.flush()
        os._exit(97)


socket.socket.connect = connect
socket.socket.connect_ex = connect_ex
socket.create_connection = create_connection
socket.getaddrinfo = getaddrinfo
atexit.register(_report)
