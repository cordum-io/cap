# Framework adapters, reference scheduler and TCK path: design note

> **Design note, not a normative spec.** It is an editorial, non-normative
> document, so it gets normal PR review (GOVERNANCE.md:76) and needs no RFC. It
> changes no proto, wire format, profile or TCK scenario. It fixes decisions for
> the downstream slices of epic-81acc19f (section 6).
>
> Repository citations are `path:line` at CAP commit
> `85097a10b8b77876107e0a2569b271dfa2fd6c98`. Installed framework code is cited
> as `<pkg>==<ver>: <path>:<line>`, with paths relative to site-packages.
> Conformance reports produced under this design are offline, self-hosted
> evidence. They are not certification or endorsement
> (spec/tck/adapter-v1.md:113-119), and the bundled reference adapter is not
> evidence of an independent implementation (spec/tck/adapter-v1.md:10-12).

Terms used below:
- **slice:** a task in epic-81acc19f.
- **NEW-1 and NEW-2:** two prerequisite slices that this note proposes (section 6).
- **adapter:** a CAP worker process that runs operator-registered framework code.

## 1. TCK path

### 1.1 Which cases apply

TCK adapter-v1 drives one process over JSONL on stdio, launched from an argv with no shell (spec/tck/adapter-v1.md:16-26). The process declares a role, `worker` or `control-plane` (internal/tck/proto.go:54-57).

A case whose role differs from the declared role is N/A (internal/tck/runner.go:44-51). A run is conformant only when every *applicable* required case passes (internal/tck/report.go:48-64), so an N/A case never counts as a pass.

A framework adapter is a worker. That makes:
- **28 cases applicable**: the worker-role cases.
- **21 cases N/A, outside this epic's DoD-1**: the control-plane cases. They are cancellation lines 5, 13 and 21; lifecycle 61-93; negotiation; safety; `duplicates/cp-duplicate-result-applies-once`; and retries 117, 132 and 147.

`cap-tck run` takes one `--suite` file (cmd/cap-tck/run.go:26) and one `--profile` (cmd/cap-tck/run.go:27). Profile selection is an exact match per scenario (internal/tck/scenario.go:69-78). Every worker-role case is required, so each framework needs **five runs**:

| run | suite file | `--profile` | worker-role scenario ids (line in file) |
|---|---|---|---|
| L | spec/tck/scenarios/core-lifecycle.json | core | `lifecycle/worker-running-to-succeeded` (:5), `lifecycle/worker-running-to-failed` (:13), `lifecycle/worker-running-to-failed-retryable` (:21), `lifecycle/worker-running-to-failed-fatal` (:29), `lifecycle/worker-same-state-idempotent` (:37), `lifecycle/worker-rejects-backwards-transition` (:45), `lifecycle/worker-rejects-second-terminal` (:53) |
| C | spec/tck/scenarios/core-cancellation.json | core | `cancellation/worker-honors-cancel` (:29) |
| M | spec/tck/scenarios/core-malformed.json | core | `malformed/truncated-wire` (:5), `malformed/overlong-wire` (:13), `malformed/non-minimal-encoding` (:21), `malformed/duplicate-oneof` (:29), `malformed/empty-envelope` (:37), `malformed/unsupported-protocol-version` (:45), `malformed/unknown-payload` (:53), `malformed/oversize-string` (:61), `malformed/oversize-map` (:69), `malformed/oversize-repeated` (:77) |
| D | spec/tck/scenarios/core-duplicates.json | cap-production | `duplicates/worker-exact-redelivery-invokes-handler-once` (:5), `duplicates/worker-exact-redelivery-replays-idempotent-result` (:20), `duplicates/worker-distinct-dispatch-is-not-suppressed` (:49) |
| R | spec/tck/scenarios/core-retries.json | cap-production | `retries/worker-new-attempt-invokes-once` (:5), `retries/worker-rejects-stale-attempt` (:21), `retries/worker-rejects-future-attempt` (:37), `retries/worker-rejects-wrong-worker-attempt` (:53), `retries/worker-honors-active-cancel` (:69), `retries/worker-rejects-stale-cancel` (:85), `retries/worker-rejects-wrong-worker-cancel` (:101) |

core-negotiation.json and core-safety.json contain no worker cases and are not run.

### 1.2 Decision: a stdio shim that drives the installed adapter over NATS

The rejected alternative is to give each adapter an adapter-v1 stdio mode. That would be a second, test-only code path, and the TCK would never exercise the adapter's real NATS worker path.

The chosen design is a shim. The new command `cap-framework-tck` gets a hidden `__shim` mode, following the `cap-tck __adapter` precedent (cmd/cap-tck/main.go:30-31, cmd/cap-tck/refadapter.go:14-19).

- **What cap-tck launches.** cap-tck launches the shim as the adapter-v1 process (internal/tck/adapter.go:25-32, internal/tck/adapter.go:82-116). Its handshake declares `role: worker` and `transport: nats`. The `sdk` field is `<distribution> <version>`, read from the installed wheel's metadata.
- **What the shim drives.** For each case, the shim drives the *installed* adapter, a separate process the harness started, over the harness's TLS NATS server.
- **Probe workloads.** The harness runs the adapter with a harness-owned operator config whose registry holds six probe workloads: `tck-succeed`, `tck-fail`, `tck-fail-retryable`, `tck-fail-fatal`, `tck-progress-twice` and `tck-wait-for-cancel`. Each probe is real framework code (a LangGraph graph, or a CrewAI crew on the section 3.8 double). Its first step emits the progress marker `invoked`.
- **Verdicts come only from observed traffic.**
  - The shim verifies every worker packet with `capsdk.VerifyTrustedProductionPacket` (sdk/go/production_signing.go:92-104).
  - It counts handler invocations by the number of `invoked` progress packets per dispatch identity.
  - It proves a negative ("not invoked", "dropped") with a barrier: it sends a fresh probe job and waits for that job's terminal result, then waits a bounded quiet window of 500 ms. This works because the harness keeps cap-sdk-python's `max_parallel` at its default of 1 (sdk/python/cap/runtime.py:354), so one worker handles jobs in delivery order.
  - A self-reported pass with no observed traffic is impossible by construction.
- **Which path each case takes:**
  - **Through `cap-reference-scheduler`:** the four `lifecycle/worker-running-to-*` cases, `lifecycle/worker-same-state-idempotent` and `cancellation/worker-honors-cancel`. The harness sets `MaxAttempts=1`, so `FAILED_RETRYABLE` is observed exactly as the worker reported it (spec/07-state-machine.md:52).
  - **Injected by the shim:** every other case. The shim signs as the scheduler, using the scheduler key from the harness trust files, and publishes on the worker's direct subject (section 2.2). The scheduler drops results for jobs it does not own (ErrJobNotFound), which is the correct behaviour.
- **How the shim reads the injected cases:**
  - **Retries and duplicates.** The shim reads the case params `attempt`, `activeAttempt`, `assignedWorkerId`, `deliveries` and `expect` (spec/tck/scenarios/core-retries.json:5-18). It first establishes `activeAttempt`, then delivers the case's packet. It maps `wrk-1` to the real worker id and maps any other id to an unknown worker. Expected outcomes mirror the reference adapter's model: internal/tck/refadapter_test.go:131-170 for duplicates and internal/tck/refadapter_test.go:209-235 for retries. A new attempt is accepted only when it equals `active+1`, and a cancel only when its attempt equals the active attempt.
  - **`lifecycle/worker-rejects-backwards-transition` and `-second-terminal`.** After a `SUCCEEDED` result, the shim redelivers the identical dispatch and sends a `JobCancel` for it. The case passes when no `invoked` marker, no `RUNNING` progress and no second terminal appears; at most a replay of the recorded `SUCCEEDED` may appear.
  - **`malformed/*`.** The shim mutates a valid signed packet into each malformed form. The case passes when the worker emits nothing for that job and the next barrier job still succeeds (internal/tck/refadapter_test.go:309-319).
