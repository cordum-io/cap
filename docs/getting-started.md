# First CAP Job (under 10 minutes)

> **Just want to try CAP?** The onboarding fixes this guide is built around
> have shipped in every release since v2.15.0 (current: **v2.16.1**). Install
> the published package instead of building from source —
> `pip install cap-sdk-python`, `npm install cap-sdk-node`, or
> `go get github.com/cordum-io/cap/v2` — then jump to whichever language
> section below matches (2A Go, 2B Python, 2C Node), skip its "obtain the
> candidate artifact" step, and use your installed package in its place. The
> rest of this guide (steps 0–4) is the from-source path: useful for
> contributors testing unreleased changes, not required to try CAP.

Starting from an empty workspace, this lab builds or downloads a candidate SDK
artifact, installs it into an empty **consumer** directory, and runs one CAP job
in Go, Python, or Node. It uses a direct NATS worker subject so every component
in the round trip is visible.

> **Development-only boundary:** the client code below publishes a validated
> protobuf packet directly to `job.echo`. That bypasses the Gateway, Scheduler,
> Safety Kernel, policy checks, authorization, authenticated identity, durable
> state, and retries. Matching `trace_id` and `job_id` is correlation, not
> authentication. Do not use this route as a production architecture.

## What runs

The lab starts exactly these components:

1. NATS `2.12.6-alpine`, with client port `4222` and monitoring port `8222`.
2. One language-specific echo worker subscribed to `job.echo`.
3. One language-specific submitter subscribed to `sys.job.result`.

It does **not** start a Gateway, Scheduler, Safety Kernel, memory resolver,
state store, retry controller, or trust authority. The `demo://context/...` and
`demo://result/...` values are opaque illustrative pointers; no service reads
or writes data at those locations.

The local flow is:

```text
client -- BusPacket{JobRequest} --> job.echo --> worker
client <-- BusPacket{JobResult} --- sys.job.result <-- worker
```

A governed deployment is different:

```text
external client --> Gateway / trusted ingress --> sys.job.submit --> Scheduler
Scheduler --> Safety decision --> Scheduler --> job.<pool> --> worker
```

The low-level `Submit`, `submit_job`, and `submitJob` helpers encode a packet
and publish Scheduler ingress at `sys.job.submit`; they do not implement an
external Gateway or authenticate a caller. Use them only behind or inside a
trusted ingress. `JobRequest.topic` is payload metadata; NATS does not inspect
it and cannot reroute a packet already published to `sys.job.submit`. The
Scheduler performs the governed dispatch.

## Prerequisites

- Docker with Compose or the Docker CLI.
- A POSIX-compatible shell (Linux/macOS, WSL, or Git Bash/MSYS2) for the command
  blocks below.
- Git when building a candidate artifact instead of receiving one from a
  maintainer.
- `curl` for the NATS health/readiness checks.
- One language toolchain:
  - Go `1.25.12` or newer.
  - CPython `3.9` through `3.14`.
  - Node.js `18`, `20`, or `22` with npm.
- The candidate artifact for the one language you selected:
  - Go uses `CAP_GO_PROXY_URL` plus the commit-unique `CAP_GO_VERSION`.
  - Python uses the absolute wheel path in `CAP_PYTHON_ARTIFACT`.
  - Node uses the absolute `.tgz` path in `CAP_NODE_ARTIFACT`.

## 0. Obtain candidate artifacts

This section builds a fresh SDK artifact directly from source instead of
installing the published package — useful for testing an unreleased change,
not required to try CAP (see the note at the top of this guide). CI creates
and installs the candidate proxy, wheel, and tarball in clean directories
without a source-tree fallback.

If a maintainer supplied the artifact for your language, set only its `CAP_*`
variable(s) and continue to step 1.
Otherwise, the following commands are the complete acquisition path from an
empty workspace after this change reaches the repository's default branch:

```bash
mkdir cap-first-job-work && cd cap-first-job-work
git clone --depth 1 https://github.com/cordum-io/cap.git cap-source
cd cap-source
```

Build **only one** of the following artifacts from `cap-source`.

For Go, Python is also required because the local proxy builder is a typed
standard-library Python tool. The commit suffix prevents a stale module-cache
entry from representing different candidate bytes:

```bash
mkdir -p .artifacts/go
export CAP_GO_VERSION="v2.5.3-onboarding.0.g$(git rev-parse --short=12 HEAD)"
python tools/onboarding/build_go_proxy.py \
  --output .artifacts/go --version "$CAP_GO_VERSION"
export CAP_GO_PROXY_URL="$(python -c \
  'from pathlib import Path; print(Path(".artifacts/go").resolve().as_uri())')"
```

For Python:

