package capsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AgentSpec is the input shape for registering a new agent identity
// against a Cordum control plane. Only fields the operator wants to
// pin should be populated; empty strings / nil slices are omitted from
// the wire payload so the server-side defaults apply.
//
// Note: PreapprovedMutatingTools is intentionally NOT a field on
// AgentSpec. The control plane treats the preapproved set as a
// post-registration scope adjustment (SetScope) so a malicious or
// misconfigured registrant cannot grant itself preapproved mutating
// tools at create time. Operators MUST follow Register with a SetScope
// call to grant that capability.
type AgentSpec struct {
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	Owner               string   `json:"owner"`
	Team                string   `json:"team,omitempty"`
	RiskTier            string   `json:"risk_tier"`
	AllowedTopics       []string `json:"allowed_topics,omitempty"`
	AllowedPools        []string `json:"allowed_pools,omitempty"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	DataClassifications []string `json:"data_classifications,omitempty"`
}

// AgentIdentity mirrors the canonical agent record returned by the
// control plane. ID is the server-assigned UUID; CreatedAt / UpdatedAt
// are RFC3339 strings.
type AgentIdentity struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Description              string   `json:"description,omitempty"`
	Owner                    string   `json:"owner"`
	Team                     string   `json:"team,omitempty"`
	RiskTier                 string   `json:"risk_tier"`
	AllowedTopics            []string `json:"allowed_topics,omitempty"`
	AllowedPools             []string `json:"allowed_pools,omitempty"`
	AllowedTools             []string `json:"allowed_tools,omitempty"`
	DataClassifications      []string `json:"data_classifications,omitempty"`
	PreapprovedMutatingTools []string `json:"preapproved_mutating_tools,omitempty"`
	Status                   string   `json:"status"`
	CreatedAt                string   `json:"created_at"`
	UpdatedAt                string   `json:"updated_at"`
	LastActive               int64    `json:"last_active,omitempty"`
}

// AgentScopeUpdate is the input for SetScope. Per-field semantics
// mirror the control plane: a nil slice means "leave alone"; an empty
// (non-nil) slice means "clear this scope dimension". For
// PreapprovedMutatingTools, both nil and empty mean clear — the wire
// payload always sends the field explicitly so operators have a
// deterministic way to revoke preapproved mutations.
type AgentScopeUpdate struct {
	AgentID                  string
	AllowedTools             []string
	AllowedTopics            []string
	AllowedPools             []string
	DataClassifications      []string
	PreapprovedMutatingTools []string
	Status                   string
	// IdempotencyKey, when non-empty, is sent as `Idempotency-Key`
	// header so retries against the same intent collapse server-side.
	IdempotencyKey string
}

// AgentClientConfig is the boot-time configuration for an AgentClient.
type AgentClientConfig struct {
	// BaseURL is the Cordum gateway root, e.g. https://gateway:8443.
	// The client appends /api/v1/agents itself.
	BaseURL string
	// APIKey is the service-account credential; sent as `X-API-Key`
	// when no per-call bearer token is provided.
	APIKey string
	// BearerToken, when non-empty, replaces X-API-Key on every call.
	// Service accounts that mint per-session delegation tokens prefer
	// this path.
	BearerToken string
	// Tenant, when non-empty, is sent as `X-Tenant-ID` so the gateway
	// resolves the correct tenant scope.
	Tenant string
	// HTTPClient lets tests inject a transport. nil = http.DefaultClient
	// with no timeout (the caller's context governs cancellation).
	HTTPClient *http.Client
}

// AgentClient wraps Register / Lookup / SetScope against the Cordum
// gateway's /api/v1/agents endpoints. Goroutine-safe.
type AgentClient struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	bearerToken string
	tenant      string
}

// NewAgentClient validates cfg and returns a ready-to-use client.
func NewAgentClient(cfg AgentClientConfig) (*AgentClient, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("capsdk/agent: BaseURL is required")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	return &AgentClient{
		httpClient:  hc,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:      cfg.APIKey,
		bearerToken: cfg.BearerToken,
		tenant:      cfg.Tenant,
	}, nil
}

// Register creates a new agent identity. Returns the server-assigned
// agent_id on success. Per the AgentSpec note, PreapprovedMutatingTools
// MUST be set via a follow-up SetScope call, not at register time.
func (c *AgentClient) Register(ctx context.Context, spec AgentSpec) (string, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return "", errors.New("capsdk/agent: Register: spec.Name is required")
	}
	body, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("capsdk/agent: marshal: %w", err)
	}
	var out AgentIdentity
	if err := c.do(ctx, http.MethodPost, "/api/v1/agents", body, "", &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", errors.New("capsdk/agent: Register: server returned empty agent id")
	}
	return out.ID, nil
}

// ErrAgentNotFound is returned by Lookup when no agent with the
// requested name exists in the requested tenant scope.
var ErrAgentNotFound = errors.New("capsdk/agent: not found")

// ErrAgentDuplicate is returned by Lookup when more than one agent
// with the requested name exists in scope. Caller must surface this
// as an operator-visible error: bootstrapping cannot pick which is
// the canonical identity.
var ErrAgentDuplicate = errors.New("capsdk/agent: duplicate identity")

// Lookup searches for an existing agent by name within the requested
// tenant scope. Returns nil + ErrAgentNotFound if no match. Returns
// ErrAgentDuplicate if multiple match (operator must resolve before
// service can boot deterministically).
//
// Tenant filtering: when tenant is non-empty, the result MUST match
// the supplied tenant. When tenant is empty, the gateway-side tenant
// header (configured at NewAgentClient) is the scope.
func (c *AgentClient) Lookup(ctx context.Context, name, tenant string) (*AgentIdentity, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("capsdk/agent: Lookup: name is required")
	}
	q := url.Values{}
	q.Set("name", name)
	if t := strings.TrimSpace(tenant); t != "" {
		q.Set("tenant", t)
	}
	path := "/api/v1/agents?" + q.Encode()
	var resp struct {
		Items []AgentIdentity `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, "", &resp); err != nil {
		return nil, err
	}
	matches := make([]AgentIdentity, 0, len(resp.Items))
	for _, item := range resp.Items {
		if !strings.EqualFold(item.Name, name) {
			continue
		}
		matches = append(matches, item)
	}
	switch len(matches) {
	case 0:
		return nil, ErrAgentNotFound
	case 1:
		out := matches[0]
		return &out, nil
	default:
		return nil, fmt.Errorf("%w: name=%q tenant=%q count=%d", ErrAgentDuplicate, name, tenant, len(matches))
	}
}