- **Non-vacuity.** The harness's own tests run the shim against a bare cap-sdk-python `Agent` with no NEW-1 fencing, and assert that runs D and R report FAIL. Today such a worker drops redelivery without replay (sdk/python/cap/runtime.py:1054-1055), and its event binding checks neither assigned worker nor attempt (sdk/python/cap/production_events.py:99-123).

### 1.3 Installed artifacts and the harness command

The harness follows the installed-artifact rule (spec/tck/MATRIX-STATUS.md:45-53) and the existing builder (tools/tck/matrix/build_artifacts.py:82-98):

1. Run `python -m build --wheel` for `sdk/python`, `adapters/langgraph` and `adapters/crewai` into a temp directory outside the repo. Run `go build` for `cap-reference-scheduler`.
2. Create one fresh venv per framework. Install the adapter's hashed lock, then the two local wheels with `--no-index --find-links <wheeldir>`. Fail when `importlib.metadata` resolves either distribution from outside that venv.
3. Start an in-process TLS nats-server on loopback with JetStream, using a per-run CA. nats-server v2 is already a dependency (go.mod:8). `tck.StartEmbeddedNATS` cannot be reused because it has no TLS (internal/tck/natsserver.go:37-45). Then start `cap-reference-scheduler`, and then the adapter's console script with the cwd outside the repo and `PYTHONPATH` set to the network-trap directory only (section 3.7).
4. Run `cap-tck run` five times (section 1.1), with `--adapter` set to the shim argv (cmd/cap-tck/adapterspec.go:10-26).

The harness command for task-dbed002a (addendum item 6):

```
go run ./cmd/cap-framework-tck run --framework langgraph --python <python3.10+> --out <dir>
go run ./cmd/cap-framework-tck run --framework crewai    --python <python3.10-3.13> --out <dir>
```

- **Outputs.** It writes `<dir>/<framework>/{lifecycle,cancellation,malformed,duplicates,retries}.json` plus `summary.json`.
- **Exit 0** only when all five reports are conformant AND the applicable PASS ids are exactly the 28 ids in section 1.1. The harness checks that set twice: against the shipped suite files read at run time, and against a list compiled into the harness. A removed or renamed case therefore fails the run instead of shrinking it.
- **Exit 1** means not conformant.
- **Exit 2** means a usage error or a missing wheel, dependency, toolchain or case. The codes follow cap-tck: 2 for usage or input errors, 1 when not conformant (cmd/cap-tck/run.go:45-84).

## 2. Reference scheduler scope

`internal/referencescheduler` (Apache-2.0, no Cordum import) is a single-process, single-replica control plane with in-memory state.

**Explicit exclusions:**
- HA, replicas and shared stores. The scheduler subscribes to the trust subjects without a queue group and runs as exactly one process, so the shared-store requirement for queue-group replicas (spec/09-transport-profile.md:53-65) does not arise. Running two replicas is unsupported (section 7).
- JetStream durability for jobs.
- Workflows, compensation and DLQ.
- Kafka.
- `sys.heartbeat`, legacy `sys.handshake`, `sys.alert` and `sys.workflow.event`.
- `meta.idempotency_key` short-circuiting. `job_id` is the idempotency key.
- A policy engine. The only policy is the static allowlist in section 2.3.

### 2.1 States and transitions

| transition | spec/07 line | owner in the reference scheduler |
|---|---|---|
| PENDING→SCHEDULED | spec/07-state-machine.md:22 | `Submit` (atomic with admission; PENDING is never stored) |
| SCHEDULED→DENIED | spec/07-state-machine.md:31 | `Submit` (allowlist miss, oversize input) |
| SCHEDULED→DISPATCHED | spec/07-state-machine.md:23 | `Dispatch` |
| DISPATCHED→RUNNING | spec/07-state-machine.md:24 | `ApplyProgress` (worker-owned, spec/07-state-machine.md:43) |
| RUNNING→SUCCEEDED/FAILED/FAILED_RETRYABLE/FAILED_FATAL | spec/07-state-machine.md:25-28 | `ApplyResult` (worker-owned) |
| RUNNING/DISPATCHED→TIMEOUT | spec/07-state-machine.md:29-30 | `Tick` (reconciler role, spec/07-state-machine.md:44) |
| any non-terminal→CANCELLED | spec/07-state-machine.md:32, spec/07-state-machine.md:50 | `Cancel` |

**Rules:**
- A repeated identical state is accepted (spec/07-state-machine.md:47).
- A backwards transition is rejected (spec/07-state-machine.md:48).
- A conflicting terminal is ignored, and the first terminal wins (spec/07-state-machine.md:49).
- `FAILED_RETRYABLE` returns the job to SCHEDULED while `attempt < MaxAttempts` (spec/07-state-machine.md:52). Once attempts are exhausted it becomes terminal `FAILED` and keeps the worker's error codes.

### 2.2 Subjects

| subject | payload | direction | source |
|---|---|---|---|
| `sys.job.submit` | JobRequest | client → scheduler | spec/09-transport-profile.md:10 |
| `sys.job.result` | JobResult | worker → scheduler | spec/09-transport-profile.md:11 |
| `sys.job.progress` | JobProgress | worker → scheduler | spec/09-transport-profile.md:14 |
| `sys.job.cancel` | JobCancel (fenced to the active dispatch) | scheduler → worker | spec/09-transport-profile.md:15 |
| `sys.worker.handshake.challenge`, `sys.worker.handshake.authenticate` | handshake messages, core request/reply | worker → scheduler | spec/09-transport-profile.md:19-20 |
| `worker.<assigned_worker_id>.jobs` | JobRequest | scheduler → one worker | subscribed by cap-sdk-python (sdk/python/cap/runtime.py:620-634) |
| `refsched.job.cancel` | JobCancel | client → scheduler | application subject |
| `refsched.job.terminal` | JobResult (scheduler-signed, the decided winner) | scheduler → clients | application subject |

- **Application subjects, not a wire change.** Only `sys.*` subjects are protocol-defined (spec/09-transport-profile.md:23-24). The two `refsched.*` subjects reuse existing messages, so they are not a wire change. `refsched.job.cancel` is the separate authenticated cancel operation that spec/19-cap-production-profile.md:111-115 calls for.
- **Why dispatch uses the direct subject.** Dispatch goes to the direct subject, not to a `job.<pool>` queue group. `DispatchIdentity` names one assigned worker (proto/cordum/agent/v1/job.proto:25-29), and a queue group could hand the job to a member that must then reject it.
- **Worker selection.** A worker can take topic T when it holds a live session from this scheduler and the operator config lists it for T. Selection is round-robin.

### 2.3 Admission before any mutation

The runtime (task-3d1b1a20) handles every inbound message on actual subject S in this order, following spec/19-cap-production-profile.md:9-24:

