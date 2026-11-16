"""Test-only child for test_proc.py, never shipped. argv[1] selects the behaviour.

Every mode first reads the one job line from stdin, as the real child must, and leaves a file
in its temp dir so that cleanup has something to remove.
"""
import json
import os
import signal
import socket
import subprocess
import sys
import tempfile
import time

LINE_LIMIT = 1 << 20
SLEEPER = ("import signal, time\n"
           "if hasattr(signal, 'SIGTERM'): signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
           "time.sleep(120)\n")


def emit(event):
    sys.stdout.buffer.write(json.dumps(event).encode("ascii") + b"\n")
    sys.stdout.buffer.flush()


def main(mode):
    job = json.loads(sys.stdin.buffer.readline())
    tmp = tempfile.gettempdir()
    with open(os.path.join(tmp, "held.txt"), "w") as handle:
        handle.write("x")
    if mode == "echo":
        emit({"type": "progress", "marker": "one"})
        emit({"type": "result", "job": job, "env": dict(os.environ), "tmp": tmp,
              "cwd_is_tmp": os.path.samefile(os.getcwd(), tmp), "trap": hasattr(socket, "_cap_nettrap_blocked")})
    elif mode == "hang":  # the child and its grandchild both ignore SIGTERM
        if hasattr(signal, "SIGTERM"):
            signal.signal(signal.SIGTERM, signal.SIG_IGN)
        grandchild = subprocess.Popen([sys.executable, "-c", SLEEPER])
        emit({"type": "progress", "pid": grandchild.pid})
        time.sleep(120)
    elif mode == "crash":
        emit({"type": "progress", "marker": "before-crash"})
        sys.exit(3)
    elif mode == "long-line":  # a bad line, then a good result: only the bound can fail the job
        sys.stdout.buffer.write(b'{"type": "progress", "pad": "' + b"x" * LINE_LIMIT + b'"}\n')
        emit({"type": "result"})
    elif mode == "flood":
        for _ in range(1000):
            emit({"type": "progress", "marker": "m" * 64})
        emit({"type": "result"})
    elif mode == "unknown-type":
        sys.stdout.buffer.write(b'{"type": "unknown"}\n')
        emit({"type": "result"})
    elif mode == "not-json":
        sys.stdout.buffer.write(b"not json\n")
        emit({"type": "result"})
    elif mode == "stderr":
        sys.stderr.buffer.write(b"e" * (4 << 20))
        sys.stderr.buffer.flush()
        emit({"type": "result"})
    elif mode == "exit-after-result":
        emit({"type": "result"})
        sys.stdout.flush()
        os._exit(97)
    elif mode == "connect":  # under the trap the attempt is refused and the child exits 97 at exit
        try:
            socket.create_connection(("192.0.2.1", 9), timeout=1)
        except OSError:
            pass
        emit({"type": "result"})


if __name__ == "__main__":
    main(sys.argv[1])