```bash
rm -rf .artifacts/python
mkdir -p .artifacts/python
python -m venv .artifact-venv
if [ -f .artifact-venv/bin/activate ]; then
  . .artifact-venv/bin/activate
else
  . .artifact-venv/Scripts/activate
fi
python -m pip install "build>=1.2,<2"
python -m build --outdir .artifacts/python sdk/python
CAP_PYTHON_ARTIFACT="$(python -c \
  'from pathlib import Path; (artifact,) = sorted(Path(".artifacts/python").glob("*.whl")); print(artifact.resolve())')" &&
test -n "$CAP_PYTHON_ARTIFACT" &&
export CAP_PYTHON_ARTIFACT
```

For Node:

```bash
mkdir -p .artifacts/node
(cd sdk/node && npm ci && npm run build && \
  npm pack --pack-destination ../../.artifacts/node)
CAP_NODE_ARTIFACT="$(node -e '
  const fs = require("fs"), path = require("path");
  const dir = path.resolve(".artifacts/node");
  const files = fs.readdirSync(dir).filter((name) => name.endsWith(".tgz"));
  if (files.length !== 1) {
    console.error(`expected one .tgz in ${dir}; found ${files.length}`);
    process.exitCode = 1;
  }
  else process.stdout.write(path.join(dir, files[0]));
')" && test -n "$CAP_NODE_ARTIFACT" && export CAP_NODE_ARTIFACT
```

After the selected block, return to the workspace with `cd ..`. The later
consumer commands install only the generated artifact; they do not import
packages from `cap-source`.

The repository CI workflow executes this local-proxy artifact path as an
installed-artifact onboarding check.

## 1. Start the pinned broker

```bash
docker run -d --rm --name cap-first-job-nats \
  -p 127.0.0.1:4222:4222 -p 127.0.0.1:8222:8222 \
  nats:2.12.6-alpine@sha256:1cfc36e2e5e638243d8c722f72c954cd0ec4b15ee82fadbc718ce12e2b3c1652 -m 8222
curl -fsS http://127.0.0.1:8222/healthz
```

The health response must be JSON whose `status` field is `"ok"`. If the
container name or ports are already in use, run the cleanup command at the end
before retrying.

After starting a worker, use NATS monitoring as the readiness authority:

```bash
curl -fsS "http://127.0.0.1:8222/subsz?subs=1"
```

Do not publish until an object in `subscriptions_list` has its `subject` field
equal to `"job.echo"`. Go and Node print READY only after flushing their subscription.
Python prints STARTING because the public `run_worker` call blocks; the
monitoring response, not that log line, proves readiness.

## 2A. Go

Start from an empty directory:

```bash
mkdir cap-first-go && cd cap-first-go
mkdir worker client
go mod init example.com/cap-first-go
mkdir .gomodcache
```

Save this canonical worker as `worker/main.go`:

<!-- cap-snippet:go-worker:start -->
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/sdk/go/worker"
	"github.com/nats-io/nats.go"
)

const (
	workerID      = "simple-echo-go-worker"
	workerSubject = "job.echo"
)

func main() {
	if err := run(); err != nil {
		log.Printf("CAP simple echo worker failed: %v", err)
		os.Exit(1)
	}
}

func run() (err error) {
	nc, err := nats.Connect(natsURL(), nats.Timeout(5*time.Second), nats.DrainTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer func() {
		if cleanupErr := cleanupNATS(nc); err == nil {
			err = cleanupErr
		}
	}()
	w := newEchoWorker(nc)
	if err := w.Start(); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	if err := nc.FlushTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("propagate worker subscription: %w", err)
	}
	log.Print("WARNING: unsigned local-development worker; no authenticated identity or policy enforcement")
	log.Printf("CAP_SIMPLE_ECHO_WORKER_READY subject=%s", workerSubject)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}

func newEchoWorker(nc worker.NATSConn) *worker.Worker {
	return &worker.Worker{
		NATS: nc, Subject: workerSubject, Handler: echo, SenderID: workerID,
		AllowUnsigned: true,
	}
}

func echo(_ context.Context, req *agentv1.JobRequest) (*agentv1.JobResult, error) {
	log.Printf("received job %q", req.GetJobId())
	return &agentv1.JobResult{
		JobId:     req.GetJobId(),
		Status:    agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
		ResultPtr: "demo://result/" + req.GetJobId(),
		WorkerId:  workerID,
	}, nil
}

func natsURL() string {
	if value := strings.TrimSpace(os.Getenv("CAP_NATS_URL")); value != "" {
		return value
	}
	return nats.DefaultURL
}

func cleanupNATS(nc *nats.Conn) error {
	if err := nc.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		nc.Close()
		return fmt.Errorf("drain NATS: %w", err)
	}
	nc.Close()
	return nil
}
```
<!-- cap-snippet:go-worker:end -->

Save this canonical client as `client/main.go`:

<!-- cap-snippet:go-client:start -->
```go
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultResultTimeout = 15 * time.Second
	directSubject        = "job.echo"
	resultTimeoutEnv     = "CAP_RESULT_TIMEOUT_SECONDS"
	successMarker        = "CAP_SIMPLE_ECHO_SUCCESS"
)