1. **Size bound and decode.** Bound the size (the NATS max payload plus the config limit), then decode the BusPacket.
2. **Trust input.** Build it locally:
   - `Audience = S`, the actual subject and never a packet field.
   - Tenant and sender come from the authenticated worker session, or from local client config for submitters (spec/19-cap-production-profile.md:96-102).
3. **Verify.** Call `capsdk.VerifyTrustedProductionPacket` (sdk/go/production_signing.go:92-104), which yields a `capsdk.VerifiedProductionPacket` (sdk/go/production_verified.go:8-13).
4. **Replay.** Call `ReplayStore.Admit` (sdk/go/production_replay.go:18-26; in-memory store at sdk/go/production_replay.go:40-49). A duplicate is a no-op.
5. **State core.** Call the state-core method.

A failure at steps 1-4 causes a drop, a counter increment and one bounded, payload-free log line. There is no state change and no publish.

**Policy.** A static allowlist of `{TenantID, Topic, Capability}` rules from local config, matched exactly. A miss is recorded as `DENIED`. An empty allowlist is a config error, so the scheduler fails closed (spec/19-cap-production-profile.md:117-126).

### 2.4 ResourceRefs

The scheduler never needs content, so neither the state core nor the runtime fetches.

- **The core.** It requires `JobRequest.context_ref` (proto/cordum/agent/v1/job.proto:178) and bounds the declared `size_bytes` (proto/cordum/agent/v1/job.proto:33-41) of the input, the result and the artifact count.
- **Resolution happens at the endpoints**, through `capsdk.NewResourceRegistry` (sdk/go/production_resource_registry.go:141-159) with a `TrustedResourceContext` built from verified identity (sdk/go/production_resource_registry.go:38-43):
  - Workers resolve the input, through the cap-sdk-python registry in NEW-2.
  - `cap-reference-submit` externalizes inputs and resolves outputs (sdk/go/production_resource_externalize.go:19-27, sdk/go/production_resource_resolve.go:17-21).
- **Backend.** A content-addressed JetStream KV store on the same NATS. The URI is `nats-kv://<bucket>/<tenant>/<sha256hex>` and the resolver id is `nats-kv`. It is delivered as `internal/resourcekv` for Go and as `cap.resource_nats_kv` for Python, both in NEW-2. The resolver enforces the spec/19-cap-production-profile.md:137-149 checks: tenant equal to the trusted tenant, digest, size, media type and expiry.

### 2.5 State-core API (held-out test contract, addendum item 4)

task-9b22cabe writes `state.go` from this section alone. Every body returns `ErrNotImplemented`. The acceptance file starts with `//go:build refsched_acceptance`, and task-046d1ce3 removes only that line plus the blank line after it. The acceptance tests never reference `ErrNotImplemented`, and task-046d1ce3 deletes it.

```go
package referencescheduler

import (
	"errors"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
)

const (
	SubjectSubmit       = "sys.job.submit"
	SubjectResult       = "sys.job.result"
	SubjectProgress     = "sys.job.progress"
	SubjectCancel       = "sys.job.cancel"
	SubjectClientCancel = "refsched.job.cancel"
	SubjectTerminal     = "refsched.job.terminal"
)

// WorkerSubject returns "worker." + workerID + ".jobs".
func WorkerSubject(workerID string) string

type Limits struct {
	MaxInputBytes   uint64        // bound on context_ref.size_bytes
	MaxOutputBytes  uint64        // bound on result_ref.size_bytes
	MaxArtifacts    int           // bound on accepted artifact_refs per job
	MaxAttempts     uint64        // >= 1; attempts are numbered from 1
	DispatchTimeout time.Duration // DISPATCHED with no admitted progress/result -> TIMEOUT
	RunTimeout      time.Duration // run deadline when budget.deadline_ms == 0
	MaxRunTimeout   time.Duration // upper bound applied to budget.deadline_ms
}

type AllowRule struct{ TenantID, Topic, Capability string }

type Config struct {
	Limits        Limits
	Allow         []AllowRule
	Now           func() time.Time       // required; the injected clock
	NewDispatchID func() (string, error) // nil: 128-bit crypto/rand hex
}

// Job is a deep-copied snapshot; mutating it never changes scheduler state.
type Job struct {
	TenantID, JobID, Topic, Capability string
	Status    agentv1.JobStatus
	Dispatch  *agentv1.DispatchIdentity // nil until the first Dispatch
	Acked     bool                      // AckDispatch seen for Dispatch
	Result    *agentv1.JobResult        // the terminal winner; nil before terminal
	Artifacts []*agentv1.ResourceRef
	Deadline  time.Time // zero until dispatched
	DispatchedAt time.Time // zero until dispatched; DispatchTimeout counts from here
	UpdatedAt    time.Time
}

type Dispatch struct {
	TenantID, JobID string
	Subject         string                    // WorkerSubject(Identity.AssignedWorkerId)
	Identity        *agentv1.DispatchIdentity // persisted before Dispatch returns
	Request         *agentv1.JobRequest       // admitted request with .Dispatch = Identity
}

func New(cfg Config) (*Scheduler, error)
func (s *Scheduler) Submit(v capsdk.VerifiedProductionPacket) (Job, error)
func (s *Scheduler) Dispatch(tenantID, jobID, workerID string) (Dispatch, error)
func (s *Scheduler) AckDispatch(d Dispatch) error
func (s *Scheduler) ApplyProgress(v capsdk.VerifiedProductionPacket) (Job, error)
func (s *Scheduler) ApplyResult(v capsdk.VerifiedProductionPacket) (Job, error)
func (s *Scheduler) Cancel(v capsdk.VerifiedProductionPacket) (Job, error)
func (s *Scheduler) Tick() []Job // jobs whose state Tick changed, sorted by (TenantID, JobID)
func (s *Scheduler) Get(tenantID, jobID string) (Job, error)

var (
	ErrInvalidConfig     = errors.New("referencescheduler: invalid config")
	ErrUnverified        = errors.New("referencescheduler: packet not verified")
	ErrWrongSubject      = errors.New("referencescheduler: wrong subject")
	ErrWrongPayload      = errors.New("referencescheduler: wrong payload")
	ErrInvalidJob        = errors.New("referencescheduler: invalid job")
	ErrDenied            = errors.New("referencescheduler: denied")
	ErrJobNotFound       = errors.New("referencescheduler: job not found")
	ErrJobConflict       = errors.New("referencescheduler: job conflict")
	ErrInvalidTransition = errors.New("referencescheduler: invalid transition")
	ErrTerminal          = errors.New("referencescheduler: job is terminal")
	ErrStaleAttempt      = errors.New("referencescheduler: stale or future attempt")
	ErrWrongWorker       = errors.New("referencescheduler: wrong worker")
	ErrInputTooLarge     = errors.New("referencescheduler: input too large")
	ErrOutputTooLarge    = errors.New("referencescheduler: output too large")
	ErrTooManyArtifacts  = errors.New("referencescheduler: too many artifacts")
	ErrNotImplemented    = errors.New("referencescheduler: not implemented") // stub only
)
```

**Behaviour.** Errors are returned bare, so `errors.Is` matches, and are never wrapped with data. No error text carries a tenant, subject, payload, pointer or signature byte.

- **`New`** returns `ErrInvalidConfig` when any of these holds:
  - `Now` is nil;
  - `Allow` is empty;
  - `MaxAttempts` is 0;
  - any `Limits` field is 0 or negative.
