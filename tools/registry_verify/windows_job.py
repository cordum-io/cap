"""Windows Job Object containment for verifier subprocess trees."""

from __future__ import annotations

import os
from typing import Optional

if os.name == "nt":
    import ctypes
    from ctypes import wintypes

    class _BasicLimitInformation(ctypes.Structure):
        _fields_ = [
            ("PerProcessUserTimeLimit", ctypes.c_longlong),
            ("PerJobUserTimeLimit", ctypes.c_longlong),
            ("LimitFlags", wintypes.DWORD),
            ("MinimumWorkingSetSize", ctypes.c_size_t),
            ("MaximumWorkingSetSize", ctypes.c_size_t),
            ("ActiveProcessLimit", wintypes.DWORD),
            ("Affinity", ctypes.c_size_t),
            ("PriorityClass", wintypes.DWORD),
            ("SchedulingClass", wintypes.DWORD),
        ]

    class _IoCounters(ctypes.Structure):
        _fields_ = [
            ("ReadOperationCount", ctypes.c_ulonglong),
            ("WriteOperationCount", ctypes.c_ulonglong),
            ("OtherOperationCount", ctypes.c_ulonglong),
            ("ReadTransferCount", ctypes.c_ulonglong),
            ("WriteTransferCount", ctypes.c_ulonglong),
            ("OtherTransferCount", ctypes.c_ulonglong),
        ]

    class _ExtendedLimitInformation(ctypes.Structure):
        _fields_ = [
            ("BasicLimitInformation", _BasicLimitInformation),
            ("IoInfo", _IoCounters),
            ("ProcessMemoryLimit", ctypes.c_size_t),
            ("JobMemoryLimit", ctypes.c_size_t),
            ("PeakProcessMemoryUsed", ctypes.c_size_t),
            ("PeakJobMemoryUsed", ctypes.c_size_t),
        ]

    _KERNEL32 = ctypes.WinDLL("kernel32", use_last_error=True)
    _CREATE_JOB = _KERNEL32.CreateJobObjectW
    _CREATE_JOB.argtypes = (ctypes.c_void_p, wintypes.LPCWSTR)
    _CREATE_JOB.restype = wintypes.HANDLE
    _SET_JOB = _KERNEL32.SetInformationJobObject
    _SET_JOB.argtypes = (wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD)
    _SET_JOB.restype = wintypes.BOOL
    _OPEN_PROCESS = _KERNEL32.OpenProcess
    _OPEN_PROCESS.argtypes = (wintypes.DWORD, wintypes.BOOL, wintypes.DWORD)
    _OPEN_PROCESS.restype = wintypes.HANDLE
    _ASSIGN_PROCESS = _KERNEL32.AssignProcessToJobObject
    _ASSIGN_PROCESS.argtypes = (wintypes.HANDLE, wintypes.HANDLE)
    _ASSIGN_PROCESS.restype = wintypes.BOOL
    _TERMINATE_JOB = _KERNEL32.TerminateJobObject
    _TERMINATE_JOB.argtypes = (wintypes.HANDLE, wintypes.UINT)
    _TERMINATE_JOB.restype = wintypes.BOOL
    _CLOSE_HANDLE = _KERNEL32.CloseHandle
    _CLOSE_HANDLE.argtypes = (wintypes.HANDLE,)
    _CLOSE_HANDLE.restype = wintypes.BOOL

    _JOB_OBJECT_EXTENDED_LIMIT_INFORMATION = 9
    _JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000
    _PROCESS_TERMINATE = 0x0001
    _PROCESS_SET_QUOTA = 0x0100


def _last_error(operation: str) -> OSError:
    import ctypes

    code = ctypes.get_last_error()
    return OSError(code, f"{operation} failed", None, code)


def assign_kill_on_close(pid: int) -> Optional[int]:
    if os.name != "nt":
        return None
    job = _CREATE_JOB(None, None)
    if not job:
        raise _last_error("CreateJobObjectW")
    handle = int(job)
    try:
        limits = _ExtendedLimitInformation()
        limits.BasicLimitInformation.LimitFlags = _JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
        if not _SET_JOB(job, _JOB_OBJECT_EXTENDED_LIMIT_INFORMATION, ctypes.byref(limits), ctypes.sizeof(limits)):
            raise _last_error("SetInformationJobObject")
        process = _OPEN_PROCESS(_PROCESS_TERMINATE | _PROCESS_SET_QUOTA, False, pid)
        if not process:
            raise _last_error("OpenProcess")
        try:
            if not _ASSIGN_PROCESS(job, process):
                raise _last_error("AssignProcessToJobObject")
        finally:
            _CLOSE_HANDLE(process)
        return handle
    except OSError:
        _CLOSE_HANDLE(job)
        raise


def terminate(handle: Optional[int]) -> None:
    if os.name == "nt" and handle is not None and not _TERMINATE_JOB(handle, 1):
        raise _last_error("TerminateJobObject")


def close(handle: Optional[int]) -> None:
    if os.name == "nt" and handle is not None and not _CLOSE_HANDLE(handle):
        raise _last_error("CloseHandle(job)")