func main() {
	if err := run(); err != nil {
		log.Printf("CAP simple echo failed: %v", err)
		os.Exit(1)
	}
}

func run() (err error) {
	timeout, err := resultTimeout()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	nc, err := nats.Connect(natsURL(), nats.Timeout(timeout), nats.DrainTimeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	sub, err := nc.SubscribeSync(capsdk.SubjectResult)
	if err != nil {
		nc.Close()
		return fmt.Errorf("subscribe for results: %w", err)
	}
	defer func() {
		if cleanupErr := cleanupNATS(nc, sub); err == nil {
			err = cleanupErr
		}
	}()
	if err := flushBefore(nc, deadline); err != nil {
		return err
	}
	req, packet, err := newRequest()
	if err != nil {
		return err
	}
	data, err := capsdk.MarshalDeterministic(packet)
	if err != nil {
		return fmt.Errorf("encode request packet: %w", err)
	}
	// DEVELOPMENT ONLY: this raw publish bypasses the Gateway, Scheduler,
	// Safety Kernel, authorization, policy, retries, and durable state.
	log.Print("WARNING: direct local-development publish; correlation is not authentication")
	if err := nc.Publish(req.GetTopic(), data); err != nil {
		return fmt.Errorf("publish request: %w", err)
	}
	if err := flushBefore(nc, deadline); err != nil {
		return err
	}
	result, err := waitForResult(sub, deadline, packet.GetTraceId(), req.GetJobId())
	if err != nil {
		return err
	}
	if result.GetStatus() != agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		return fmt.Errorf("job %q finished with %s: %s", req.GetJobId(), result.GetStatus(), result.GetErrorMessage())
	}
	fmt.Println(successMarker)
	return nil
}

func newRequest() (*agentv1.JobRequest, *agentv1.BusPacket, error) {
	suffix, err := randomSuffix()
	if err != nil {
		return nil, nil, fmt.Errorf("create correlation IDs: %w", err)
	}
	jobID := "simple-echo-go-" + suffix
	req := &agentv1.JobRequest{
		JobId:      jobID,
		Topic:      directSubject,
		ContextPtr: "demo://context/" + jobID,
	}
	if err := validateDirectRequest(req); err != nil {
		return nil, nil, err
	}
	packet := &agentv1.BusPacket{
		TraceId:         "trace-simple-echo-go-" + suffix,
		SenderId:        "simple-echo-go-client",
		CreatedAt:       timestamppb.Now(),
		ProtocolVersion: capsdk.DefaultProtocolVersion,
		Payload:         &agentv1.BusPacket_JobRequest{JobRequest: req},
	}
	if err := capsdk.ValidateBusPacket(packet); err != nil {
		return nil, nil, fmt.Errorf("validate request packet: %w", err)
	}
	return req, packet, nil
}

func validateDirectRequest(req *agentv1.JobRequest) error {
	if err := capsdk.ValidateJobRequest(req); err != nil {
		return fmt.Errorf("validate request: %w", err)
	}
	subject := strings.TrimSpace(req.GetTopic())
	if subject == "" || strings.ContainsAny(subject, "*>\t\r\n ") {
		return fmt.Errorf("direct subject %q must be nonblank and contain no wildcards or whitespace", req.GetTopic())
	}
	for _, token := range strings.Split(subject, ".") {
		if token == "" {
			return fmt.Errorf("direct subject %q must contain no empty tokens", req.GetTopic())
		}
	}
	return nil
}

func waitForResult(sub *nats.Subscription, deadline time.Time, traceID, jobID string) (*agentv1.JobResult, error) {
	for {
		wait, err := remaining(deadline)
		if err != nil {
			return nil, fmt.Errorf("wait for job %q: %w", jobID, err)
		}
		msg, err := sub.NextMsg(wait)
		if errors.Is(err, nats.ErrTimeout) {
			return nil, fmt.Errorf("wait for job %q: result timeout", jobID)
		}
		if err != nil {
			return nil, fmt.Errorf("read result: %w", err)
		}
		var packet agentv1.BusPacket
		if err := proto.Unmarshal(msg.Data, &packet); err != nil {
			continue
		}
		result := packet.GetJobResult()
		if result == nil || packet.GetTraceId() != traceID || result.GetJobId() != jobID {
			continue
		}
		if err := capsdk.ValidateBusPacket(&packet); err != nil {
			return nil, fmt.Errorf("matching result packet is invalid: %w", err)
		}
		if err := capsdk.ValidateJobResult(result); err != nil {
			return nil, fmt.Errorf("matching job result is invalid: %w", err)
		}
		switch result.GetStatus() {
		case agentv1.JobStatus_JOB_STATUS_PENDING,
			agentv1.JobStatus_JOB_STATUS_SCHEDULED,
			agentv1.JobStatus_JOB_STATUS_DISPATCHED,
			agentv1.JobStatus_JOB_STATUS_RUNNING:
			continue
		case agentv1.JobStatus_JOB_STATUS_SUCCEEDED,
			agentv1.JobStatus_JOB_STATUS_FAILED,
			agentv1.JobStatus_JOB_STATUS_CANCELLED,
			agentv1.JobStatus_JOB_STATUS_DENIED,
			agentv1.JobStatus_JOB_STATUS_TIMEOUT,
			agentv1.JobStatus_JOB_STATUS_FAILED_RETRYABLE,
			agentv1.JobStatus_JOB_STATUS_FAILED_FATAL:
			return result, nil
		default:
			return nil, fmt.Errorf("matching job result has unknown status %d", result.GetStatus())
		}
	}
}

func resultTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(resultTimeoutEnv))
	if raw == "" {
		return defaultResultTimeout, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", resultTimeoutEnv, raw)
	}
	return time.Duration(seconds) * time.Second, nil
}

func natsURL() string {
	if value := strings.TrimSpace(os.Getenv("CAP_NATS_URL")); value != "" {
		return value
	}
	return nats.DefaultURL
}

func randomSuffix() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func flushBefore(nc *nats.Conn, deadline time.Time) error {
	wait, err := remaining(deadline)
	if err != nil {
		return err
	}
	if err := nc.FlushTimeout(wait); err != nil {
		return fmt.Errorf("flush NATS: %w", err)
	}
	return nil
}

func remaining(deadline time.Time) (time.Duration, error) {
	wait := time.Until(deadline)
	if wait <= 0 {
		return 0, errors.New("result deadline exceeded")
	}
	return wait, nil
}

func cleanupNATS(nc *nats.Conn, sub *nats.Subscription) error {
	var cleanupErr error
	if err := sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrBadSubscription) {
		cleanupErr = fmt.Errorf("unsubscribe results: %w", err)
	}
	if err := nc.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("drain NATS: %w", err))
	}
	nc.Close()
	return cleanupErr
}
```
<!-- cap-snippet:go-client:end -->

After both source files exist, resolve the exact candidate version and vendor
its dependency graph. Resolving before saving the files leaves Go with no
consumer imports to analyze and produces an incomplete vendor tree. The first
download is deliberately local-only, into the new module cache, so a missing
candidate proxy file cannot fall through to a remote module with the same
version.

```bash
GOWORK=off GOMODCACHE="$PWD/.gomodcache" \
  GOPROXY="$CAP_GO_PROXY_URL" \
  GONOSUMDB=github.com/cordum-io/cap/v2 \
  go mod download "github.com/cordum-io/cap/v2@$CAP_GO_VERSION"
GOWORK=off GOMODCACHE="$PWD/.gomodcache" \
  GOPROXY="$CAP_GO_PROXY_URL,https://proxy.golang.org,direct" \
  GONOSUMDB=github.com/cordum-io/cap/v2 \
  go get "github.com/cordum-io/cap/v2@$CAP_GO_VERSION"
GOWORK=off GOMODCACHE="$PWD/.gomodcache" \
  GOPROXY="$CAP_GO_PROXY_URL,https://proxy.golang.org,direct" \
  GONOSUMDB=github.com/cordum-io/cap/v2 go mod tidy
GOWORK=off GOMODCACHE="$PWD/.gomodcache" \
  GOPROXY="$CAP_GO_PROXY_URL,https://proxy.golang.org,direct" \
  GONOSUMDB=github.com/cordum-io/cap/v2 go mod vendor
```

In terminal 1, run the worker:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 \
GOWORK=off go run -mod=vendor ./worker
```

Confirm `job.echo` in `/subsz`, then in terminal 2 run the bounded client:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 \
CAP_RESULT_TIMEOUT_SECONDS=10 \
GOWORK=off go run -mod=vendor ./client
```

Success is exit code `0` with exactly one `CAP_SIMPLE_ECHO_SUCCESS` marker.
Timeout, invalid matching data, or a non-success terminal result exits nonzero.

## 2B. Python

Start from an empty directory and install only the candidate wheel:

```bash
mkdir cap-first-python && cd cap-first-python
python -m venv .venv
if [ -f .venv/bin/activate ]; then
  . .venv/bin/activate
else
  . .venv/Scripts/activate
fi
python -m pip install --upgrade pip
python -m pip install "$CAP_PYTHON_ARTIFACT"
python -m pip check
```

Save this canonical worker as `worker.py`:

<!-- cap-snippet:python-worker:start -->
```python
import asyncio
import os

