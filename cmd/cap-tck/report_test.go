package main

import (
	"bytes"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func putFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestOutputUnsafeNamesO1(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	names := []string{"../x", "a/../../x", filepath.Join(outside, "x"), "", ".", "d"}
	if runtime.GOOS == "windows" {
		names = append(names, "NUL", "COM1")
	}
	before := dirNames(t, dir)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := writeOutput(dir, name, []byte("new")); !errors.Is(err, errUnsafeOutput) {
				t.Fatalf("writeOutput = %v", err)
			}
			if !reflect.DeepEqual(dirNames(t, dir), before) || len(dirNames(t, outside)) != 0 {
				t.Fatal("directory changed")
			}
		})
	}
}

func TestOutputFinalSymlinksO2(t *testing.T) {
	for _, place := range []string{"inside", "outside"} {
		t.Run(place, func(t *testing.T) {
			dir, targetDir := t.TempDir(), t.TempDir()
			if place == "inside" {
				targetDir = dir
			}
			target := putFile(t, filepath.Join(targetDir, "target"), []byte("old"))
			link := filepath.Join(dir, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("os.Symlink: %v", err)
			}
			before := dirNames(t, dir)
			if err := writeOutput(dir, "link", []byte("new")); !errors.Is(err, errUnsafeOutput) {
				t.Fatalf("writeOutput = %v", err)
			}
			fi, err := os.Lstat(link)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("link changed: %v", err)
			}
			if string(fileBytes(t, target)) != "old" || !reflect.DeepEqual(dirNames(t, dir), before) {
				t.Fatal("output changed")
			}
		})
	}
}

func TestOutputParentSymlinkO2(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "esc")); err != nil {
		t.Skipf("os.Symlink: %v", err)
	}
	if err := writeOutput(dir, "esc/x", []byte("new")); !errors.Is(err, errUnsafeOutput) {
		t.Fatalf("writeOutput = %v", err)
	}
	if len(dirNames(t, outside)) != 0 || !reflect.DeepEqual(dirNames(t, dir), []string{"esc"}) {
		t.Fatal("output changed")
	}
}

func TestOutputModeAndReplaceO3(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"new", "existing"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if name == "existing" {
				putFile(t, path, []byte("old"))
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeOutput(dir, name, []byte("replacement")); err != nil {
				t.Fatal(err)
			}
			if string(fileBytes(t, path)) != "replacement" {
				t.Fatal("wrong output")
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
				t.Fatalf("mode = %o", fi.Mode().Perm())
			}
		})
	}
	if !reflect.DeepEqual(dirNames(t, dir), []string{"existing", "new"}) {
		t.Fatal("temp file left behind")
	}
}

func TestOutputWriteErrorO4(t *testing.T) {
	dir := t.TempDir()
	putFile(t, filepath.Join(dir, "existing"), []byte("old"))
	before := dirNames(t, dir)
	injected := errors.New("injected partial write failure")
	original := writeTemp
	t.Cleanup(func() { writeTemp = original })
	writeTemp = func(w io.Writer, b []byte) error {
		if _, err := w.Write(b[:1]); err != nil {
			return err
		}
		return injected
	}
	for _, name := range []string{"new", "existing"} {
		t.Run(name, func(t *testing.T) {
			if err := writeOutput(dir, name, []byte("new")); !errors.Is(err, injected) {
				t.Fatalf("writeOutput = %v", err)
			}
			if !reflect.DeepEqual(dirNames(t, dir), before) || string(fileBytes(t, filepath.Join(dir, "existing"))) != "old" {
				t.Fatal("partial write changed directory/target")
			}
		})
	}
}

func TestReadBoundedO5(t *testing.T) {
	path := putFile(t, filepath.Join(t.TempDir(), "input"), []byte("1234"))
	b, err := readBounded(path, 4)
	if err != nil || string(b) != "1234" {
		t.Fatalf("readBounded = %q, %v", b, err)
	}
	for _, max := range []int64{3, 0, -1} {
		b, err := readBounded(path, max)
		if !errors.Is(err, errTooLarge) || b != nil {
			t.Fatalf("max %d: %q, %v", max, b, err)
		}
	}
	if b, err := readBounded(path+"-missing", 4); err == nil || b != nil {
		t.Fatal("missing input accepted")
	}
	putFile(t, path, nil)
	if b, err := readBounded(path, 0); err != nil || len(b) != 0 {
		t.Fatalf("empty: %q, %v", b, err)
	}
}

func TestReadPEMO6(t *testing.T) {
	good := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("der")})
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"valid", good, nil},
		{"whitespace", append(bytes.Clone(good), ' ', '\n'), nil},
		{"wrong-type", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("der")}), errKeyFile},
		{"two-blocks", append(bytes.Clone(good), good...), errKeyFile},
		{"trailing", append(bytes.Clone(good), []byte("junk")...), errKeyFile},
		{"headers", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Headers: map[string]string{"X": "Y"}, Bytes: []byte("der")}), errKeyFile},
		{"not-pem", []byte("not a PEM"), errKeyFile},
		{"empty", nil, errKeyFile},
		{"oversize", bytes.Repeat([]byte{'x'}, maxKeyBytes+1), errTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := putFile(t, filepath.Join(t.TempDir(), "key.pem"), tc.data)
			b, err := readPEM(path, "PRIVATE KEY")
			if !errors.Is(err, tc.want) {
				t.Fatalf("readPEM = %q, %v; want %v", b, err, tc.want)
			}
			if tc.want != nil && b != nil || tc.want == nil && string(b) != "der" {
				t.Fatalf("unexpected DER: %q", b)
			}
		})
	}
}
