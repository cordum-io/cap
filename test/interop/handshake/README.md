# Installed-artifact handshake clients

These small clients drive Cordum's build-tagged real-NATS/real-Redis
interoperability gate. They intentionally use only public CAP APIs from the
installed Go module, Python wheel, or Node tarball.

They cover valid ISSUE and RENEW with token rotation plus impersonation, exact
replay, clock skew, a validly re-signed wrong audience, missing identity/trace,
unsupported version, and post-signing tamper. Invalid wire packets use each
language's raw protobuf encoder so local safe-marshal validation cannot turn a
server-bound negative into a false pass.

`manifest_driver_test.go` first proves that all four positive fixtures pass the
public Go SDK packet and signature validators. It then classifies all 38
declared vectors without treating bookkeeping as execution: 19 mutate real
fixture bytes and must produce a typed rejection from `ValidateWorkerTrustPacket`
or `VerifyTrustHandshake`; the other 19 are explicitly `SERVER_REQUIRED`.

The server-required group needs validly re-signed peer input or authoritative
time, replay, session-token, rotation, and Redis state. It is deliberately not
simulated with counters or booleans here. Only implementation-level tests such
as Cordum's stateful real-NATS/real-Redis handshake harness can prove those
outcomes. Cordum's `TestCAPManifestServerVectorsExecuteAgainstScheduler`
reads this manifest from the installed CAP module, requires a registered
handler for every one of the 19 IDs, and compares each declared outcome. A
green local CAP manifest test alone is not server conformance evidence.

The clients emit bounded status JSON only. They never print session tokens,
private keys, signatures, or packet bytes. They are test drivers, not independent
CAP implementations and not evidence of external conformance.
