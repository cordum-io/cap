package publicreport

import (
	"crypto/ed25519"
	"errors"
)

// Signature errors. Sign and Verify also return Validate's sentinels for a
// report that is not a valid canonical public report.
var (
	ErrBadKey            = errors.New("publicreport: not an Ed25519 key of the right size")
	ErrBadSignature      = errors.New("publicreport: signature is not 64 bytes")
	ErrSignatureMismatch = errors.New("publicreport: signature does not match the report and key")
)

// Sign returns the detached signature over a public report: a raw 64-byte
// Ed25519 (RFC 8032) signature over the exact canonical bytes. It signs only
// bytes that Validate accepts, so a signature never covers a non-canonical copy.
func Sign(report []byte, key ed25519.PrivateKey) ([]byte, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, ErrBadKey
	}
	if _, err := Validate(report); err != nil {
		return nil, err
	}
	return ed25519.Sign(key, report), nil
}

// Verify checks sig over the exact report bytes before it parses them, then
// validates the report. On any error it returns the zero Report.
func Verify(report, sig []byte, pub ed25519.PublicKey) (Report, error) {
	switch {
	case len(pub) != ed25519.PublicKeySize:
		return Report{}, ErrBadKey
	case len(sig) != ed25519.SignatureSize:
		return Report{}, ErrBadSignature
	case !ed25519.Verify(pub, report, sig):
		return Report{}, ErrSignatureMismatch
	}
	return Validate(report)
}