import cap
from cap.pb.cordum.agent.v1 import job_pb2


WORKER_ID = "worker-echo-py"
WORKER_SUBJECT = "job.echo"


async def handle(request: job_pb2.JobRequest) -> job_pb2.JobResult:
    return job_pb2.JobResult(
        job_id=request.job_id,
        status=job_pb2.JOB_STATUS_SUCCEEDED,
        result_ptr=f"demo://result/{request.job_id}",
        worker_id=WORKER_ID,
    )


async def main() -> None:
    print(f"CAP_SIMPLE_ECHO_WORKER_STARTING subject={WORKER_SUBJECT}", flush=True)
    await cap.run_worker(
        nats_url=os.getenv("CAP_NATS_URL", "nats://127.0.0.1:4222"),
        subject=WORKER_SUBJECT,
        handler=handle,
        sender_id=WORKER_ID,
    )


if __name__ == "__main__":
    asyncio.run(main())
```
<!-- cap-snippet:python-worker:end -->

Save this canonical client as `client.py`:

<!-- cap-snippet:python-client:start -->
```python
import asyncio
import math
import os
import sys
from typing import Optional, Sequence
from uuid import uuid4

import cap
import cap.constants
import nats
from google.protobuf import timestamp_pb2
from google.protobuf.message import DecodeError
from nats.aio.client import Client as NATS
from nats.aio.subscription import Subscription
from nats.errors import TimeoutError as NATSTimeoutError

from cap.pb.cordum.agent.v1 import buspacket_pb2, job_pb2


WORKER_SUBJECT = "job.echo"
CLIENT_ID = "client-echo-py"
EXIT_TRANSPORT = 2
EXIT_TIMEOUT = 4
EXIT_TERMINAL = 5
EXIT_PROTOCOL = 6
CONNECT_TIMEOUT_SECONDS = 5.0


def _result_timeout_seconds() -> float:
    raw = os.getenv("CAP_RESULT_TIMEOUT_SECONDS", "10")
    try:
        timeout = float(raw)
    except ValueError as exc:
        raise ValueError("CAP_RESULT_TIMEOUT_SECONDS must be a number") from exc
    if not math.isfinite(timeout) or timeout <= 0:
        raise ValueError(
            "CAP_RESULT_TIMEOUT_SECONDS must be finite and greater than zero"
        )
    return timeout


def _validation_summary(errors: Sequence[cap.ValidationError]) -> str:
    return "; ".join(f"{error.field}: {error.message}" for error in errors)


def _error_summary(error: Exception) -> str:
    return str(error).strip() or type(error).__name__


async def _report_nats_error(error: Exception) -> None:
    print(f"NATS client warning: {_error_summary(error)}", file=sys.stderr)


def _build_request(job_id: str) -> job_pb2.JobRequest:
    request = job_pb2.JobRequest(
        job_id=job_id,
        topic=WORKER_SUBJECT,
        context_ptr=f"demo://context/{job_id}",
    )
    errors = cap.validate_job_request(request)
    if errors:
        raise ValueError(f"invalid JobRequest: {_validation_summary(errors)}")
    has_whitespace = any(char.isspace() for char in request.topic)
    has_empty_token = any(not token for token in request.topic.split("."))
    has_wildcard = "*" in request.topic or ">" in request.topic
    if has_whitespace or has_empty_token or has_wildcard:
        raise ValueError(
            "direct worker subject requires nonempty tokens and no "
            "whitespace or wildcards"
        )
    return request


def _build_packet(
    request: job_pb2.JobRequest, trace_id: str
) -> buspacket_pb2.BusPacket:
    created_at = timestamp_pb2.Timestamp()
    created_at.GetCurrentTime()
    packet = buspacket_pb2.BusPacket(
        trace_id=trace_id,
        sender_id=CLIENT_ID,
        protocol_version=cap.constants.DEFAULT_PROTOCOL_VERSION,
    )
    packet.created_at.CopyFrom(created_at)
    packet.job_request.CopyFrom(request)
    errors = cap.validate_bus_packet(packet)
    if errors:
        raise ValueError(f"invalid BusPacket: {_validation_summary(errors)}")
    return packet


def _status_name(status: int) -> str:
    try:
        return str(job_pb2.JobStatus.Name(status))
    except ValueError:
        return f"UNKNOWN({status})"


