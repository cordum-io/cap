package capsdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestAgentClient(t *testing.T, baseURL string) *AgentClient {
	t.Helper()
	c, err := NewAgentClient(AgentClientConfig{
		BaseURL: baseURL,
		APIKey:  "svc-test-key",
		Tenant:  "tenant-a",
	})
	if err != nil {
		t.Fatalf("NewAgentClient: %v", err)
	}
	return c
}

func TestNewAgentClient_RequiresBaseURL(t *testing.T) {
	t.Parallel()
	if _, err := NewAgentClient(AgentClientConfig{}); err == nil {
		t.Fatal("expected error for empty BaseURL")
	}
}

// Register: POSTs /api/v1/agents, sends spec fields, does NOT include
// preapproved_mutating_tools (rail #2), and surfaces server-assigned id.
func TestAgentClient_Register_Success(t *testing.T) {
	t.Parallel()
	var seenMethod, seenPath, seenAPIKey, seenTenant string
	var bodyBytes []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAPIKey = r.Header.Get("X-API-Key")
		seenTenant = r.Header.Get("X-Tenant-ID")
		bodyBytes, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(AgentIdentity{
			ID:        "agent-uuid-1",
			Name:      "chat-assistant",
			Owner:     "platform",
			RiskTier:  "medium",
			Status:    "active",
			CreatedAt: "2026-04-26T10:00:00Z",
			UpdatedAt: "2026-04-26T10:00:00Z",
		})
	}))
	defer srv.Close()

	c := newTestAgentClient(t, srv.URL)
	id, err := c.Register(context.Background(), AgentSpec{
		Name:          "chat-assistant",
		Owner:         "platform",
		RiskTier:      "medium",
		AllowedTools:  []string{"cordum_list_jobs", "cordum_submit_job"},
		AllowedTopics: []string{"job.*"},
	})
	if err != nil {
		t.Fatalf("Register err: %v", err)
	}
	if id != "agent-uuid-1" {
		t.Errorf("id = %q, want agent-uuid-1", id)
	}
	if seenMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", seenMethod)
	}
	if seenPath != "/api/v1/agents" {
		t.Errorf("path = %q, want /api/v1/agents", seenPath)
	}
	if seenAPIKey != "svc-test-key" {
		t.Errorf("X-API-Key = %q", seenAPIKey)
	}
	if seenTenant != "tenant-a" {
		t.Errorf("X-Tenant-ID = %q", seenTenant)
	}
	// Critical: preapproved_mutating_tools MUST NOT appear in the
	// register payload (rail #2).
	if strings.Contains(string(bodyBytes), "preapproved_mutating_tools") {
		t.Errorf("Register sent preapproved_mutating_tools — rail #2 violation: body=%s", bodyBytes)
	}
	// allowed_tools should appear.
	if !strings.Contains(string(bodyBytes), "allowed_tools") {
		t.Errorf("Register did not send allowed_tools: body=%s", bodyBytes)
	}
}

func TestAgentClient_Register_RequiresName(t *testing.T) {
	t.Parallel()
	c := newTestAgentClient(t, "http://localhost")
	if _, err := c.Register(context.Background(), AgentSpec{Owner: "p", RiskTier: "low"}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestAgentClient_Register_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	_, err := c.Register(context.Background(), AgentSpec{Name: "x", Owner: "y", RiskTier: "low"})
	var apiErr *AgentAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *AgentAPIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

// Lookup: GETs /api/v1/agents with name+tenant query, filters by name
// case-insensitive, returns ErrAgentNotFound on no match,
// ErrAgentDuplicate on multi-match.
func TestAgentClient_Lookup_Hit(t *testing.T) {
	t.Parallel()
	var seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []AgentIdentity{
				{ID: "agent-1", Name: "chat-assistant", Owner: "platform", RiskTier: "medium", Status: "active"},
			},
		})
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	got, err := c.Lookup(context.Background(), "chat-assistant", "tenant-a")
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if got == nil || got.ID != "agent-1" {
		t.Fatalf("got = %+v, want id=agent-1", got)
	}
	if !strings.Contains(seenQuery, "name=chat-assistant") {
		t.Errorf("query = %q, want name=chat-assistant", seenQuery)
	}
	if !strings.Contains(seenQuery, "tenant=tenant-a") {
		t.Errorf("query = %q, want tenant=tenant-a", seenQuery)
	}
}

func TestAgentClient_Lookup_NotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []AgentIdentity{}})
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	got, err := c.Lookup(context.Background(), "missing", "tenant-a")
	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("err = %v, want ErrAgentNotFound", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

func TestAgentClient_Lookup_Duplicate(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []AgentIdentity{
				{ID: "agent-1", Name: "chat-assistant"},
				{ID: "agent-2", Name: "chat-assistant"},
			},
		})
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	_, err := c.Lookup(context.Background(), "chat-assistant", "tenant-a")
	if !errors.Is(err, ErrAgentDuplicate) {
		t.Fatalf("err = %v, want ErrAgentDuplicate", err)
	}
}

