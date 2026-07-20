"""Build installed CAP SDK artifacts and their cross-language matrix drivers.

Each stable SDK is consumed the way a third party consumes it:

* Go     - an external module resolving ``cap/v2`` through a local file proxy
           built from committed git bytes, with ``GOWORK=off``.
* Python - a wheel built from a copy of ``sdk/python`` and installed into a
           fresh venv, exercised with ``python -I`` so repo source cannot shadow.
* Node   - an ``npm pack`` tarball installed into an empty consumer package.

Writes ``<out>/drivers.json`` describing how to invoke each driver, so the Go
test harness never has to know per-language packaging details.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

MODULE_VERSION = "v2.99.0-tck.0"
DRIVERS = "drivers.json"


class BuildError(RuntimeError):
    """An expected, user-actionable artifact build failure."""


def run(argv: list[str], cwd: Path, env: dict[str, str] | None = None) -> None:
    merged = {**os.environ, **(env or {})}
    result = subprocess.run(argv, cwd=cwd, env=merged, capture_output=True, text=True)
    if result.returncode != 0:
        raise BuildError(
            f"{argv[0]} failed ({result.returncode}) in {cwd}\n"
            f"argv: {argv}\nstdout:\n{result.stdout[-4000:]}\nstderr:\n{result.stderr[-4000:]}"
        )


def proxy_url(path: Path) -> str:
    """Go requires file:///C:/... on Windows; a two-slash form is rejected."""
    resolved = path.resolve().as_posix()
    return f"file:///{resolved.lstrip('/')}"


def build_go(root: Path, out: Path) -> dict[str, str]:
    proxy, consumer = out / "goproxy", out / "godriver"
    run([sys.executable, str(root / "tools" / "onboarding" / "build_go_proxy.py"),
         "--root", str(root), "--output", str(proxy), "--version", MODULE_VERSION], cwd=root)
    consumer.mkdir(parents=True, exist_ok=True)
    for source in (root / "tools" / "tck" / "matrix" / "go").glob("*.go"):
        shutil.copy2(source, consumer / source.name)
    (consumer / "go.mod").write_bytes(
        b"module cordum.io/tck/matrixdriver\n\ngo 1.25\n"
    )
    # GOPRIVATE/GONOPROXY must stay empty: either one makes Go bypass the file
    # proxy and attempt a direct VCS fetch of the unpublished version.
    env = {
        "GOWORK": "off", "GOFLAGS": "-mod=mod", "GOSUMDB": "off",
        "GOPRIVATE": "", "GONOPROXY": "", "GONOSUMDB": "",
        "GOPROXY": proxy_url(proxy),
    }
    run(["go", "mod", "edit", f"-require=github.com/cordum-io/cap/v2@{MODULE_VERSION}"],
        cwd=consumer, env=env)
    # `go mod tidy` resolves the whole module graph, including the test
    # dependencies of our dependencies' packages (grpc's balancer tests pull
    # go.opentelemetry.io/otel and gonum). The driver never builds those, the
    # file proxy carries only the build closure, and GOPROXY has no fallback, so
    # a strict tidy fails on a cold module cache -- it passed locally only
    # because a warm GOMODCACHE happened to satisfy them. `-e` proceeds despite
    # those unresolvable test-only packages; `go build` immediately after is the
    # real gate, so a genuinely missing BUILD dependency still fails loudly.
    run(["go", "mod", "tidy", "-e"], cwd=consumer, env=env)
    binary = consumer / ("matrixdriver.exe" if os.name == "nt" else "matrixdriver")
    run(["go", "build", "-o", binary.name, "."], cwd=consumer, env=env)
    return {"argv": [str(binary)], "artifact": f"local module proxy {MODULE_VERSION}"}


def build_python(root: Path, out: Path) -> dict[str, str]:
    source, venv = out / "pysrc", out / "pyvenv"
    # sdk/python only gitignores docs/, so build from a copy to keep the
    # worktree free of build/, dist/ and egg-info pollution.
    shutil.copytree(root / "sdk" / "python", source,
                    ignore=shutil.ignore_patterns("build", "dist", "*.egg-info", "__pycache__"))
    run([sys.executable, "-m", "build", "--wheel"], cwd=source)
    wheels = sorted((source / "dist").glob("*.whl"))
    if len(wheels) != 1:
        raise BuildError(f"expected exactly one wheel, found {[w.name for w in wheels]}")
    run([sys.executable, "-m", "venv", str(venv)], cwd=out)
    python = venv / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    run([str(python), "-m", "pip", "install", "--disable-pip-version-check", "-q",
         str(wheels[0]), "cryptography"], cwd=out)
    driver = out / "python-driver.py"
    shutil.copy2(root / "tools" / "tck" / "matrix" / "python" / "driver.py", driver)
    return {"argv": [str(python), "-I", str(driver)], "artifact": wheels[0].name}


def build_node(root: Path, out: Path) -> dict[str, str]:
    sdk, consumer = root / "sdk" / "node", out / "nodecons"
    npm = npm_argv()
    if not (sdk / "node_modules").is_dir():
        run(npm + ["ci", "--no-audit", "--no-fund"], cwd=sdk)
    run(npm + ["run", "build"], cwd=sdk)
    run(npm + ["pack", "--pack-destination", str(out)], cwd=sdk)
    tarballs = sorted(out.glob("cap-sdk-node-*.tgz"))
    if len(tarballs) != 1:
        raise BuildError(f"expected exactly one tarball, found {[t.name for t in tarballs]}")
    consumer.mkdir(parents=True, exist_ok=True)
    # Set package.json directly: `npm init -y --prefix` can mutate the SDK's own
    # package.json under npm 11 instead of the prefixed directory.
    (consumer / "package.json").write_bytes(
        b'{"name":"cap-matrix-consumer","version":"1.0.0","private":true}\n'
    )
    run(npm + ["install", "--no-audit", "--no-fund", str(tarballs[0])], cwd=consumer)
    driver = consumer / "driver.cjs"
    shutil.copy2(root / "tools" / "tck" / "matrix" / "node" / "driver.cjs", driver)
    return {"argv": ["node", str(driver)], "artifact": tarballs[0].name}


def npm_argv() -> list[str]:
    """On Windows Node, spawning npm.cmd without a shell fails with EINVAL."""
    if os.name != "nt":
        return ["npm"]
    node = shutil.which("node") or "node"
    cli = Path(node).parent / "npm-cli.js"
    candidate = Path(node).parent / "node_modules" / "npm" / "bin" / "npm-cli.js"
    return [node, str(candidate if candidate.is_file() else cli)]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    root, out = args.root.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    try:
        drivers = {
            "go": build_go(root, out),
            "python": build_python(root, out),
            "node": build_node(root, out),
        }
    except BuildError as error:
        print(f"build_artifacts: {error}", file=sys.stderr)
        return 1
    (out / DRIVERS).write_bytes(
        (json.dumps(drivers, indent=2, sort_keys=True) + "\n").encode()
    )
    print(f"build_artifacts: wrote {out / DRIVERS}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