- **All four packet methods** run these checks first, with no state change on failure:
  - `!v.IsVerified()` (sdk/go/production_verified.go:39) → `ErrUnverified`.
  - `v.Subject()` (sdk/go/production_verified.go:52) differs from the method's subject → `ErrWrongSubject`. The subjects are `SubjectSubmit`, `SubjectProgress`, `SubjectResult` and `SubjectClientCancel`.
  - A payload other than `job_request`, `job_progress`, `job_result` or `job_cancel` respectively (proto/cordum/agent/v1/buspacket.proto:32-43) → `ErrWrongPayload`.
- **Job key.** Jobs are keyed by (`v.Tenant()`, `job_id`) (sdk/go/production_verified.go:60). Another tenant's job is `ErrJobNotFound`, never `ErrJobConflict`, so the response gives no existence oracle.
- **`Submit`**
  - Records nothing, and returns `ErrInvalidJob`, for an empty or oversized `job_id` or `topic`, a missing `context_ref`, or a non-empty tenant mirror that differs from `v.Tenant()` (spec/19-cap-production-profile.md:96-102).
  - Records `DENIED` and returns that snapshot together with `ErrDenied` on an allowlist miss.
  - Records `DENIED` and returns that snapshot together with `ErrInputTooLarge` when `context_ref.size_bytes > MaxInputBytes`.
  - Otherwise stores `SCHEDULED`.
  - A repeat with a `proto.Equal` request returns the existing job and nil. A different request under the same key returns `ErrJobConflict` and changes nothing.
- **`Dispatch`**
  - Requires `SCHEDULED`. A terminal job returns `ErrTerminal`; any other state returns `ErrInvalidTransition`. An empty `workerID` returns `ErrInvalidJob`.
  - Atomically persists a new `dispatch_id`, `attempt = previous+1` (first attempt 1) and the assigned worker, as spec/19-cap-production-profile.md:104-109 requires. It also sets `Deadline = Now + min(budget.deadline_ms or RunTimeout, MaxRunTimeout)` (proto/cordum/agent/v1/job.proto:80).
  - Only then returns the `Dispatch`, which the runtime publishes.
- **`AckDispatch`**
  - Sets `Acked` when `d.Identity` equals the current identity.
  - Is a no-op returning nil when the job has already moved past DISPATCHED. A result that arrives before the ack is therefore kept.
  - Returns `ErrStaleAttempt` for any other identity and `ErrJobNotFound` for an unknown job.
- **`ApplyProgress` and `ApplyResult`**
  - The payload's `dispatch` must equal the current identity. A different `dispatch_id` or `attempt` returns `ErrStaleAttempt`, which covers both stale and future attempts. `v.Sender()` (sdk/go/production_verified.go:68) or `assigned_worker_id` not matching the assigned worker returns `ErrWrongWorker`. These checks follow spec/19-cap-production-profile.md:111-115.
  - Progress with status RUNNING, or with no status, moves DISPATCHED→RUNNING; RUNNING→RUNNING is accepted. Progress carrying a terminal status returns `ErrInvalidTransition`.
  - `artifact_refs` are appended only before terminal. Past `MaxArtifacts` the call returns `ErrTooManyArtifacts` and nothing is appended.
  - A result with `result_ref.size_bytes > MaxOutputBytes` becomes terminal `FAILED` with `ERROR_CODE_JOB_RESOURCE_EXHAUSTED`, and the call returns that snapshot plus `ErrOutputTooLarge`.
  - On a terminal job, a `proto.Equal` repeat of the winning result from the same dispatch returns nil. Anything else returns the winner's snapshot plus `ErrTerminal`.
  - A worker-reported `CANCELLED` is accepted only when the job is already `CANCELLED`.
- **`Cancel`**
  - The `JobCancel` names the job by `job_id`. The lookup uses `v.Tenant()`, so only the owning tenant can cancel.
  - Any non-terminal job becomes `CANCELLED`, and the call returns it. The runtime then sends a fenced `sys.job.cancel` to the active dispatch.
  - On a terminal job the call returns the winner's snapshot plus `ErrTerminal`.
  - When N goroutines race `Cancel` against `ApplyResult`, exactly one terminal wins.
- **`Tick`**
  - Moves DISPATCHED jobs with `Now - DispatchedAt >= DispatchTimeout`, and DISPATCHED or RUNNING jobs with `Now >= Deadline`, to `TIMEOUT`.
  - Uses only `Config.Now`, so tests advance a fake clock and never sleep.

The core holds no `ResourceRegistry`, no network client and no goroutines. Its locks are never held across callbacks.

## 3. Adapter contract

### 3.1 Package layout (addendum item 3)

Decision: **two self-contained packages and no shared adapter core.**
- `adapters/langgraph`: distribution `cap-adapter-langgraph`, import `cap_adapter_langgraph`.
- `adapters/crewai`: distribution `cap-adapter-crewai`, import `cap_adapter_crewai`.

Why no shared core:
- The shared surface is small: config loading and the error table.
- A third distribution would add a version-skew axis and its own CI and wheel-smoke obligations, and both packages already depend on cap-sdk-python.
- The duplicated test-only network trap lives once, at `testdata/adapters/nettrap/sitecustomize.py`, and is never shipped.

The names follow the `cap-sdk-*` pattern (sdk/python/pyproject.toml:6, sdk/node/package.json:2). Each package:
- is Apache-2.0 with exact framework pins;
- uses a `requirements.lock` installed with `pip --require-hashes`;
- ships a console script, `cap-adapter-langgraph` or `cap-adapter-crewai`.

### 3.2 Operator registry config

Config is local JSON, read once at startup. JSON keeps the stdlib floor at 3.10, where `tomllib` is absent. Unknown keys are rejected.

```json
{
  "schemaVersion": "cap-adapter-config/v1",
  "topic": "job.adapter.langgraph",
  "workerId": "wrk-lg-1",
  "nats": {"url": "tls://127.0.0.1:4222", "caFile": "...", "certFile": "...", "keyFile": "..."},
  "trustFile": "...",
  "limits": {"maxInputBytes": 65536, "maxOutputBytes": 65536, "maxArtifacts": 8,
             "maxArtifactBytes": 65536, "maxDeadlineMs": 30000},
  "workloads": {
    "two-step-upper": {"factory": "operator_graphs:build_upper", "inputModel": "operator_graphs:UpperInput"}
  }
}
```

**Workload selection**
- One registered adapter topic serves every workload.
- A job names its workload with `JobMetadata.capability` (proto/cordum/agent/v1/job.proto:130). That is a symbolic key looked up in `workloads`, and it never names a module or path.
- Every `factory` and `inputModel` is imported at startup, so a bad config fails before connecting. The CrewAI parent never imports `crewai` or operator modules itself: it runs one child in check mode at startup, and that child imports them. Per job, the child validates the input model before kickoff.
- A capability that is not registered fails with `JOB_NOT_FOUND`, without calling a factory or the framework (testdata/adapters/framework-golden-v1.json:100).
- Using one topic avoids cap-sdk-python's silent drop of unknown direct-subject topics (sdk/python/cap/runtime.py:1140-1142).

**Fields the adapter never reads as code, paths or environment.** `JobRequest.env`, `labels` and `adapter_id` (proto/cordum/agent/v1/job.proto:163-164, proto/cordum/agent/v1/job.proto:173) are never used to select code, paths or environment.

