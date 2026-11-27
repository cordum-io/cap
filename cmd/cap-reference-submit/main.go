// Command cap-reference-submit submits one CAP-PRODUCTION job to the Apache-2.0
// CAP reference scheduler (cmd/cap-reference-scheduler) and waits for its
// scheduler-signed terminal result.
//
// It prints one JSON summary of the verified terminal and exits 0 when the job
// SUCCEEDED, 1 for any other terminal status or a failure to submit, 2 for a
// usage or configuration error, and 3 when no verified terminal arrives before
// the deadline. See README.md.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	agentv1 "github.com/cordum-io/cap/v2/cordum/agent/v1"
	"github.com/cordum-io/cap/v2/internal/referencescheduler"
	"github.com/cordum-io/cap/v2/internal/resourcekv"
	capsdk "github.com/cordum-io/cap/v2/sdk/go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	exitSucceeded = 0
	exitFailed    = 1
	exitConfig    = 2
	exitDeadline  = 3
	maxField      = 1024 // bytes kept of each summary string
	inputPurpose  = "job-context"
)

var (
	errUsage        = errors.New("usage: cap-reference-submit -flag value ...; see -h and README.md")
	errRequired     = errors.New("value is required, without surrounding spaces")
	errDeadline     = errors.New("-deadline must be positive")
	errKeyFile      = errors.New("private key file is missing")
	errKeySymlink   = errors.New("private key file is a symlink")
	errKeyMode      = errors.New("private key file is readable or writable by group or others")
	errSigningKey   = errors.New("-signing-key-file is not a P-256 private key")
	errSchedulerKey = errors.New("-scheduler-key-file is not a P-256 public key")
	errInput        = errors.New("-input file is unreadable")
	errTransport    = errors.New("NATS is unavailable")
	errBucket       = errors.New("-kv-bucket is not an available JetStream KV bucket")
)

// required names the flags that must be set, checked in flag order.
var required = map[string]bool{"sender-id": true, "tenant-id": true, "signing-key-id": true, "signing-key-file": true,
	"scheduler-id": true, "scheduler-key-id": true, "scheduler-key-file": true, "topic": true, "capability": true,
	"job-id": true, "input": true, "media-type": true, "kv-bucket": true}

type client struct {
	nats                                          referencescheduler.RuntimeConfig // only NATSURL, TLS* and DevPlaintextLoopback
	senderID, tenantID, signingKeyID, signingFile string
	schedulerID, schedulerKeyID, schedulerFile    string
	jobID, topic, capability, input, mediaType    string
	bucket                                        string
	deadline                                      time.Duration
	tls                                           *tls.Config
	key                                           *ecdsa.PrivateKey
	scheduler                                     *ecdsa.PublicKey
	inputFile                                     *os.File
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run writes the summary to stdout and only fixed messages, sentinel errors
// and ids to stderr, never key material or job content.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	c, err := load(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cap-reference-submit: config error: %v\n", err)
		return exitConfig
	}
	defer c.inputFile.Close()
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	result, err := c.submit(ctx)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		fmt.Fprintf(stderr, "cap-reference-submit: no verified terminal for job %s within %s\n", c.jobID, c.deadline)
		return exitDeadline
	case err != nil:
		fmt.Fprintf(stderr, "cap-reference-submit: %v\n", err)
		return exitFailed
	}
	printSummary(stdout, result)
	if result.GetStatus() == agentv1.JobStatus_JOB_STATUS_SUCCEEDED {
		return exitSucceeded
	}
	return exitFailed
}

