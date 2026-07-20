"""Pinned foreground Docker NATS server for required integration tests."""

import queue
import subprocess
import threading
import uuid


NATS_IMAGE = "nats@sha256:73b0ba5fec5518c5f698597c58d2a3350a2b5ccae43e84c308f8d2da1242deca"


class DockerNATSServer:
    def __init__(self, name: str, process: subprocess.Popen[str], url: str) -> None:
        self.name = name
        self.process = process
        self.url = url

    @classmethod
    def start(cls) -> "DockerNATSServer":
        cls._ensure_image()
        name = f"cap-python-nats-{uuid.uuid4().hex}"
        process = subprocess.Popen(
            [
                "docker", "run", "--rm", "--name", name,
                "-p", "127.0.0.1::4222", NATS_IMAGE,
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        try:
            cls._await_ready(process)
            mapping = subprocess.run(
                ["docker", "port", name, "4222/tcp"],
                check=True,
                capture_output=True,
                text=True,
                timeout=10,
            ).stdout.strip()
            return cls(name, process, f"nats://127.0.0.1:{mapping.rsplit(':', 1)[1]}")
        except BaseException:
            cls._remove(name, process)
            raise

    @staticmethod
    def _await_ready(process: subprocess.Popen[str]) -> None:
        output: queue.Queue[str | None] = queue.Queue()
        lines: list[str] = []

        def read_output() -> None:
            assert process.stdout is not None
            for line in process.stdout:
                output.put(line)
            output.put(None)

        threading.Thread(target=read_output, daemon=True).start()
        while True:
            try:
                line = output.get(timeout=30)
            except queue.Empty as exc:
                raise AssertionError("timed out waiting for real NATS readiness") from exc
            if line is None:
                raise AssertionError("NATS container exited: " + " | ".join(lines[-20:]))
            lines.append(line.rstrip())
            if "Server is ready" in line:
                return

    @staticmethod
    def _ensure_image() -> None:
        inspect = subprocess.run(
            ["docker", "image", "inspect", NATS_IMAGE],
            capture_output=True,
            timeout=10,
        )
        if inspect.returncode == 0:
            return
        pulled = subprocess.run(
            ["docker", "pull", NATS_IMAGE],
            capture_output=True,
            text=True,
            timeout=180,
        )
        if pulled.returncode != 0:
            raise AssertionError("pinned real NATS image is unavailable: " + pulled.stderr)

    @staticmethod
    def _remove(name: str, process: subprocess.Popen[str]) -> None:
        subprocess.run(
            ["docker", "rm", "-f", name],
            capture_output=True,
            text=True,
            timeout=10,
        )
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=10)

    def close(self) -> None:
        self._remove(self.name, self.process)