**CrewAI additions.** CrewAI workloads add `"artifactTasks": ["<task name>"]` and a `"child"` block: `{"graceMs": 2000, "maxStdoutBytes": 1048576, "maxStderrBytes": 65536}`.

**Agent construction.** The adapter builds `cap.Agent` (sdk/python/cap/runtime.py:338) with:
- `retries=0` (sdk/python/cap/runtime.py:357);
- `max_context_bytes` and `max_result_bytes` taken from `limits` (sdk/python/cap/runtime.py:360-361);
- a TLS `connect_fn` (sdk/python/cap/runtime.py:362), passed as `functools.partial(nats.connect, tls=ssl_ctx)`;
- `production_trust` and `replay_store` (sdk/python/cap/runtime.py:369-370).

Worker trust stays at ENFORCE, as cap-sdk-python requires (sdk/python/cap/runtime.py:1094-1099).

### 3.3 Input, output, progress and artifacts

The input is the bytes resolved from `context_ref` (NEW-2). They are bound-checked, parsed as JSON and validated with the workload's pydantic `inputModel` before any framework call. A failure gives `JOB_INVALID_INPUT` (testdata/adapters/framework-golden-v1.json:89).

| | LangGraph (`langgraph==1.2.18`) | CrewAI (`crewai==1.15.30`) |
|---|---|---|
| build | `factory()` returns a compiled `StateGraph` (langgraph==1.2.18: langgraph/graph/state.py:131, langgraph/graph/state.py:1177) | `factory()` returns a `Crew` (crewai==1.15.30: crewai/crew.py:997 `kickoff`) |
| run | `astream(input, stream_mode=["updates","custom"])` (langgraph==1.2.18: langgraph/pregel/main.py:3115) | `kickoff(inputs=...)` in the child process (section 3.6) |
| progress | each `updates` event yields marker = node name | `task_callback` (crewai==1.15.30: crewai/crew.py:305) yields marker = `Task.name` (crewai==1.15.30: crewai/task.py:150) |
| artifact | a node calls `get_stream_writer()` (langgraph==1.2.18: langgraph/config.py:126) with `{"cap.artifact": {"name","mediaType","content"}}` | output of a task listed in `artifactTasks` |
| output | final state, JSON-encoded | `CrewOutput.json_dict` (crewai==1.15.30: crewai/crews/crew_output.py:21), else `{"raw": CrewOutput.raw}` (crewai==1.15.30: crewai/crews/crew_output.py:17) |

- **Progress** is published in order before the result (testdata/adapters/framework-golden-v1.json:17).
- **Artifacts** are capped by `maxArtifacts` and `maxArtifactBytes`, externalized through the registry and attached as `artifact_refs` before the terminal result (testdata/adapters/framework-golden-v1.json:28; proto/cordum/agent/v1/job.proto:195).
- **Output** larger than `maxOutputBytes` fails with `JOB_RESOURCE_EXHAUSTED` (testdata/adapters/framework-golden-v1.json:77).

cap-sdk-python cannot do any of this today, and NEW-1 adds it:
- its `Context` has no progress, artifact or cancel hook (sdk/python/cap/runtime.py:299-313);
- a SUCCEEDED result carries no artifacts (sdk/python/cap/runtime.py:1283-1289);
- `emit_progress` is unsealed (sdk/python/cap/progress.py:75-87).

### 3.4 Typed error taxonomy (spec/13)

`error_message` is a fixed text per row and never exception text, as in cap-sdk-python's existing generic message (sdk/python/cap/runtime.py:61). When the enum is UNSPECIFIED, the string is authoritative (spec/13-error-codes.md:84). For timeouts, both the enum and the string are set (spec/13-error-codes.md:77-94).

| condition | status | `error_code_enum` (spec/13-error-codes.md:36-46) | `error_code` |
|---|---|---|---|
| unregistered capability | FAILED | `ERROR_CODE_JOB_NOT_FOUND` | — |
| oversize input or output | FAILED | `ERROR_CODE_JOB_RESOURCE_EXHAUSTED` | — |
| input fails `inputModel` or is not JSON | FAILED | `ERROR_CODE_JOB_INVALID_INPUT` | — |
| deadline reached | FAILED | `ERROR_CODE_JOB_TIMEOUT` | `"TIMEOUT"` (spec/13-error-codes.md:90) |
| framework raised | FAILED | `ERROR_CODE_UNSPECIFIED` | `"FRAMEWORK_ERROR"` |
| CrewAI child crashed or broke the IPC bounds | FAILED | `ERROR_CODE_UNSPECIFIED` | `"CHILD_CRASHED"` |
| cancel honoured | CANCELLED | — | — |
| operator code raised `cap.JobFailure(retryable=True/False)` (NEW-1) | FAILED_RETRYABLE or FAILED_FATAL | operator-chosen job or application code (spec/13-error-codes.md:15-24) | operator-chosen |

The worker reports a deadline as `FAILED` + `JOB_TIMEOUT`, not as status `TIMEOUT`, because `TIMEOUT` belongs to the control plane (spec/07-state-machine.md:43-44). This matches the golden case at testdata/adapters/framework-golden-v1.json:64. The `framework-failure-redacted` golden case asserts that no exception text leaks (testdata/adapters/framework-golden-v1.json:40).

### 3.5 Cancellation and deadlines

- **Deadline.** It is `min(budget.deadline_ms, limits.maxDeadlineMs)` (proto/cordum/agent/v1/job.proto:80), and `maxDeadlineMs` applies when `deadline_ms` is 0. It is measured from handler start.
- **Cancel.** A fenced `sys.job.cancel` for the active attempt sets the job's cancel event (NEW-1). The bounds below assume a 2 s grace.
- **LangGraph** runs the graph as an asyncio task, wrapped in `asyncio.wait_for`, and cancels the task on a cancel event.
  - Async nodes stop at the next await.
  - A synchronous node runs in LangGraph's executor thread and cannot be killed in-process. The adapter still reports `CANCELLED` or `TIMEOUT` at the bound, but the thread finishes its current node. This is the ceiling for task-45d29cb0's "no task or thread is left running" requirement: it holds for async nodes only. Operators who need hard kill should use process isolation, as CrewAI does.
- **CrewAI** gets hard kill of the whole process tree (section 3.6).

### 3.6 CrewAI child-process model

One fresh child per job.

**Launch**
- argv: `[sys.executable, "-m", "cap_adapter_crewai._child"]`. There is no shell and no `-I`, so `PYTHONPATH` reaches operator modules and the test trap.
- The child's env is an allowlist:
  - `PATH`, `SYSTEMROOT`, `PYTHONPATH`;
  - `TMP`, `TEMP` and `TMPDIR`, all set to the job's temp dir;
  - `XDG_DATA_HOME` (POSIX);
  - `APPDATA`, `LOCALAPPDATA` and `USERPROFILE` (Windows). Overriding these breaks `import crewai` through pywin32's `SHGetFolderPath`;
  - the forced switches from section 4.2.
- stdin is a pipe, so there is no TTY and no consent prompt (crewai==1.15.30: crewai/events/listeners/tracing/utils.py:531-547). It carries exactly one JSON job line, which is written only after tree containment is in place.