// load parses the flags, checks the private key files before anything reads
// them, then loads the keys, the TLS files and the input file.
func load(args []string, stderr io.Writer) (*client, error) {
	c := &client{}
	fs := flag.NewFlagSet("cap-reference-submit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.nats.NATSURL, "nats-url", "", "NATS endpoint, tls://host:port (nats:// only with -dev-plaintext-loopback)")
	fs.StringVar(&c.nats.TLSCAFile, "tls-ca", "", "PEM CA bundle that signs the NATS server certificate")
	fs.StringVar(&c.nats.TLSCertFile, "tls-cert", "", "PEM client certificate for NATS")
	fs.StringVar(&c.nats.TLSKeyFile, "tls-key", "", "PEM private key of the NATS client certificate")
	fs.BoolVar(&c.nats.DevPlaintextLoopback, "dev-plaintext-loopback", false, "allow plaintext NATS to a loopback host (development only)")
	fs.StringVar(&c.senderID, "sender-id", "", "this client's sender id, as in the scheduler's Clients[].SenderID")
	fs.StringVar(&c.tenantID, "tenant-id", "", "the tenant of the job, as in Clients[].TenantID")
	fs.StringVar(&c.signingKeyID, "signing-key-id", "", "key id of this client's production signing key")
	fs.StringVar(&c.signingFile, "signing-key-file", "", "this client's production signing P-256 private key PEM")
	fs.StringVar(&c.schedulerID, "scheduler-id", "", "the scheduler's SchedulerID, the sender of terminal results")
	fs.StringVar(&c.schedulerKeyID, "scheduler-key-id", "", "the scheduler's SigningKeyID")
	fs.StringVar(&c.schedulerFile, "scheduler-key-file", "", "public half of the scheduler's SigningKeyFile, a P-256 PEM")
	fs.StringVar(&c.topic, "topic", "", "job topic")
	fs.StringVar(&c.capability, "capability", "", "job capability, matched by the scheduler's allowlist")
	fs.StringVar(&c.jobID, "job-id", "", "job id (default: a random id)")
	fs.StringVar(&c.input, "input", "", "file holding the job input")
	fs.StringVar(&c.mediaType, "media-type", "application/json", "media type of the input")
	fs.StringVar(&c.bucket, "kv-bucket", "", "JetStream KV bucket of the nats-kv resource backend")
	fs.DurationVar(&c.deadline, "deadline", 30*time.Second, "how long to wait for the terminal result")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return nil, errUsage
	}
	if c.jobID == "" {
		c.jobID = "job-" + rand.Text()
	}
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if v := f.Value.String(); err == nil && required[f.Name] && (v == "" || v != strings.TrimSpace(v)) {
			err = fmt.Errorf("-%s: %w", f.Name, errRequired)
		}
	})
	if err != nil {
		return nil, err
	}
	if c.deadline <= 0 {
		return nil, errDeadline
	}
	for _, f := range [][2]string{{"signing-key-file", c.signingFile}, {"tls-key", c.nats.TLSKeyFile}} {
		if err := checkPrivateKeyFile(f[0], f[1]); err != nil {
			return nil, err
		}
	}
	if c.tls, err = c.nats.ResolveTransport(); err != nil {
		return nil, err
	}
	if c.key = referencescheduler.ReadPrivateKey(c.signingFile); c.key == nil {
		return nil, errSigningKey
	}
	if c.scheduler = referencescheduler.ReadPublicKey(c.schedulerFile); c.scheduler == nil {
		return nil, errSchedulerKey
	}
	if c.inputFile, err = os.Open(c.input); err != nil {
		return nil, errInput
	}
	return c, nil
}

// checkPrivateKeyFile refuses a private key file that is a symlink or, on
// POSIX, one that group or others may read or write, as the scheduler does.
func checkPrivateKeyFile(name, path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return fmt.Errorf("%s: %w", name, errKeyFile)
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("%s: %w", name, errKeySymlink)
	case runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0:
		return fmt.Errorf("%s: %w", name, errKeyMode)
	}
	return nil
}

