"""Static fail-closed contracts for the Python release workflow."""

from pathlib import Path
import re

from workflow_support import (
    action_input_value,
    action_inputs,
    action_step_index,
    id_step_index,
    id_step_text,
    job_value,
    require_action_input,
    run_step_index,
    run_step_text,
    run_text,
    uses_values,
    workflow_mapping,
)


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "publish-python.yml"
NATS_IMAGE = (
    "nats:2.12.6-alpine@sha256:"
    "1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652"
)


def _workflow() -> str:
    return WORKFLOW_PATH.read_text(encoding="utf-8")


def test_release_triggers_require_checked_in_tag_version() -> None:
    workflow = _workflow()
    document = workflow_mapping(workflow)
    triggers = document.get("on")
    assert isinstance(triggers, dict)
    release = triggers.get("release")
    assert isinstance(release, dict)
    commands = run_text(workflow, "build-and-verify")
    guard = run_step_text(
        workflow, "build-and-verify", 'if [[ "${GITHUB_REF_TYPE}" != "tag"'
    )
    validation = run_step_text(
        workflow, "build-and-verify",
        "python sdk/python/scripts/validate_release.py --tag",
    )
    assert "workflow_dispatch" in triggers
    assert release.get("types") == ["published"]
    assert '"${GITHUB_REF_TYPE}" != "tag"' in guard
    assert '"${GITHUB_REF}" != refs/tags/*' in guard
    assert 'validate_release.py --tag "${RELEASE_TAG}"' in validation
    for mutation in (
        "Set version from tag",
        "Using fallback PEP 440 version",
        "write_text(",
        "lstrip(\"v\")",
        "sed -i",
    ):
        assert mutation not in commands


def test_release_has_bounded_single_flight_and_oidc_permissions() -> None:
    workflow = _workflow()
    concurrency = workflow_mapping(workflow).get("concurrency")
    assert isinstance(concurrency, dict)
    build_permissions = job_value(workflow, "build-and-verify", "permissions")
    publish_permissions = job_value(workflow, "publish", "permissions")
    assert isinstance(build_permissions, dict)
    assert isinstance(publish_permissions, dict)
    build_actions = uses_values(workflow, "build-and-verify")
    publish_actions = uses_values(workflow, "publish")
    assert concurrency.get("group") == "publish-python-${{ github.ref }}"
    assert concurrency.get("cancel-in-progress") == "false"
    assert job_value(workflow, "build-and-verify", "timeout-minutes") == "30"
    assert job_value(workflow, "publish", "timeout-minutes") == "10"
    assert build_permissions == {"contents": "read"}
    assert publish_permissions == {"id-token": "write"}
    assert job_value(workflow, "publish", "needs") == "build-and-verify"
    assert any(action.startswith("actions/checkout@") for action in build_actions)
    require_action_input(
        workflow, "build-and-verify", "actions/checkout@", "persist-credentials", "false"
    )
    require_action_input(
        workflow, "build-and-verify", "actions/checkout@", "ref", "${{ github.ref }}"
    )
    assert not any(action.startswith("actions/checkout@") for action in publish_actions)
    assert not any(action.startswith("actions/setup-python@") for action in publish_actions)
    publish_commands = run_text(workflow, "publish")
    for forbidden in ("pip install", "pytest", "mypy", "twine", "python -m build"):
        assert forbidden not in publish_commands


def test_release_uses_exact_codegen_and_mandatory_nats() -> None:
    workflow = _workflow()
    assert job_value(
        workflow, "build-and-verify", "services", "nats", "image"
    ) == NATS_IMAGE
    assert job_value(
        workflow, "build-and-verify", "env", "CAP_TEST_NATS_URL"
    ) == "nats://127.0.0.1:4222"
    install = run_step_text(
        workflow, "build-and-verify", 'python -m pip install "sdk/python[dev]"'
    )
    assert "python -m pip install -r sdk/python/requirements-codegen.txt" in install
    run_step_index(
        workflow, "build-and-verify",
        "python sdk/python/scripts/generate_protos.py --check",
    )


def test_release_runs_source_dependency_and_strong_typing_gates() -> None:
    workflow = _workflow()
    tests = run_step_text(
        workflow, "build-and-verify", "python -m pytest -q sdk/python/tests"
    )
    typing = run_step_text(
        workflow, "build-and-verify", "python -m mypy --strict"
    )
    ignores = re.findall(r"--ignore=([^\s\\]+)", tests)
    assert ignores == ["sdk/python/tests/test_artifacts.py"]
    run_step_index(workflow, "build-and-verify", "python -m pip check")
    assert "sdk/python/tests/typing/consumer.py" in typing
    for flag in (
        "--follow-imports=silent",
        "--disallow-any-expr",
        "--disallow-any-explicit",
        "--disallow-any-decorated",
        "--disallow-any-unimported",
        "--no-incremental",
        "--python-version=3.9",
    ):
        assert flag in typing


def test_release_builds_one_exact_wheel_sdist_pair() -> None:
    workflow = _workflow()
    job = "build-and-verify"
    export = run_step_text(workflow, job, 'RELEASE_ROOT="${RUNNER_TEMP}/cap-python-source"')
    build = run_step_text(workflow, job, "rm -rf dist")
    selection = run_step_text(workflow, job, "shopt -s nullglob")
    run_step_index(
        workflow, job, 'test "$(/usr/bin/git rev-parse HEAD)" = "$GITHUB_SHA"'
    )
    assert build.count("python -m build") == 1
    assert 'RELEASE_SOURCE="$RELEASE_ROOT/sdk/python"' in export
    archive_command = (
        '/usr/bin/git --no-replace-objects archive --format=tar --prefix=sdk/python/ '
        '"${GITHUB_SHA}:sdk/python"'
    )
    assert archive_command in export
    assert 'python -m build --outdir dist/release "$RELEASE_SOURCE"' in build
    assert "--outdir dist/release sdk/python" not in build
    assert "${#wheels[@]} != 1" in selection
    assert "${#sdists[@]} != 1" in selection
    run_step_index(workflow, job, 'python -m twine check "$WHEEL" "$SDIST"')


