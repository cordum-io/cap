package publicreport

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func signingKey(seed byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
}

func verifyError(t *testing.T, report, sig []byte, pub ed25519.PublicKey, want error) {
	t.Helper()
	r, err := Verify(report, sig, pub)
	if !errors.Is(err, want) || !reflect.DeepEqual(r, Report{}) {
		t.Fatalf("Verify = %+v, %v; want zero report, %v", r, err, want)
	}
}

func TestSignRoundTripS1(t *testing.T) {
	key := signingKey(1)
	for _, name := range []string{"pass", "fail", "incomplete"} {
		t.Run(name, func(t *testing.T) {
			b := readTestdata(t, name+".golden.json")
			want, err := Validate(b)
			if err != nil {
				t.Fatal(err)
			}
			sig, err := Sign(b, key)
			if err != nil || len(sig) != ed25519.SignatureSize {
				t.Fatalf("Sign = %x, %v", sig, err)
			}
			got, err := Verify(b, sig, key.Public().(ed25519.PublicKey))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("Verify = %+v, %v", got, err)
			}
			again, err := Sign(b, key)
			if err != nil || !bytes.Equal(sig, again) {
				t.Fatalf("not deterministic: %v", err)
			}
		})
	}
}

func TestVerifyEveryReportByteS2(t *testing.T) {
	b, key := readTestdata(t, "pass.golden.json"), signingKey(1)
	sig := ed25519.Sign(key, b)
	for i := range b {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			flipped := bytes.Clone(b)
			flipped[i] ^= 1
			verifyError(t, flipped, sig, key.Public().(ed25519.PublicKey), ErrSignatureMismatch)
		})
	}
}

func TestVerifyEverySignatureByteS3(t *testing.T) {
	b, key := readTestdata(t, "pass.golden.json"), signingKey(1)
	sig := ed25519.Sign(key, b)
	for i := range sig {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			flipped := bytes.Clone(sig)
			flipped[i] ^= 1
			verifyError(t, b, flipped, key.Public().(ed25519.PublicKey), ErrSignatureMismatch)
		})
	}
}

func TestVerifyWrongKeyS4(t *testing.T) {
	b := readTestdata(t, "pass.golden.json")
	verifyError(t, b, ed25519.Sign(signingKey(1), b), signingKey(2).Public().(ed25519.PublicKey), ErrSignatureMismatch)
}

func TestSignVerifyKeySizesS5(t *testing.T) {
	b := readTestdata(t, "pass.golden.json")
	for _, n := range []int{0, 32, 63, 65} {
		t.Run(fmt.Sprintf("private-%d", n), func(t *testing.T) {
			sig, err := Sign(b, make(ed25519.PrivateKey, n))
			if !errors.Is(err, ErrBadKey) || sig != nil {
				t.Fatalf("Sign = %x, %v", sig, err)
			}
		})
	}
	for _, n := range []int{0, 31, 33, 64} {
		t.Run(fmt.Sprintf("public-%d", n), func(t *testing.T) {
			verifyError(t, b, nil, make(ed25519.PublicKey, n), ErrBadKey)
		})
	}
}

func TestVerifySignatureSizesS6(t *testing.T) {
	for _, n := range []int{0, 63, 65} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			verifyError(t, nil, make([]byte, n), signingKey(1).Public().(ed25519.PublicKey), ErrBadSignature)
		})
	}
}

func invalidSigningInputs(t *testing.T) []struct {
	name string
	data []byte
	want error
} {
	t.Helper()
	b := readTestdata(t, "pass.golden.json")
	var indented bytes.Buffer
	if err := json.Indent(&indented, b, "", "  "); err != nil {
		t.Fatal(err)
	}
	return []struct {
		name string
		data []byte
		want error
	}{
		{"crlf", bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n")), ErrNotCanonical},
		{"indented", indented.Bytes(), ErrNotCanonical},
		{"trailing-space", append(bytes.Clone(b), ' '), ErrNotCanonical},
		{"unknown-field", bytes.Replace(b, []byte(`{"schemaVersion"`), []byte(`{"extra":true,"schemaVersion"`), 1), ErrUnknownField},
		{"verdict", bytes.Replace(b, []byte(`"verdict":"PASS"`), []byte(`"verdict":"FAIL"`), 1), ErrVerdictMismatch},
	}
}

func TestSignInvalidInputS7(t *testing.T) {
	for _, tc := range invalidSigningInputs(t) {
		t.Run(tc.name, func(t *testing.T) {
			sig, err := Sign(tc.data, signingKey(1))
			if !errors.Is(err, tc.want) || sig != nil {
				t.Fatalf("Sign = %x, %v; want nil, %v", sig, err, tc.want)
			}
		})
	}
}

func TestVerifySignedInvalidInputS8(t *testing.T) {
	key := signingKey(1)
	for _, tc := range invalidSigningInputs(t) {
		t.Run(tc.name, func(t *testing.T) {
			verifyError(t, tc.data, ed25519.Sign(key, tc.data), key.Public().(ed25519.PublicKey), tc.want)
		})
	}
}
