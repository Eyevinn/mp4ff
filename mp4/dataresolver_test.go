package mp4_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLocalFileResolver(t *testing.T) {
	wantStart, err := os.ReadFile("testdata/prog_8s.mp4")
	if err != nil {
		t.Fatal(err)
	}
	wantStart = wantStart[:16]
	cases := []struct {
		location  string
		wantErr   string
		wantErrIs error
	}{
		{location: "prog_8s.mp4"},
		{location: "./prog_8s.mp4"},
		{location: "sub/../prog_8s.mp4"},
		{location: "prog%5F8s.mp4"},
		{location: "../testdata/prog_8s.mp4", wantErr: "is not a path inside the directory"},
		{location: "/etc/hosts", wantErr: "is not a path inside the directory"},
		{location: "http://example.com/prog_8s.mp4", wantErr: "is not a relative path"},
		{location: "file:///prog_8s.mp4", wantErr: "is not a relative path"},
		{location: "C:/prog_8s.mp4", wantErr: "is not a relative path"},
		{location: "prog_8s.mp4?x=1", wantErr: "is not a relative path"},
		{location: "prog_8s.mp4#1", wantErr: "is not a relative path"},
		{location: "%zz", wantErr: `location "%zz"`},
		{location: "missing.mp4", wantErrIs: fs.ErrNotExist},
	}
	res := newTestResolver(t)
	for _, c := range cases {
		t.Run(c.location, func(t *testing.T) {
			ra, err := res.ResolveDataEntry(&mp4.URLBox{Location: c.location})
			if c.wantErr != "" || c.wantErrIs != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("got error %q, want it to contain %q", err, c.wantErr)
				}
				if c.wantErrIs != nil && !errors.Is(err, c.wantErrIs) {
					t.Errorf("got error %q, want it to wrap %q", err, c.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(wantStart))
			if _, err := ra.ReadAt(got, 0); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, wantStart) {
				t.Errorf("read %x instead of %x", got, wantStart)
			}
		})
	}
}

// TestLocalFileResolverReuse checks that locations naming the same file give the same open file,
// also after concurrent calls, and that Close closes it.
func TestLocalFileResolverReuse(t *testing.T) {
	res := mp4.NewLocalFileResolver("testdata")
	first, err := res.ResolveDataEntry(&mp4.URLBox{Location: "prog_8s.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, location := range []string{"prog_8s.mp4", "./prog_8s.mp4", "prog%5F8s.mp4", "sub/../prog_8s.mp4"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ra, err := res.ResolveDataEntry(&mp4.URLBox{Location: location})
			if err != nil {
				t.Error(err)
				return
			}
			if ra != first {
				t.Errorf("location %q gave another file", location)
			}
		}()
	}
	wg.Wait()
	if err := res.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.ReadAt(make([]byte, 1), 0); !errors.Is(err, os.ErrClosed) {
		t.Errorf("read after Close: got %v instead of %v", err, os.ErrClosed)
	}
	if _, err := res.ResolveDataEntry(&mp4.URLBox{Location: "prog_8s.mp4"}); err != nil {
		t.Errorf("resolve after Close: %v", err)
	}
	if err := res.Close(); err != nil {
		t.Error(err)
	}
}

// TestLocalFileResolverSymlinks checks that a symbolic link below the directory is followed
// while it stays inside it, and rejected when it leads outside.
func TestLocalFileResolverSymlinks(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "media")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inside.mp4"), []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.mp4")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside.mp4", filepath.Join(dir, "link-in.mp4")); err != nil {
		t.Skipf("cannot create symbolic links: %v", err)
	}
	for name, target := range map[string]string{"link-out.mp4": filepath.Join("..", "outside.mp4"), "abs-out.mp4": outside} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	res := mp4.NewLocalFileResolver(dir)
	defer res.Close()
	ra, err := res.ResolveDataEntry(&mp4.URLBox{Location: "link-in.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("inside"))
	if _, err := ra.ReadAt(got, 0); err != nil {
		t.Fatal(err)
	}
	if string(got) != "inside" {
		t.Errorf("link-in.mp4: read %q instead of %q", got, "inside")
	}
	for _, location := range []string{"link-out.mp4", "abs-out.mp4"} {
		if _, err := res.ResolveDataEntry(&mp4.URLBox{Location: location}); err == nil {
			t.Errorf("%s: expected error for a link that leads outside the directory", location)
		}
	}
}

func TestLocalFileResolverUnsupportedEntry(t *testing.T) {
	res := mp4.NewLocalFileResolver("testdata")
	urn := mp4.CreateUnknownBox("urn ", 16, []byte{0, 0, 0, 0, 'u', 'r', 'n', 0})
	_, err := res.ResolveDataEntry(urn)
	if err == nil || !strings.Contains(err.Error(), `data entry "urn " not supported`) {
		t.Errorf("got %v", err)
	}
}