// submit stores the input, publishes the signed request after subscribing to
// terminals, and returns the first verified terminal of this job.
func (c *client) submit(ctx context.Context) (*agentv1.JobResult, error) {
	opts := []nats.Option{nats.Name("cap-reference-submit"), nats.Timeout(2 * time.Second), nats.IgnoreDiscoveredServers()}
	if c.tls != nil {
		opts = append(opts, nats.Secure(c.tls))
	}
	nc, err := nats.Connect(c.nats.NATSURL, opts...)
	if err != nil {
		return nil, errTransport
	}
	defer nc.Close()
	ref, err := c.externalize(ctx, nc)
	if err != nil {
		return nil, err
	}
	sub, err := nc.SubscribeSync(referencescheduler.SubjectTerminal)
	if err != nil {
		return nil, errTransport
	}
	raw, err := c.request(ref)
	if err != nil {
		return nil, err
	}
	if err := nc.Publish(referencescheduler.SubjectSubmit, raw); err != nil {
		return nil, errTransport
	}
	trust := capsdk.ProductionTrustStore{Audience: referencescheduler.SubjectTerminal, Tenant: c.tenantID, Sender: c.schedulerID,
		PublicKeys: map[string]*ecdsa.PublicKey{c.schedulerKeyID: c.scheduler}}
	for {
		m, err := sub.NextMsgWithContext(ctx)
		if err != nil {
			return nil, err
		}
		// Terminals of other jobs, tenants or signers are not ours: skip them.
		if v, err := capsdk.VerifyTrustedProductionPacket(m.Data, trust); err == nil {
			if r := v.Packet().GetJobResult(); r.GetJobId() == c.jobID {
				return r, nil
			}
		}
	}
}

// externalize stores the input in the nats-kv bucket and returns the job's
// context_ref, which expires at the deadline.
func (c *client) externalize(ctx context.Context, nc *nats.Conn) (*agentv1.ResourceRef, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, errTransport
	}
	kv, err := js.KeyValue(ctx, c.bucket)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBucket, err)
	}
	backend, err := resourcekv.New(ctx, kv)
	if err != nil {
		return nil, err
	}
	registry, err := capsdk.NewResourceRegistry(nil, backend.Registration())
	if err != nil {
		return nil, err
	}
	expires, _ := ctx.Deadline()
	return registry.Externalize(ctx, resourcekv.ResolverID, c.inputFile, c.mediaType, inputPurpose, expires,
		capsdk.TrustedResourceContext{TenantID: c.tenantID, JobID: c.jobID})
}

// request signs the JobRequest; the identity binding carries the tenant.
func (c *client) request(ref *agentv1.ResourceRef) ([]byte, error) {
	id := make([]byte, 16)
	_, _ = rand.Read(id) // never fails since Go 1.24
	p := &agentv1.BusPacket{TraceId: hex.EncodeToString(id), SenderId: c.senderID, ProtocolVersion: capsdk.DefaultProtocolVersion,
		CreatedAt: timestamppb.Now(), Identity: &agentv1.IdentityBinding{TenantId: c.tenantID},
		SignatureMetadata: &agentv1.SignatureMetadata{ProfileVersion: capsdk.ProductionProfileVersion, Algorithm: capsdk.ProductionAlgorithm,
			MessageId: id, Audience: referencescheduler.SubjectSubmit, KeyId: c.signingKeyID, ExpiresAt: timestamppb.New(time.Now().Add(2 * time.Minute))},
		Payload: &agentv1.BusPacket_JobRequest{JobRequest: &agentv1.JobRequest{JobId: c.jobID, Topic: c.topic, ContextRef: ref,
			Meta: &agentv1.JobMetadata{Capability: c.capability}}}}
	return capsdk.SignProductionPacket(p, c.key)
}

// summary is the printed result; result_* name the output, never its content.
type summary struct {
	JobID           string `json:"job_id"`
	Status          string `json:"status"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message,omitempty"`
	WorkerID        string `json:"worker_id,omitempty"`
	ResultURI       string `json:"result_uri,omitempty"`
	ResultSHA256    string `json:"result_sha256,omitempty"`
	ResultMediaType string `json:"result_media_type,omitempty"`
	ResultSizeBytes uint64 `json:"result_size_bytes,omitempty"`
}

func printSummary(w io.Writer, r *agentv1.JobResult) {
	ref := r.GetResultRef()
	_ = json.NewEncoder(w).Encode(summary{JobID: clip(r.GetJobId()), Status: r.GetStatus().String(), ErrorCode: r.GetErrorCodeEnum().String(),
		ErrorMessage: clip(r.GetErrorMessage()), WorkerID: clip(r.GetWorkerId()), ResultURI: clip(ref.GetUri()),
		ResultSHA256: clip(hex.EncodeToString(ref.GetSha256())), ResultMediaType: clip(ref.GetMediaType()), ResultSizeBytes: ref.GetSizeBytes()})
}

func clip(s string) string { return s[:min(len(s), maxField)] }