// Server returns extra rows that don't match the requested name —
// client filters them out.
func TestAgentClient_Lookup_FiltersByName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []AgentIdentity{
				{ID: "agent-x", Name: "other-agent"},
				{ID: "agent-1", Name: "chat-assistant"},
			},
		})
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	got, err := c.Lookup(context.Background(), "chat-assistant", "")
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if got.ID != "agent-1" {
		t.Errorf("got.ID = %q, want agent-1", got.ID)
	}
}

func TestAgentClient_Lookup_RequiresName(t *testing.T) {
	t.Parallel()
	c := newTestAgentClient(t, "http://localhost")
	if _, err := c.Lookup(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}

// SetScope: PUT /api/v1/agents/{id}, sends preapproved_mutating_tools
// always (even empty), respects nil=leave for other fields.
func TestAgentClient_SetScope_SendsPreapprovedExplicitly(t *testing.T) {
	t.Parallel()
	var seenMethod, seenPath, seenIdempotency string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenIdempotency = r.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestAgentClient(t, srv.URL)
	err := c.SetScope(context.Background(), AgentScopeUpdate{
		AgentID:                  "agent-1",
		AllowedTools:             []string{"cordum_list_jobs", "cordum_submit_job"},
		PreapprovedMutatingTools: []string{"cordum_submit_job"},
		Status:                   "active",
		IdempotencyKey:           "boot-2026-04-26",
	})
	if err != nil {
		t.Fatalf("SetScope err: %v", err)
	}
	if seenMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", seenMethod)
	}
	if seenPath != "/api/v1/agents/agent-1" {
		t.Errorf("path = %q", seenPath)
	}
	if seenIdempotency != "boot-2026-04-26" {
		t.Errorf("Idempotency-Key = %q, want boot-2026-04-26", seenIdempotency)
	}
	// Critical: preapproved_mutating_tools field MUST be in the body
	// even when caller passed nil/empty (rail: deterministic revoke).
	if _, ok := body["preapproved_mutating_tools"]; !ok {
		t.Errorf("body missing preapproved_mutating_tools: %+v", body)
	}
	// allowed_tools sent
	if _, ok := body["allowed_tools"]; !ok {
		t.Errorf("body missing allowed_tools: %+v", body)
	}
	// allowed_topics NOT sent (nil=leave)
	if _, ok := body["allowed_topics"]; ok {
		t.Errorf("body contains allowed_topics despite nil input: %+v", body)
	}
}

// Empty PreapprovedMutatingTools must STILL appear (revoke-all
// semantics).
func TestAgentClient_SetScope_EmptyPreapprovedSendsExplicitly(t *testing.T) {
	t.Parallel()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	if err := c.SetScope(context.Background(), AgentScopeUpdate{
		AgentID:                  "agent-1",
		PreapprovedMutatingTools: []string{},
	}); err != nil {
		t.Fatalf("SetScope err: %v", err)
	}
	v, ok := body["preapproved_mutating_tools"]
	if !ok {
		t.Fatalf("body missing preapproved_mutating_tools: %+v", body)
	}
	arr, ok := v.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("preapproved_mutating_tools = %v, want []", v)
	}
}

func TestAgentClient_SetScope_RequiresAgentID(t *testing.T) {
	t.Parallel()
	c := newTestAgentClient(t, "http://localhost")
	if err := c.SetScope(context.Background(), AgentScopeUpdate{}); err == nil {
		t.Fatal("expected error for empty AgentID")
	}
}

// Bearer token replaces X-API-Key (per-call delegation tokens never
// accompany the service key).
func TestAgentClient_BearerSupplantsAPIKey(t *testing.T) {
	t.Parallel()
	var seenAuth, seenAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenAPIKey = r.Header.Get("X-API-Key")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []AgentIdentity{}})
	}))
	defer srv.Close()
	c, err := NewAgentClient(AgentClientConfig{
		BaseURL:     srv.URL,
		APIKey:      "svc-test-key",
		BearerToken: "delegation-token-abc",
	})
	if err != nil {
		t.Fatalf("NewAgentClient: %v", err)
	}
	_, _ = c.Lookup(context.Background(), "x", "")
	if seenAuth != "Bearer delegation-token-abc" {
		t.Errorf("Authorization = %q, want Bearer delegation-token-abc", seenAuth)
	}
	if seenAPIKey != "" {
		t.Errorf("X-API-Key = %q, want empty (bearer must replace, not accompany)", seenAPIKey)
	}
}

// Context cancellation surfaces as ctx error.
func TestAgentClient_CtxCancel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.Lookup(ctx, "x", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// Counter test: SetScope makes exactly one HTTP request.
func TestAgentClient_SetScope_SingleRequest(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := newTestAgentClient(t, srv.URL)
	if err := c.SetScope(context.Background(), AgentScopeUpdate{AgentID: "agent-1"}); err != nil {
		t.Fatalf("SetScope err: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}
