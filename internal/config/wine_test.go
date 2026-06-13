package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWineRoot(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "bin", "wine64"))

	if err := ValidateWineRoot(root); err != nil {
		t.Fatal(err)
	}

	if err := ValidateWineRoot("relative"); !errors.Is(err, ErrWineRootAbs) {
		t.Fatalf("expected ErrWineRootAbs, got %v", err)
	}

	if err := ValidateWineRoot(t.TempDir()); !errors.Is(err, ErrWineRootInvalid) {
		t.Fatalf("expected ErrWineRootInvalid, got %v", err)
	}
}

func TestValidateProtonRoot(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "proton"))

	if err := ValidateWineRoot(root); err != nil {
		t.Fatal(err)
	}
}

func TestResolveWineRootUsesPath(t *testing.T) {
	bin := t.TempDir()
	touch(t, filepath.Join(bin, "wine64"))
	t.Setenv("PATH", bin)

	root, candidate, err := ResolveWineRoot("")
	if err != nil {
		t.Fatal(err)
	}
	if root != "" {
		t.Fatalf("expected system Wine root, got %q", root)
	}
	if candidate.Binary != filepath.Join(bin, "wine64") {
		t.Fatalf("unexpected candidate: %#v", candidate)
	}
}

func TestResolveWineRootRejectsBadCustomRoot(t *testing.T) {
	_, _, err := ResolveWineRoot(t.TempDir())
	if !errors.Is(err, ErrWineRootInvalid) {
		t.Fatalf("expected ErrWineRootInvalid, got %v", err)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