// SetScope applies the scope update to an existing agent. PUT
// /api/v1/agents/{id} with the explicit allowed_*/preapproved_*
// fields. Idempotency-Key, when supplied, lets retries fold safely
// at the server.
//
// PreapprovedMutatingTools is ALWAYS sent (even empty []) so an
// operator can deterministically revoke. Other fields (AllowedTopics
// etc.) follow nil=leave, []=clear semantics matching the gateway.
func (c *AgentClient) SetScope(ctx context.Context, update AgentScopeUpdate) error {
	id := strings.TrimSpace(update.AgentID)
	if id == "" {
		return errors.New("capsdk/agent: SetScope: AgentID is required")
	}
	body := map[string]any{}
	if update.AllowedTools != nil {
		body["allowed_tools"] = append([]string{}, update.AllowedTools...)
	}
	if update.AllowedTopics != nil {
		body["allowed_topics"] = append([]string{}, update.AllowedTopics...)
	}
	if update.AllowedPools != nil {
		body["allowed_pools"] = append([]string{}, update.AllowedPools...)
	}
	if update.DataClassifications != nil {
		body["data_classifications"] = append([]string{}, update.DataClassifications...)
	}
	// Always send preapproved_mutating_tools — empty slice means
	// "revoke all preapproved mutations", which must be deterministic.
	body["preapproved_mutating_tools"] = append([]string{}, update.PreapprovedMutatingTools...)
	if v := strings.TrimSpace(update.Status); v != "" {
		body["status"] = v
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("capsdk/agent: marshal: %w", err)
	}
	path := "/api/v1/agents/" + url.PathEscape(id)
	return c.do(ctx, http.MethodPut, path, raw, update.IdempotencyKey, nil)
}

// do is the shared request path. Auth hierarchy: bearer token (when
// non-empty) replaces X-API-Key entirely, so a per-session delegation
// token does not leak the service-account API key downstream.
func (c *AgentClient) do(ctx context.Context, method, path string, body []byte, idempotencyKey string, out any) error {
	full := c.baseURL + path
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return fmt.Errorf("capsdk/agent: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	} else if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if c.tenant != "" {
		req.Header.Set("X-Tenant-ID", c.tenant)
	}
	if k := strings.TrimSpace(idempotencyKey); k != "" {
		req.Header.Set("Idempotency-Key", k)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("capsdk/agent: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap response read at 4 MiB so a misbehaving gateway can't OOM
	// the SDK caller.
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if readErr != nil {
		return fmt.Errorf("capsdk/agent: read body: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &AgentAPIError{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Body:       truncateBody(string(respBody), 1024),
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("capsdk/agent: decode body: %w", err)
	}
	return nil
}

// AgentAPIError wraps a non-2xx response from the agent endpoints.
// Callers use errors.As to inspect StatusCode for retry / typed
// handling.
type AgentAPIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *AgentAPIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("capsdk/agent: %s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("capsdk/agent: %s %s: status %d", e.Method, e.Path, e.StatusCode)
}

func truncateBody(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
