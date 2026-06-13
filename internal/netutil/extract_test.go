package netutil

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

func TestExtractURL(t *testing.T) {
	var buf bytes.Buffer
	writeTarXZ(t, &buf, []tarEntry{
		{name: "nested/file.txt", body: "hello"},
	})

	withTestClient(t, http.StatusOK, buf.Bytes())

	dir := t.TempDir()
	if err := ExtractURL("https://example.test/archive.tar.xz", dir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected file content: %q", got)
	}
}

func TestExtractURLBadStatus(t *testing.T) {
	withTestClient(t, http.StatusNotFound, nil)

	if err := ExtractURL("https://example.test/archive.tar.xz", t.TempDir()); !errors.Is(err, ErrBadStatus) {
		t.Fatalf("expected ErrBadStatus, got %v", err)
	}
}

func TestExtractURLRejectsUnsafePaths(t *testing.T) {
	tests := []tarEntry{
		{name: "../escape.txt", body: "nope"},
		{name: "/tmp/escape.txt", body: "nope"},
		{name: "safe/link", linkname: "../../escape.txt", typ: tar.TypeSymlink},
		{name: "safe/hardlink", linkname: "../escape.txt", typ: tar.TypeLink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeTarXZ(t, &buf, []tarEntry{tt})

			withTestClient(t, http.StatusOK, buf.Bytes())

			if err := ExtractURL("https://example.test/archive.tar.xz", t.TempDir()); err == nil {
				t.Fatal("expected unsafe path error")
			}
		})
	}
}

func withTestClient(t *testing.T, status int, body []byte) {
	t.Helper()

	old := client
	client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	t.Cleanup(func() {
		client = old
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type tarEntry struct {
	name     string
	body     string
	linkname string
	typ      byte
}

func writeTarXZ(t *testing.T, dst *bytes.Buffer, entries []tarEntry) {
	t.Helper()

	xzw, err := xz.NewWriter(dst)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(xzw)

	for _, entry := range entries {
		typ := entry.typ
		if typ == 0 {
			typ = tar.TypeReg
		}

		h := &tar.Header{
			Name:       entry.name,
			Mode:       0o644,
			Size:       int64(len(entry.body)),
			Typeflag:   typ,
			Linkname:   entry.linkname,
			AccessTime: time.Unix(1, 0),
			ModTime:    time.Unix(1, 0),
		}
		if typ != tar.TypeReg {
			h.Size = 0
		}

		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := xzw.Close(); err != nil {
		t.Fatal(err)
	}
}