**IPC**
- The child writes JSONL events to stdout: `progress`, `artifact`, `result` and `error`.
- Each line is capped at 1 MiB, the same bound as adapter-v1 (spec/tck/adapter-v1.md:24-25), and the total is capped at `maxStdoutBytes`. Exceeding either cap kills the tree and yields `CHILD_CRASHED`.
- stderr is captured up to 64 KiB. It is never parsed or published, the same rule as adapter-v1 (spec/tck/adapter-v1.md:22-23).

**Process-tree termination**

| OS | containment | kill |
|---|---|---|
| Linux, macOS | `start_new_session=True` | `os.killpg(pgid, SIGTERM)`, then after `graceMs` `os.killpg(pgid, SIGKILL)` |
| Windows | `CREATE_NEW_PROCESS_GROUP` plus a ctypes Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, assigned before the job line is written | `TerminateJobObject`. If job-object setup fails, the child is killed and the job fails closed (`CHILD_CRASHED`). There is no `taskkill` fallback. |

Kill triggers are the deadline, a cancel, an IPC-bound violation and adapter shutdown.

**Cleanup.** A `finally` block reaps the child, then runs `shutil.rmtree` on the temp dir on every exit path. The order is fixed: tree dead first, then rmtree.

### 3.7 Network trap

The trap is test-only and never shipped: `testdata/adapters/nettrap/sitecustomize.py`, loaded through `PYTHONPATH` in both the parent and the CrewAI child.

- **Patched calls:** `socket.socket.connect`, `socket.socket.connect_ex`, `socket.create_connection` and `socket.getaddrinfo`.
- **Allowed:** loopback addresses, `localhost` names and AF_UNIX sockets.
- **Everything else** is refused and recorded. The process then exits with status 97 at exit.

This is the trap that produced the section 4 records. Its ceilings are listed in section 7.

### 3.8 Deterministic no-LLM paths (addendum item 2)

**LangGraph.** Pure-Python nodes on `StateGraph` with `START` and `END`, built with `add_edge` (langgraph==1.2.18: langgraph/graph/state.py:928).

**CrewAI decision: a custom `BaseLLM` double, not a Flow.** A Flow bypasses `Crew`, `Agent` and `Task`, so it would not exercise the `task_callback` and artifact mapping that operators rely on. The double:
- subclasses `crewai.BaseLLM` (crewai==1.15.30: crewai/__init__.py:21; crewai==1.15.30: crewai/llms/base_llm.py:163);
- implements the abstract `call` (crewai==1.15.30: crewai/llms/base_llm.py:462-463) as `call(self, messages, tools=None, callbacks=None, available_functions=None, from_task=None, from_agent=None, response_model=None)` returning `"Final Answer: <scripted>"`;
- implements `supports_function_calling()` returning `False`;
- is built with `ScriptedLLM(model="scripted-double")`.

Crews use `Process.sequential`, set `tracing=False` (crewai==1.15.30: crewai/crew.py:414) and use `Task.output_pydantic` (crewai==1.15.30: crewai/task.py:185) where structured output is needed. The double lives in the tests only. The Flow path was also recorded trap-clean (section 4.3) but is not used.

## 4. Version pins (addendum item 1)

### 4.1 Pins

The note was recorded on 2026-11-11, so a pin must have been uploaded on or before 2026-10-28 (at least 14 days earlier). Both pins below were uploaded before that date.

| framework | pin | uploaded (PyPI) | license | Python | resolved companions |
|---|---|---|---|---|---|
| LangGraph | `langgraph==1.2.18` | 2026-10-26T20:04:29Z | MIT | `>=3.10` | `langchain-core 1.7.7`, `langsmith 0.14.17` |
| CrewAI | `crewai==1.15.30` | 2026-10-24T01:56:42Z | MIT, stated only in the README; the sdist has no LICENSE file and no license metadata (**license gap**) | `>=3.10,<3.14` | `crewai-core 1.15.30`, `opentelemetry-sdk 1.45.2` |

- **Newer releases.** Newer releases exist (`langgraph 1.2.19` on 2026-10-30 and `crewai 1.15.31` on 2026-10-27). They were not pinned because they were inside, or at the edge of, the 14-day window when recorded. A slice may bump to a release that is at least 14 days old only if it re-runs section 4.3.
- **Python floor.** The adapters' floor is 3.10. cap-sdk-python itself supports `>=3.9` (sdk/python/pyproject.toml:10).
- **CrewAI license gap.** task-c82d5007 must record the README license evidence in its package README and recheck it on any bump.

### 4.2 Phone-home switches, applied before import

**LangGraph** (`os.environ` is set before `import langgraph`):
- `LANGSMITH_TRACING_V2`, `LANGSMITH_TRACING`, `LANGCHAIN_TRACING_V2` and `LANGCHAIN_TRACING` all set to `"false"`. langsmith reads these env vars (langsmith==0.14.17: langsmith/utils.py:119-140), and langchain-core treats `"false"` as unset (langchain-core==1.7.7: langchain_core/utils/env.py:9-23).
- Every run is also wrapped in `langsmith.tracing_context(enabled=False)` (langsmith==0.14.17: langsmith/run_helpers.py:172). This is needed because the context var and the global flag take precedence over env vars (langsmith==0.14.17: langsmith/utils.py:130, langsmith==0.14.17: langsmith/utils.py:136).

**CrewAI** (set in the child env, before import):
- `OTEL_SDK_DISABLED=true`, `CREWAI_DISABLE_TELEMETRY=true` and `CREWAI_DISABLE_TRACKING=true` (crewai-core==1.15.30: crewai_core/telemetry.py:354-359).
- `CREWAI_TRACING_ENABLED=false` (crewai==1.15.30: crewai/events/listeners/tracing/utils.py:144-174, crewai==1.15.30: crewai/events/listeners/tracing/utils.py:168), plus `Crew(tracing=False)`.
- `CREWAI_DISABLE_VERSION_CHECK=true`. Without it, CrewAI calls `urlopen` on pypi.org (crewai-core==1.15.30: crewai_core/version.py:112-126). The skip conditions are at crewai==1.15.30: crewai/events/utils/console_formatter.py:60-78; `CI=true` also skips the check, which hides a missing switch in CI (section 7).

The adapters force these values and ignore any operator value that would turn them on.

### 4.3 Recorded trap results

Recorded on Windows 11 with CPython 3.12.13 and uv venvs, on 2026-11-11. Each script ran as `PYTHONPATH=<trap> python -s <script>`.

| run | switches | result | non-loopback attempts |
|---|---|---|---|
| `import langgraph.graph` | on | imported | 0 |
| LangGraph two-node graph (`upper`, `suffix`), `stream` + `invoke` | on | `{'text': 'HELLO!', 'steps': ['upper', 'suffix']}` | 0 |
| same graph, default env | none set | same | 0 |
| same graph, tracing env on (negative control) | off | same | **16** × `getaddrinfo('api.smith.langchain.com')`, exit 97 |
| `import crewai` | on / off | `crewai 1.15.30` | 0 / 0 |
| Crew + `BaseLLM` double, one task | on | `'SCRIPTED-OK'` | 0 |
| same crew (negative control) | off | `'SCRIPTED-OK'` | **8** × `getaddrinfo('telemetry.crewai.com')`, exit 97 |
| two-step Flow | on | `'HELLO!'` | 0 |
| two-step Flow (negative control) | off | `'HELLO!'` | **10**: 8 × `telemetry.crewai.com`, 2 × `create_connection(('pypi.org', 443))` |
| two-step Flow without the version switch | partly on | `'HELLO!'` | **2** × `pypi.org:443` |