def _matching_verdict(
    packet: buspacket_pb2.BusPacket, job_id: str, trace_id: str
) -> Optional[int]:
    if (
        packet.trace_id != trace_id
        or packet.WhichOneof("payload") != "job_result"
    ):
        return None
    if packet.job_result.job_id != job_id:
        return None
    errors = cap.validate_bus_packet(packet)
    if errors:
        print(
            f"matching result is invalid: {_validation_summary(errors)}",
            file=sys.stderr,
        )
        return EXIT_PROTOCOL
    status = packet.job_result.status
    if status == job_pb2.JOB_STATUS_SUCCEEDED:
        print(f"CAP_SIMPLE_ECHO_SUCCESS job_id={job_id}")
        return 0
    if status in (
        job_pb2.JOB_STATUS_FAILED,
        job_pb2.JOB_STATUS_CANCELLED,
        job_pb2.JOB_STATUS_DENIED,
        job_pb2.JOB_STATUS_TIMEOUT,
        job_pb2.JOB_STATUS_FAILED_RETRYABLE,
        job_pb2.JOB_STATUS_FAILED_FATAL,
    ):
        print(f"job ended with {_status_name(status)}", file=sys.stderr)
        return EXIT_TERMINAL
    if status in (
        job_pb2.JOB_STATUS_PENDING,
        job_pb2.JOB_STATUS_SCHEDULED,
        job_pb2.JOB_STATUS_DISPATCHED,
        job_pb2.JOB_STATUS_RUNNING,
    ):
        return None
    print(
        f"matching result has unsupported status {_status_name(status)}",
        file=sys.stderr,
    )
    return EXIT_PROTOCOL


async def _wait_for_result(
    subscription: Subscription, deadline: float, job_id: str, trace_id: str
) -> int:
    loop = asyncio.get_running_loop()
    while True:
        remaining = deadline - loop.time()
        if remaining <= 0:
            print(
                "timed out waiting for a matching successful JobResult",
                file=sys.stderr,
            )
            return EXIT_TIMEOUT
        try:
            message = await subscription.next_msg(timeout=remaining)
        except (asyncio.TimeoutError, NATSTimeoutError):
            print(
                "timed out waiting for a matching successful JobResult",
                file=sys.stderr,
            )
            return EXIT_TIMEOUT
        packet = buspacket_pb2.BusPacket()
        try:
            packet.ParseFromString(message.data)
        except DecodeError:
            continue
        verdict = _matching_verdict(packet, job_id, trace_id)
        if verdict is not None:
            return verdict


async def _cleanup(
    connection: Optional[NATS], subscription: Optional[Subscription]
) -> None:
    if subscription is not None:
        try:
            await subscription.unsubscribe()
        except Exception as exc:
            print(f"cleanup warning: unsubscribe failed: {exc}", file=sys.stderr)
    if connection is not None:
        try:
            await connection.drain()
        except Exception as exc:
            print(f"cleanup warning: NATS drain failed: {exc}", file=sys.stderr)


async def main() -> int:
    job_id = f"job-echo-{uuid4().hex}"
    trace_id = f"trace-echo-{uuid4().hex}"
    try:
        request = _build_request(job_id)
        packet = _build_packet(request, trace_id)
        timeout = _result_timeout_seconds()
    except ValueError as exc:
        print(exc, file=sys.stderr)
        return EXIT_PROTOCOL

    connection: Optional[NATS] = None
    subscription: Optional[Subscription] = None
    try:
        nats_url = os.getenv("CAP_NATS_URL", "nats://127.0.0.1:4222")
        connection = await asyncio.wait_for(
            nats.connect(
                nats_url,
                name=CLIENT_ID,
                error_cb=_report_nats_error,
                connect_timeout=2,
                max_reconnect_attempts=1,
            ),
            timeout=CONNECT_TIMEOUT_SECONDS,
        )
        subscription = await connection.subscribe(cap.SUBJECT_RESULT)
        await connection.flush()
        deadline = asyncio.get_running_loop().time() + timeout
        print(
            "DEVELOPMENT ONLY: direct NATS publish bypasses Gateway, "
            "Scheduler, and Safety Kernel.",
            file=sys.stderr,
        )
        payload = packet.SerializeToString(deterministic=True)
        await connection.publish(request.topic, payload)
        await connection.flush()
        return await _wait_for_result(subscription, deadline, job_id, trace_id)
    except Exception as exc:
        print(f"NATS transport failed: {_error_summary(exc)}", file=sys.stderr)
        return EXIT_TRANSPORT
    finally:
        await _cleanup(connection, subscription)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
```
<!-- cap-snippet:python-client:end -->

In terminal 1, run the worker:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 python worker.py
```

Wait for `job.echo` in `/subsz`, then in terminal 2 run:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 \
CAP_RESULT_TIMEOUT_SECONDS=10 \
python client.py
```

Success is exit code `0` with exactly one `CAP_SIMPLE_ECHO_SUCCESS` marker.
Timeout exits `4`; invalid matching data and terminal failure also exit nonzero.

## 2C. Node.js

Start from an empty directory and install only the candidate tarball:

```bash
mkdir cap-first-node && cd cap-first-node
npm init -y
npm install --ignore-scripts --no-audit --no-fund "$CAP_NODE_ARTIFACT"
npm ls --omit=dev
```

Save this canonical worker as `worker.js`:

<!-- cap-snippet:node-worker:start -->
```javascript
const { connectNATS, startWorker } = require("cap-sdk-node");

