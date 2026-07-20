"""Static contracts for the Node release real-NATS gate."""

from pathlib import Path

import workflow_support


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "publish-node.yml"
NATS_IMAGE = (
    "nats:2.12.6-alpine@sha256:"
    "1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652"
)


def _publish_job(workflow: str) -> dict[object, object]:
    document = workflow_support.workflow_mapping(workflow)
    jobs = document.get("jobs")
    assert isinstance(jobs, dict)
    job = jobs.get("publish")
    assert isinstance(job, dict)
    return job


def test_release_installs_exact_required_nats_binary() -> None:
    workflow = WORKFLOW_PATH.read_text(encoding="utf-8")
    job = _publish_job(workflow)
    env = job.get("env")
    steps = job.get("steps")
    assert isinstance(env, dict)
    assert isinstance(steps, list)
    commands = "\n".join(
        str(step.get("run", "")) for step in steps if isinstance(step, dict)
    )
    assert env.get("NATS_IMAGE") == NATS_IMAGE
    for required in (
        'docker pull "$NATS_IMAGE"',
        'binary="$RUNNER_TEMP/nats-server"',
        'docker cp "$cid:/usr/local/bin/nats-server" "$binary"',
        'chmod 0755 "$binary"',
        'echo "CAP_NATS_SERVER_BIN=$binary" >> "$GITHUB_ENV"',
        'nats-server: v2.12.6',
        "npm run test:nats",
    ):
        assert required in commands
    assert "|| true" not in commands
