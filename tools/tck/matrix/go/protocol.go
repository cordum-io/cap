package main

// The driver protocol is shared verbatim by the Go, Python, and Node drivers.
// Every field is language-neutral JSON so no SDK can smuggle its own encoding
// assumptions into another SDK's view of a fixture.

type corpusCase struct {
	Name      string            `json:"name"`
	TraceID   string            `json:"traceId"`
	MessageID string            `json:"messageId"` // base64, exactly 16 bytes
	Identity  corpusIdentity    `json:"identity"`
	Payload   map[string]any    `json:"payload"`
	Env       map[string]string `json:"env,omitempty"`
}

type corpusIdentity struct {
	TenantID     string `json:"tenantId"`
	PrincipalID  string `json:"principalId"`
	ActorID      string `json:"actorId"`
	DelegationID string `json:"delegationId"`
}

type corpus struct {
	Audience string       `json:"audience"`
	TenantID string       `json:"tenantId"`
	SenderID string       `json:"senderId"`
	Cases    []corpusCase `json:"cases"`
}

type produceRequest struct {
	SDK           string `json:"sdk"`
	Corpus        corpus `json:"corpus"`
	KeyID         string `json:"keyId"`
	PrivateKeyPEM string `json:"privateKeyPem"` // PKCS#8, P-256
	ExpiresAtUnix int64  `json:"expiresAtUnix"`
}

type fixtureOut struct {
	Case             string `json:"case"`
	Wire             string `json:"wire"` // base64 exact CAP-PRODUCTION raw bytes
	NormalizedDigest string `json:"normalizedDigest"`
	PreimageDigest   string `json:"preimageDigest"`
	KeyID            string `json:"keyId"`
}

type produceResponse struct {
	SDK      string       `json:"sdk"`
	Fixtures []fixtureOut `json:"fixtures"`
}

type consumeJob struct {
	ID           string `json:"id"`
	Wire         string `json:"wire"`
	KeyID        string `json:"keyId"`
	PublicKeyDER string `json:"publicKeyDer"` // base64 PKIX/SPKI
}

type consumeRequest struct {
	SDK      string       `json:"sdk"`
	Audience string       `json:"audience"`
	TenantID string       `json:"tenantId"`
	SenderID string       `json:"senderId"`
	Jobs     []consumeJob `json:"jobs"`
}

type consumeResult struct {
	ID               string `json:"id"`
	OK               bool   `json:"ok"`
	NormalizedDigest string `json:"normalizedDigest"`
	PreimageDigest   string `json:"preimageDigest"`
	Error            string `json:"error"`
}

type consumeResponse struct {
	SDK     string          `json:"sdk"`
	Results []consumeResult `json:"results"`
}
