"""Selects how tools/proto_codegen.py invokes its generators.

CAP has one canonical generator (tools/proto_codegen.py) but two ways to reach
the plugins it drives:

* ``networked``  - buf resolves plugins from buf.build (//buf.gen.yaml) and npm
  supplies the Node bundler. This is what a developer or a normal CI job runs.
* ``hermetic``   - every generator is preinstalled in tools/codegen/Dockerfile
  and buf resolves them locally (tools/codegen/buf.gen.offline.yaml), so
  generation runs under ``docker run --network=none``.

buf emits identical bytes for a remote and a local plugin of the same version,
so these are two paths to one generator, not two generators. Keeping the
difference in this module means proto_codegen.py states *what* must be
generated and validated without branching on *how* the tools are reached.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence

REPO_ROOT = Path(__file__).resolve().parents[1]
BUF_TOOL = "github.com/bufbuild/buf/cmd/buf@v1.71.0"
ONLINE_TEMPLATE = REPO_ROOT / "buf.gen.yaml"
OFFLINE_TEMPLATE = REPO_ROOT / "tools" / "codegen" / "buf.gen.offline.yaml"
DEFAULT_NODE_MODULES = REPO_ROOT / "tools" / "codegen" / "node_modules"
# tools/codegen/Dockerfile bakes node_modules in at this path and exports it.
NODE_MODULES_ENV = "CAP_CODEGEN_NODE_MODULES"


class ToolchainError(RuntimeError):
    """The requested generator toolchain is not usable."""


@dataclass(frozen=True)
class Toolchain:
    """How to reach every generator, and which invariants that path can check."""

    name: str
    template: Path
    buf_command: tuple[str, ...]
    node_modules: Path
    # `buf breaking --against origin/main` needs remote git history, which a
    # hermetic container deliberately does not have. It is an online review
    # gate, not a reproducibility gate, so the hermetic path skips it and CI
    # runs it on the networked path instead.
    checks_breaking: bool
    # The hermetic image installs node_modules at build time from the same
    # lockfile; re-running `npm ci` there would need the network.
    installs_node_modules: bool

    def buf(self, *arguments: str) -> list[str]:
        return [*self.buf_command, *arguments]

    @property
    def protobufjs_bin(self) -> Path:
        return self.node_modules / "protobufjs-cli" / "bin"

    def require_ready(self) -> None:
        if not self.template.is_file():
            raise ToolchainError(f"missing Buf generation template: {self.template}")
        if self.installs_node_modules:
            return
        binary = self.protobufjs_bin / "pbjs"
        if not binary.is_file():
            raise ToolchainError(
                f"{self.name} toolchain expects preinstalled Node modules at {self.node_modules} "
                f"(missing {binary}); set {NODE_MODULES_ENV} or rebuild tools/codegen/Dockerfile"
            )


def networked() -> Toolchain:
    return Toolchain(
        name="networked",
        template=ONLINE_TEMPLATE,
        buf_command=("go", "run", BUF_TOOL),
        node_modules=DEFAULT_NODE_MODULES,
        checks_breaking=True,
        installs_node_modules=True,
    )


def hermetic(node_modules: Path | None = None) -> Toolchain:
    resolved = node_modules or Path(os.environ.get(NODE_MODULES_ENV, DEFAULT_NODE_MODULES))
    return Toolchain(
        name="hermetic",
        template=OFFLINE_TEMPLATE,
        buf_command=("buf",),
        node_modules=resolved,
        checks_breaking=False,
        installs_node_modules=False,
    )


def select(offline: bool) -> Toolchain:
    toolchain = hermetic() if offline else networked()
    toolchain.require_ready()
    return toolchain


def describe(toolchain: Toolchain) -> dict[str, object]:
    return {
        "toolchain": toolchain.name,
        "template": str(toolchain.template.relative_to(REPO_ROOT)),
        "buf": " ".join(toolchain.buf_command),
        "breakingChecked": toolchain.checks_breaking,
    }


def resolved_plugin_versions(template: Path) -> Sequence[str]:
    """Version strings a template pins, used to prove the two mirrors agree."""
    import re

    text = template.read_text(encoding="utf-8")
    return tuple(sorted(re.findall(r"buf\.build/[\w.-]+/([\w.-]+:v[\w.-]+)", text)))