const JOB_SUBJECT = "job.echo";
const WORKER_ID = "simple-echo-node-worker";

async function main() {
  const nc = await connectNATS({
    url: process.env.CAP_NATS_URL ?? "nats://127.0.0.1:4222",
    name: WORKER_ID,
  });
  try {
    await startWorker({
      nc,
      subject: JOB_SUBJECT,
      senderId: WORKER_ID,
      handler: async (req) => ({
        jobId: req.jobId,
        status: "JOB_STATUS_SUCCEEDED",
        resultPtr: `demo://result/${req.jobId}`,
        workerId: WORKER_ID,
      }),
    });
    await nc.flush();
    console.log(`CAP_SIMPLE_ECHO_READY worker=${WORKER_ID} subject=${JOB_SUBJECT}`);
  } catch (error) {
    await nc.drain().catch(() => undefined);
    throw error;
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
```
<!-- cap-snippet:node-worker:end -->

Save this canonical client as `client.js`:

<!-- cap-snippet:node-client:start -->
```javascript
const { randomUUID } = require("node:crypto");
const {
  connectNATS,
  DEFAULT_PROTOCOL_VERSION,
  loadRoot,
  SUBJECT_RESULT,
  validateBusPacket,
} = require("cap-sdk-node");

const JOB_SUBJECT = "job.echo";
const TERMINAL_FAILURES = new Set([
  "JOB_STATUS_FAILED",
  "JOB_STATUS_CANCELLED",
  "JOB_STATUS_DENIED",
  "JOB_STATUS_TIMEOUT",
  "JOB_STATUS_FAILED_RETRYABLE",
  "JOB_STATUS_FAILED_FATAL",
]);

function resultTimeoutMs() {
  const raw = process.env.CAP_RESULT_TIMEOUT_SECONDS ?? "15";
  const seconds = Number(raw);
  if (!Number.isFinite(seconds) || seconds <= 0) {
    throw new Error("CAP_RESULT_TIMEOUT_SECONDS must be a positive number");
  }
  return Math.max(1, Math.floor(seconds * 1000));
}

async function loadTypes() {
  const root = await loadRoot();
  return {
    busPacket: root.lookupType("cordum.agent.v1.BusPacket"),
    jobRequest: root.lookupType("cordum.agent.v1.JobRequest"),
    priorities: root.lookupEnum("cordum.agent.v1.JobPriority"),
    statuses: root.lookupEnum("cordum.agent.v1.JobStatus"),
  };
}

function timestampNow() {
  const milliseconds = Date.now();
  return {
    seconds: Math.floor(milliseconds / 1000),
    nanos: (milliseconds % 1000) * 1_000_000,
  };
}

function requireDirectSubject(topic) {
  const hasEmptyToken =
    typeof topic === "string" &&
    topic.split(".").some((token) => token.length === 0);
  if (
    typeof topic !== "string" ||
    topic.length === 0 ||
    hasEmptyToken ||
    /[\s*>]/u.test(topic)
  ) {
    throw new Error("direct JobRequest topic must be a concrete NATS subject");
  }
  return topic;
}

function buildRequest(types, jobId) {
  return types.jobRequest.create({
    jobId,
    topic: JOB_SUBJECT,
    priority: types.priorities.values.JOB_PRIORITY_INTERACTIVE,
    contextPtr: `demo://context/${jobId}`,
  });
}

function buildPacket(types, req, traceId) {
  requireDirectSubject(req.topic);
  const packet = types.busPacket.create({
    traceId,
    senderId: "simple-echo-node-client",
    createdAt: timestampNow(),
    protocolVersion: DEFAULT_PROTOCOL_VERSION,
    jobRequest: req,
  });
  const errors = validateBusPacket(packet);
  if (errors.length > 0) {
    throw new Error(`invalid JobRequest packet: ${JSON.stringify(errors)}`);
  }
  return packet;
}

function inspectResult(types, data, traceId, jobId) {
  let packet;
  try {
    packet = types.busPacket.decode(data);
  } catch {
    return { kind: "ignore" };
  }
  const result = packet.jobResult;
  if (!result || packet.traceId !== traceId || result.jobId !== jobId) {
    return { kind: "ignore" };
  }
  const errors = validateBusPacket(packet);
  if (errors.length > 0) {
    return {
      kind: "error",
      message: `invalid matching result: ${JSON.stringify(errors)}`,
    };
  }
  const status = types.statuses.valuesById[result.status];
  if (status === "JOB_STATUS_SUCCEEDED") return { kind: "success" };
  if (!status || TERMINAL_FAILURES.has(status)) {
    const detail = status ?? `unknown status ${result.status}`;
    return { kind: "error", message: `job ended with ${detail}` };
  }
  return { kind: "ignore" };
}

function subscribeForResult(nc, types, traceId, jobId, timeoutMs) {
  let settled = false;
  let timer;
  let resolveDone;
  let rejectDone;
  const done = new Promise((resolve, reject) => {
    resolveDone = resolve;
    rejectDone = reject;
  });
  void done.catch(() => undefined);
  const finish = (error) => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
    if (error) rejectDone(error);
    else resolveDone();
  };
  timer = setTimeout(
    () => finish(new Error(`timed out after ${timeoutMs}ms waiting for JobResult`)),
    timeoutMs
  );
  let subscription;
  try {
    subscription = nc.subscribe(SUBJECT_RESULT, {
      callback: (error, message) => {
        if (error) return finish(error);
        try {
          const verdict = inspectResult(types, message.data, traceId, jobId);
          if (verdict.kind === "success") finish();
          if (verdict.kind === "error") finish(new Error(verdict.message));
        } catch (inspectionError) {
          finish(inspectionError);
        }
      },
    });
  } catch (error) {
    finish(error);
    throw error;
  }
  const cancel = () => {
    if (settled) return;
    settled = true;
    clearTimeout(timer);
  };
  return { cancel, done, subscription };
}

async function cleanup(nc, observer, primaryError) {
  observer?.cancel();
  const cleanupErrors = [];
  try {
    observer?.subscription.unsubscribe();
  } catch (error) {
    cleanupErrors.push(error);
  }
  try {
    await nc.drain();
  } catch (error) {
    cleanupErrors.push(error);
  }
  if (cleanupErrors.length === 0) return;
  if (!primaryError) throw cleanupErrors[0];
  for (const error of cleanupErrors) {
    console.error(
      `secondary NATS cleanup error: ${
        error instanceof Error ? error.message : String(error)
      }`
    );
  }
}

async function main() {
  const nc = await connectNATS({
    url: process.env.CAP_NATS_URL ?? "nats://127.0.0.1:4222",
    name: "cap-simple-echo-node-client",
  });
  let observer;
  let primaryError;
  try {
    const types = await loadTypes();
    const jobId = `job-${randomUUID()}`;
    const traceId = `trace-${randomUUID()}`;
    const req = buildRequest(types, jobId);
    const packet = buildPacket(types, req, traceId);
    observer = subscribeForResult(nc, types, traceId, jobId, resultTimeoutMs());
    await nc.flush();

    // DEVELOPMENT ONLY: publishes to the worker pool without platform governance.
    console.warn(
      `DEV-ONLY direct publish to ${req.topic}; bypasses Gateway, Scheduler, ` +
        "Safety Kernel, policy, and authentication"
    );
    nc.publish(req.topic, types.busPacket.encode(packet).finish());
    await nc.flush();
    await observer.done;
    console.log(`CAP_SIMPLE_ECHO_SUCCESS job_id=${jobId} trace_id=${traceId}`);
  } catch (error) {
    primaryError = error;
    throw error;
  } finally {
    await cleanup(nc, observer, primaryError);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
```
<!-- cap-snippet:node-client:end -->

In terminal 1, run the worker:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 node worker.js
```

Wait for `job.echo` in `/subsz`, then in terminal 2 run:

```bash
CAP_NATS_URL=nats://127.0.0.1:4222 \
CAP_RESULT_TIMEOUT_SECONDS=10 \
node client.js
```

Success is exit code `0` with exactly one `CAP_SIMPLE_ECHO_SUCCESS` marker.
Timeout, invalid matching data, or a non-success terminal result exits nonzero.

## 3. Prove the failure path

Stop the worker, keep NATS running, and set
`CAP_RESULT_TIMEOUT_SECONDS=1` before running the client again. The client must
exit nonzero and must not print `CAP_SIMPLE_ECHO_SUCCESS`. This verifies that a
missing or late worker cannot produce a false green.

## 4. Clean up

Stop the worker with `Ctrl+C`, then remove the broker:

```bash
docker rm -f cap-first-job-nats
```

The `cap-first-*` consumer directories contain only normal language project
files and installed dependencies; remove them when finished.

## Troubleshooting and next steps

- If `/healthz` fails, inspect `docker logs cap-first-job-nats`.
- If `/subsz` lacks `job.echo`, do not submit; inspect the worker error first.
- If a client times out, verify that both terminals use the same
  `CAP_NATS_URL` and that the worker was ready before the one-shot publish.
- See [Troubleshooting](troubleshooting.md#5-job-stuck-in-pending) for the two
  transport topologies and why `JobRequest.topic` cannot reroute ingress.
- See the [simple-echo source](../examples/simple-echo/) and the
  [transport profile](../spec/09-transport-profile.md) for normative subjects.