Import alone makes no attempt. Every attempt comes at run time, and the switches remove all of them. The adapter tests (task-45d29cb0, task-c82d5007) re-assert 0 under the trap in CI on every declared OS and Python version.

## 5. CI

ci.yml and sdk-gates.yml stay untouched. Each slice adds its own workflow.

| workflow (slice) | runners | Python | runs |
|---|---|---|---|
| `adapter-langgraph.yml` (task-45d29cb0) | `ubuntu-24.04` | 3.10, 3.11, 3.12, 3.13, 3.14 | lock install, pytest under the trap, wheel build + `python -I` smoke outside the repo |
| `adapter-crewai.yml` (task-c82d5007) | `ubuntu-24.04` (3.10-3.13), `macos-latest` and `windows-latest` (3.10, 3.13) | `<3.14` per the pin | same as above, plus the child-lifecycle tests per OS |
| `framework-tck.yml` (task-dbed002a) | `ubuntu-latest` | 3.12 | the section 1.3 command for both frameworks; uploads both report sets; no `continue-on-error`, no skip path |

- **Runners.** `ubuntu-24.04` follows ci.yml (.github/workflows/ci.yml:126). The multi-OS matrix follows .github/workflows/onboarding.yml:14.
- **Matrix rule.** A Python version that cannot install the lock is removed from the matrix in that slice, with the reason stated. It is never skipped at runtime.
- **NEW-1 and NEW-2 tests.** They add their real-NATS cases to `sdk/python/tests/integration/test_worker_nats.py`, the file the existing python-nats job runs (.github/workflows/ci.yml:194). That needs no workflow edit.

**cap-sdk-python install source (addendum item 6): the repo-built wheel.**
- CI builds `sdk/python` with `python -m build --wheel` in the same run and installs it with `--no-index --find-links`, following spec/tck/MATRIX-STATUS.md:52.
- A published version is never used, because the adapters need NEW-1 and NEW-2 code that is unreleased at 2.17.1.dev0 (sdk/python/pyproject.toml:7). The adapters declare `cap-sdk-python>=<version that ships NEW-1>,<3`.

**CAP-owned artifact names (addendum item 5; task-b1c17394).** Readiness must never count these as independent implementations, whatever affiliation an attestation declares:

`cap-reference-scheduler`, `cap-reference-submit`, `cap-adapter-langgraph`, `cap-adapter-crewai`, `cap-tck-reference-adapter`, `cap-framework-tck`, `cap-sdk-go`, `cap-sdk-python`, `cap-sdk-node`

- Today readiness excludes only forks and affiliation `cordum` (internal/governance/readiness.go:123-125, internal/governance/readiness.go:156-173). It counts any complete attestation (internal/governance/readiness.go:55-57), which is why this list is needed.
- The `cap-tck __adapter` reference adapter (cmd/cap-tck/refadapter.go:14-19) is the item named `cap-tck-reference-adapter`.
- governance/readiness.json:3 lists no implementations, so readiness stays BLOCKED.

**No-Cordum check** (task-dbed002a):
- Fail on the Go modules `github.com/cordum-io/cordum` and `github.com/cordum-io/cordum-packs`, matched by module path.
- Fail on any Python distribution whose name starts with `cordum`, for example `cordum-guard`, which examples/framework-crewai/main.py:9 imports. AD3 keeps that example untouched.
- Never match the substring "cordum". CAP's own proto package `github.com/cordum-io/cap/v2/cordum/agent/v1` and the vendored `cap.pb.cordum.agent.v1` are allowed.

## 6. Per-slice file lists (each at most 10 files)

New paths are marked NEW. **FLAG** marks a slice whose scope this note changes.

The dependency order:
- NEW-2 → NEW-1 → 45d29cb0 → c82d5007
- 9b22cabe → 046d1ce3 → 3d1b1a20 → aa883942
- aa883942, 45d29cb0 and c82d5007 → dbed002a
- d5aa5967 → db433d5d → b1c17394
- everything → e511e1c0

| slice | files | count |
|---|---|---|
| **NEW-2** (proposed): cap-sdk-python ResourceRef registry + JetStream KV backend (Go and Python) | `sdk/python/cap/resource_registry.py` NEW, `sdk/python/cap/resource_nats_kv.py` NEW, `sdk/python/cap/runtime.py` (`context_ref` in, `result_ref` out), `sdk/python/cap/__init__.py`, `sdk/python/tests/test_resource_registry.py` NEW, `sdk/python/tests/integration/test_worker_nats.py`, `internal/resourcekv/resourcekv.go` NEW, `internal/resourcekv/resourcekv_test.go` NEW, `CHANGELOG.md` | 9 |
| **NEW-1** (proposed): cap-sdk-python worker outcomes, progress, artifacts, cancel and dispatch fencing | `sdk/python/cap/runtime.py`, `sdk/python/cap/outcome.py` NEW (`JobFailure`), `sdk/python/cap/dispatch_fence.py` NEW (accept attempt = active+1, replay the recorded result on exact redelivery, assigned-worker check), `sdk/python/cap/cancel.py` NEW (fenced `sys.job.cancel`), `sdk/python/cap/__init__.py`, `sdk/python/tests/test_outcome.py` NEW, `sdk/python/tests/test_dispatch_fence.py` NEW, `sdk/python/tests/integration/test_worker_nats.py`, `CHANGELOG.md` | 9 |
| d5aa5967 public report v1 (no change) | `spec/tck/schema/public-report-v1.schema.json` NEW, `internal/tck/publicreport/project.go` NEW, `validate.go` NEW, `project_test.go` NEW, `testdata/` with one source report and one golden per verdict (6) | 10 |
| 9b22cabe state-core held-out tests (API from section 2.5) | `internal/referencescheduler/state.go` NEW (stubs plus every exported type, constant and sentinel in section 2.5), `internal/referencescheduler/state_acceptance_test.go` NEW | 2 |
| **FLAG** 046d1ce3 state-core implementation | `internal/referencescheduler/state.go`, `store.go` NEW, `errors.go` NEW (sentinels may move here from `state.go`; `ErrNotImplemented` deleted), `state_internal_test.go` NEW, `state_acceptance_test.go` (build line only) | 5 |
| **FLAG** 3d1b1a20 NATS runtime | `internal/referencescheduler/runtime.go` NEW, `nats.go` NEW, `config.go` NEW (allowlist, worker list per topic, TLS), `trust.go` NEW (scheduler-side worker-trust handshake issuer), `runtime_test.go` NEW, `nats_e2e_test.go` NEW, `trust_test.go` NEW | 7 |
| **FLAG** aa883942 commands | `cmd/cap-reference-scheduler/main.go` NEW, `main_test.go` NEW, `README.md` NEW, `cmd/cap-reference-submit/main.go` NEW, `main_test.go` NEW, `CHANGELOG.md` | 6 |
| **FLAG** 45d29cb0 LangGraph adapter | `adapters/langgraph/pyproject.toml` NEW, `requirements.lock` NEW, `README.md` NEW, `src/cap_adapter_langgraph/__init__.py` NEW, `tests/golden_graphs.py` NEW, `tests/test_adapter.py` NEW, `testdata/adapters/nettrap/sitecustomize.py` NEW, `.github/workflows/adapter-langgraph.yml` NEW, `CHANGELOG.md` | 9 |
| **FLAG** c82d5007 CrewAI adapter | `adapters/crewai/pyproject.toml` NEW, `requirements.lock` NEW, `README.md` NEW, `src/cap_adapter_crewai/__init__.py` NEW, `src/cap_adapter_crewai/_proc.py` NEW (spawn, IPC bounds, tree kill), `src/cap_adapter_crewai/_child.py` NEW, `tests/golden_crews.py` NEW (the `BaseLLM` double), `tests/test_adapter.py` NEW, `.github/workflows/adapter-crewai.yml` NEW, `CHANGELOG.md` | 10 |
| db433d5d report CLI (no change) | `cmd/cap-tck/report.go` NEW, `report_test.go` NEW, `cmd/cap-tck/main.go`, `internal/tck/publicreport/sign.go` NEW, `badge.go` NEW, `sign_test.go` NEW, `badge_test.go` NEW, `internal/tck/publicreport/testdata/badge-pass.golden.svg` NEW, `CHANGELOG.md` | 9 |
| **FLAG** dbed002a framework TCK harness | `cmd/cap-framework-tck/main.go` NEW, `build.go` NEW (wheels, venvs, no-Cordum closure), `natstls.go` NEW, `shim.go` NEW, `cases.go` NEW, `main_test.go` NEW, `testdata/adapters/tck-probes/langgraph_probes.py` NEW, `testdata/adapters/tck-probes/crewai_probes.py` NEW, `.github/workflows/framework-tck.yml` NEW, `CHANGELOG.md` | 10 |
| **FLAG** b1c17394 readiness | `internal/governance/readiness.go`, `internal/governance/attestation.go` NEW (holds the section 5 CAP-owned list), `internal/governance/readiness_test.go`, `internal/governance/testdata/attestations/pass-report.json` NEW, `internal/governance/testdata/attestations/manifest.json` NEW, `cmd/cap-governance/readiness_cmd.go`, `governance/READINESS.md` (only if the render changes), `CHANGELOG.md` | 8 |
| e511e1c0 capstone (no change) | `README.md`, `docs/adoption.md` NEW, `docs/conformance.md` NEW, `docs/reference-scheduler.md` NEW, `CHANGELOG.md` | 5 |

