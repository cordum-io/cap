"""Static fail-closed contracts for the Python release workflow."""

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "publish-python.yml"


def _workflow() -> str:
    return WORKFLOW_PATH.read_text(encoding="utf-8")


def _job(name: str) -> str:
    workflow = _workflow()
    match = re.search(
        rf"(?ms)^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [a-zA-Z0-9_-]+:\n|\Z)",
        workflow,
    )
    assert match is not None, f"missing release job: {name}"
    return match.group(0)


def test_release_triggers_require_checked_in_tag_version() -> None:
    workflow = _workflow()
    assert "workflow_dispatch:" in workflow
    assert "release:" in workflow and "types: [published]" in workflow
    assert '"${GITHUB_REF_TYPE}" != "tag"' in workflow
    assert '"${GITHUB_REF}" != refs/tags/*' in workflow
    assert 'validate_release.py --tag "${RELEASE_TAG}"' in workflow
    for mutation in (
        "Set version from tag",
        "Using fallback PEP 440 version",
        "write_text(",
        "lstrip(\"v\")",
        "sed -i",
    ):
        assert mutation not in workflow


def test_release_has_bounded_single_flight_and_oidc_permissions() -> None:
    workflow = _workflow()
    build = _job("build-and-verify")
    publish = _job("publish")
    assert "concurrency:" in workflow
    assert "group: publish-python-${{ github.ref }}" in workflow
    assert "cancel-in-progress: false" in workflow
    assert "timeout-minutes:" in build and "timeout-minutes:" in publish
    assert "id-token: write" not in build
    assert "id-token: write" in publish and "needs: build-and-verify" in publish
    assert "contents: read" in build and "contents: read" not in publish
    assert "actions/checkout@" in build and "persist-credentials: false" in build
    assert "ref: ${{ github.ref }}" in build
    assert "actions/checkout@" not in publish
    for forbidden in ("setup-python", "pip install", "pytest", "mypy", "twine", "python -m build"):
        assert forbidden not in publish


def test_release_uses_exact_codegen_and_mandatory_nats() -> None:
    job = _job("build-and-verify")
    assert "image: nats:2.10.29-alpine" in job
    assert "CAP_TEST_NATS_URL: nats://127.0.0.1:4222" in job
    assert 'python -m pip install "sdk/python[dev]"' in job
    assert "python -m pip install -r sdk/python/requirements-codegen.txt" in job
    assert "python sdk/python/scripts/generate_protos.py --check" in job
    assert "--ignore=sdk/python/tests/integration" not in job


def test_release_runs_source_dependency_and_strong_typing_gates() -> None:
    job = _job("build-and-verify")
    assert "python -m pytest -q sdk/python/tests" in job
    ignores = re.findall(r"--ignore=([^\s\\]+)", job)
    assert ignores == ["sdk/python/tests/test_artifacts.py"]
    assert "python -m pip check" in job
    assert "python -m mypy --strict" in job
    assert "sdk/python/tests/typing/consumer.py" in job
    for flag in (
        "--follow-imports=silent",
        "--disallow-any-expr",
        "--disallow-any-explicit",
        "--disallow-any-decorated",
        "--disallow-any-unimported",
        "--no-incremental",
        "--python-version=3.9",
    ):
        assert flag in job


def test_release_builds_one_exact_wheel_sdist_pair() -> None:
    job = _job("build-and-verify")
    assert "rm -rf dist" in job
    assert job.count("python -m build") == 1
    assert 'RELEASE_ROOT="${RUNNER_TEMP}/cap-python-source"' in job
    assert 'RELEASE_SOURCE="$RELEASE_ROOT/sdk/python"' in job
    assert 'test "$(/usr/bin/git rev-parse HEAD)" = "$GITHUB_SHA"' in job
    archive_command = (
        '/usr/bin/git --no-replace-objects archive --format=tar --prefix=sdk/python/ '
        '"${GITHUB_SHA}:sdk/python"'
    )
    assert archive_command in job
    assert 'python -m build --outdir dist/release "$RELEASE_SOURCE"' in job
    assert "--outdir dist/release sdk/python" not in job
    assert "shopt -s nullglob" in job
    assert "${#wheels[@]} != 1" in job
    assert "${#sdists[@]} != 1" in job
    assert 'python -m twine check "$WHEEL" "$SDIST"' in job


def test_release_records_verifier_checksums_and_inventory_evidence() -> None:
    job = _job("build-and-verify")
    assert 'python "$RELEASE_SOURCE/scripts/verify_artifacts.py"' in job
    assert "python sdk/python/scripts/verify_artifacts.py" not in job
    assert '--wheel "$WHEEL"' in job and '--sdist "$SDIST"' in job
    assert "tee dist/evidence/artifact-verification.json" in job
    assert "sha256sum" in job and "SHA256SUMS" in job
    assert "uses: actions/upload-artifact@" in job
    assert "dist/evidence/artifact-verification.json" in job
    assert "dist/evidence/SHA256SUMS" in job
    assert "if-no-files-found: error" in job
    assert 'echo "wheel_sha256=$wheel_sha256" >> "$GITHUB_OUTPUT"' in job
    assert 'echo "sdist_sha256=$sdist_sha256" >> "$GITHUB_OUTPUT"' in job
    report_validation = (
        '"$RELEASE_SOURCE/scripts/validate_release.py" --tag "$RELEASE_TAG" '
        '--artifact-report dist/evidence/artifact-verification.json'
    )
    assert report_validation in job
    assert job.index("verify_artifacts.py") < job.index(report_validation)
    assert job.index(report_validation) < job.index("id: release_hashes")
    assert job.index(report_validation) < job.index("actions/upload-artifact@")


def test_release_rejects_branch_before_tools_and_exports_after_gates() -> None:
    job = _job("build-and-verify")
    tag_guard = job.index('"${GITHUB_REF_TYPE}" != "tag"')
    checkout = job.index("actions/checkout@")
    install = job.index("pip install")
    archive = job.index("/usr/bin/git --no-replace-objects archive --format=tar")
    build = job.index("python -m build")
    assert tag_guard < checkout < install
    assert job.index("generate_protos.py --check") < archive < build
    assert job.index("python -m pytest") < archive
    assert "pip install --upgrade pip" not in job


def test_release_rechecks_bytes_then_publishes_only_verified_directory() -> None:
    job = _job("publish")
    assert "actions/download-artifact@" in job
    recheck = job.index("sha256sum --check")
    publish = job.index("pypa/gh-action-pypi-publish@")
    assert recheck < publish
    between = job[recheck:publish]
    assert "python -m build" not in between
    assert "cp " not in between and "mv " not in between
    assert "${#wheels[@]} != 1" in job and "${#sdists[@]} != 1" in job
    assert "EXPECTED_WHEEL_SHA256" in job and "EXPECTED_SDIST_SHA256" in job
    assert ".artifacts.wheel.sha256" in job and ".artifacts.sdist.sha256" in job
    assert "packages-dir: dist/release" in job
    assert "skip-existing" not in job


def test_release_actions_are_commit_pinned() -> None:
    workflow = _workflow()
    actions = re.findall(r"uses: ([^\s#]+)", workflow)
    assert actions
    for action in actions:
        owner, revision = action.rsplit("@", 1)
        assert "/" in owner
        assert re.fullmatch(r"[0-9a-f]{40}", revision), action
    assert "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683" in actions
    assert "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065" in actions
    assert "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02" in actions
    assert "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093" in actions
