package main

import (
	"bytes"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Input bounds: publicreport leaves size limits to its caller.
const (
	maxTCKBytes      = 8 << 20
	maxReportBytes   = 1 << 20
	maxKeyBytes      = 16 << 10
	maxScenarioBytes = 1 << 20
	maxScenarioFiles = 64
)

var (
	errTooLarge         = errors.New("input file is over its size limit")
	errKeyFile          = errors.New("key file is not exactly one PEM block of the expected type")
	errNotEd25519       = errors.New("key is not an Ed25519 key")
	errUnsafeOutput     = errors.New("refusing unsafe output path")
	errManifest         = errors.New("bad scenario directory")
	errManifestMismatch = errors.New("TCK report cases differ from the scenario directory")
)

// writeTemp fills the output temp file. Tests replace it to inject a write error.
var writeTemp = func(w io.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}

// readBounded reads a whole file and refuses one over max bytes. A refused or
// failed read is zeroed, since key files go through here too.
func readBounded(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err == nil && int64(len(b)) > max {
		err = fmt.Errorf("%w: %s is over %d bytes", errTooLarge, path, max)
	}
	if err != nil {
		clear(b)
		return nil, err
	}
	return b, nil
}

// readPEM returns the DER bytes of the one PEM block in a key file. The file
// buffer is zeroed here; the caller zeroes the returned DER.
func readPEM(path, blockType string) ([]byte, error) {
	raw, err := readBounded(path, maxKeyBytes)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != blockType || len(block.Headers) > 0 || len(bytes.TrimSpace(rest)) > 0 {
		if block != nil {
			clear(block.Bytes)
		}
		return nil, fmt.Errorf("%w: want one %q block", errKeyFile, blockType)
	}
	return block.Bytes, nil
}

// writeOutput writes data to name inside dir: a 0600 temp file in the same
// directory, then a rename. Every step goes through an os.Root, so no "..",
// absolute path or symlink can leave dir. An existing name must be a regular
// file. The temp file is removed on every error.
func writeOutput(dir, name string, data []byte) (err error) {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("%w: %q is not a relative path inside --out-dir", errUnsafeOutput, name)
	}
	name = filepath.Clean(name)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if fi, lerr := root.Lstat(name); lerr == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s exists and is not a regular file", errUnsafeOutput, name)
	} else if lerr != nil && !errors.Is(lerr, fs.ErrNotExist) {
		return fmt.Errorf("%w: %v", errUnsafeOutput, lerr)
	}
	tmp := filepath.Join(filepath.Dir(name), "."+filepath.Base(name)+"."+rand.Text()+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmp)
		}
	}()
	if err = writeTemp(f, data); err == nil {
		err = f.Chmod(0o600)
	}
	if err == nil {
		err = f.Sync()
	}
	if err == nil {
		err = f.Close()
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	return err
}