**Flags**
- **046d1ce3.**
  - The core holds no `ResourceRegistry` (section 2.4). The rule "every ResourceRef goes through the registry" is met by having no fetch path at all.
  - The sentinels already exist in 9b22cabe's stub.
- **3d1b1a20.**
  - It adds the scheduler-side worker-trust issuer. CAP-PRODUCTION in cap-sdk-python requires ENFORCE sessions (sdk/python/cap/runtime.py:1094-1099), and sdk/go ships only worker-side helpers (sdk/go/worker_trust_build.go:30, sdk/go/worker_trust_verify.go:13).
  - It adds the `refsched.*` subjects, direct-subject dispatch and the static allowlist.
  - If the issuer needs unexported sdk/go primitives, the slice stops and calls `report_blocked` (no sdk/go change).
- **aa883942.**
  - `cap-reference-submit` cancels over `refsched.job.cancel` and waits on `refsched.job.terminal`.
  - It externalizes and resolves through `internal/resourcekv` (NEW-2), so it gains a dependency on NEW-2.
- **45d29cb0.**
  - It depends on NEW-1 and NEW-2.
  - The capability-keyed registry (section 3.2) replaces "topic → module:callable".
  - Python is 3.10+.
  - The no-leftover-thread requirement holds for async nodes only (section 3.5).
  - It owns the shared `testdata/adapters/nettrap/sitecustomize.py`.
- **c82d5007.**
  - It depends on NEW-1, NEW-2 and 45d29cb0 (for the trap).
  - The CrewAI license gap applies (section 4.1).
  - Python is `>=3.10,<3.14`.
  - It must use the Windows job-object decision (section 3.6).
  - It sits at the 10-file cap. Any extra file means splitting `_proc.py` and its lifecycle tests into a separate slice.
- **dbed002a.**
  - It is a new command, `cmd/cap-framework-tck`, with the hidden shim.
  - It runs its own TLS nats-server.
  - It produces five reports per framework.
  - It uses probe workloads.
  - It depends on NEW-1 and NEW-2.
- **b1c17394.**
  - The CAP-owned list is the section 5 list, and it includes the SDK and harness names.
  - Negative fixtures are generated in-test from one committed PASS report, which keeps the slice at 10 files or fewer.

## 7. Risks

1. **Python-level trap only.** The trap patches Python sockets, so native-extension sockets would bypass it. The mitigation is an optional Linux-only `unshare -n` re-run. It cannot be mandatory, because Ubuntu 24.04's AppArmor restricts unprivileged user namespaces.
2. **Masked controls.** `CI=true` and a fresh CrewAI version cache both skip the version check (crewai==1.15.30: crewai/events/utils/console_formatter.py:60-78), so a negative control in CI can pass vacuously. Negative-control tests unset `CI` and use an empty storage dir. Note that `CREWAI_STORAGE_DIR` is an app-name subdirectory, not a path (crewai-core==1.15.30: crewai_core/paths.py:13).
3. **LangSmith precedence.** Context vars outrank env vars (section 4.2), so a framework upgrade could re-enable tracing even with the env switches set. The adapter wraps every run in `tracing_context(enabled=False)`, and the tests set tracing env to on to assert that it is forced off.
4. **CrewAI storage location.** On Windows, CrewAI stores data per user under `APPDATA`, which the child cannot override (section 3.6). The child's temp dir does not contain it, so this is documented in the README.
5. **Cooperative cancel ceiling.** LangGraph's synchronous nodes cannot be killed in-process (section 3.5).
6. **cap-sdk-python gaps.** The adapters cannot meet their DoD until NEW-1 and NEW-2 land. The gaps are no fencing, no replay of results (sdk/python/cap/runtime.py:1054-1055), no cancel subscription (sdk/python/cap/runtime.py:620-634), FAILED-only results with exception text dropped (sdk/python/cap/runtime.py:1239-1256), and `redis://`-only pointers (sdk/python/cap/runtime.py:259-271).
7. **Worker-trust issuer.** It is new security-critical code in 3d1b1a20. Challenges must be single-use, and concurrent authentication must have one winner (spec/09-transport-profile.md:53-58, spec/09-transport-profile.md:73-76). Those guarantees hold only for a single process, so the scheduler must refuse to start as a second replica, or the README must say that it is unsupported.
8. **Citation drift.** Line citations are pinned to the base commit above. A slice that rebases re-checks the lines it relies on.
9. **CI timing.** The required aggregate checks queue last, so slices must poll to completion and never read queued as green.
10. **CrewAI license.** The license is MIT in the README only (section 4.1). A future release could change it. Recheck on every bump.

## Addendum index (comment-2358fde2)

| item | question | answered in |
|---|---|---|
| 1 | phone-home switches and network-trap proof | 4.2, 4.3 (enforced by 45d29cb0 and c82d5007, section 5) |
| 2 | deterministic no-LLM path, with pinned CrewAI API names | 3.8 |
| 3 | shared adapter core vs self-contained packages; distribution and import names | 3.1 |
| 4 | held-out tests: build tag and the precise state-core API | 2.5 |
| 5 | CAP-owned artifact names readiness must never count | 5 |
| 6 | exact harness command, worker-role scenario ids, cap-sdk-python install source | 1.3, 1.1, 5 |