def test_release_records_verifier_checksums_and_inventory_evidence() -> None:
    workflow = _workflow()
    verify_body = run_step_text(
        workflow, "build-and-verify",
        'python "$RELEASE_SOURCE/scripts/verify_artifacts.py"',
    )
    checksums_body = id_step_text(workflow, "build-and-verify", "release_hashes")
    assert "python sdk/python/scripts/verify_artifacts.py" not in verify_body
    assert '--wheel "$WHEEL"' in verify_body and '--sdist "$SDIST"' in verify_body
    assert "tee dist/evidence/artifact-verification.json" in verify_body
    assert "sha256sum" in checksums_body and "SHA256SUMS" in checksums_body
    upload_path = str(action_input_value(
        workflow, "build-and-verify", "actions/upload-artifact@", "path"
    ))
    assert "dist/evidence/artifact-verification.json" in upload_path
    assert "dist/evidence/SHA256SUMS" in upload_path
    require_action_input(
        workflow, "build-and-verify", "actions/upload-artifact@",
        "if-no-files-found", "error",
    )
    assert 'echo "wheel_sha256=$wheel_sha256" >> "$GITHUB_OUTPUT"' in checksums_body
    assert 'echo "sdist_sha256=$sdist_sha256" >> "$GITHUB_OUTPUT"' in checksums_body
    report_validation = (
        'python "$RELEASE_SOURCE/scripts/validate_release.py" --tag "$RELEASE_TAG" '
        '--artifact-report dist/evidence/artifact-verification.json'
    )
    validation_body = run_step_text(
        workflow, "build-and-verify", report_validation
    )
    assert report_validation in validation_body
    verify = run_step_index(
        workflow, "build-and-verify", 'python "$RELEASE_SOURCE/scripts/verify_artifacts.py"'
    )
    validation = run_step_index(workflow, "build-and-verify", report_validation)
    checksums = id_step_index(workflow, "build-and-verify", "release_hashes")
    upload = action_step_index(workflow, "build-and-verify", "actions/upload-artifact@")
    assert verify < validation < checksums < upload


def test_release_rejects_branch_before_tools_and_exports_after_gates() -> None:
    workflow = _workflow()
    job = "build-and-verify"
    tag_guard = run_step_index(
        workflow, job, 'if [[ "${GITHUB_REF_TYPE}" != "tag"'
    )
    checkout = action_step_index(workflow, job, "actions/checkout@")
    install = run_step_index(workflow, job, 'python -m pip install "sdk/python[dev]"')
    archive = run_step_index(
        workflow, job, "/usr/bin/git --no-replace-objects archive --format=tar"
    )
    build = run_step_index(workflow, job, "python -m build --outdir dist/release")
    assert tag_guard < checkout < install
    assert run_step_index(
        workflow, job, "python sdk/python/scripts/generate_protos.py --check"
    ) < archive < build
    assert run_step_index(workflow, job, "python -m pytest") < archive
    assert "pip install --upgrade pip" not in run_text(workflow, job)


def test_release_rechecks_bytes_then_publishes_only_verified_directory() -> None:
    workflow = _workflow()
    commands = run_text(workflow, "publish")
    download = action_step_index(workflow, "publish", "actions/download-artifact@")
    recheck = run_step_index(workflow, "publish", "set -euo pipefail")
    recheck_body = run_step_text(workflow, "publish", "set -euo pipefail")
    publish = action_step_index(workflow, "publish", "pypa/gh-action-pypi-publish@")
    assert download < recheck < publish
    assert "python -m build" not in commands
    assert "cp " not in commands and "mv " not in commands
    assert "${#wheels[@]} != 1" in recheck_body
    assert "${#sdists[@]} != 1" in recheck_body
    assert job_value(workflow, "publish", "env", "EXPECTED_WHEEL_SHA256") == (
        "${{ needs.build-and-verify.outputs.wheel_sha256 }}"
    )
    assert job_value(workflow, "publish", "env", "EXPECTED_SDIST_SHA256") == (
        "${{ needs.build-and-verify.outputs.sdist_sha256 }}"
    )
    assert ".artifacts.wheel.sha256" in recheck_body
    assert ".artifacts.sdist.sha256" in recheck_body
    assert "sha256sum --check" in recheck_body
    require_action_input(
        workflow, "publish", "pypa/gh-action-pypi-publish@",
        "packages-dir", "dist/release",
    )
    assert "skip-existing" not in action_inputs(
        workflow, "publish", "pypa/gh-action-pypi-publish@"
    )


def test_release_actions_are_commit_pinned() -> None:
    workflow = _workflow()
    actions = (*uses_values(workflow, "build-and-verify"),
               *uses_values(workflow, "publish"))
    assert actions
    for action in actions:
        owner, revision = action.rsplit("@", 1)
        assert "/" in owner
        assert re.fullmatch(r"[0-9a-f]{40}", revision), action
    assert "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683" in actions
    assert "actions/setup-python@a26af69be951a213d495a4c3e4e4022e16d87065" in actions
    assert "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02" in actions
    assert "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093" in actions
