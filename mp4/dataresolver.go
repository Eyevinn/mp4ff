package mp4

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// DataResolver gives access to media data that a track's dref box places outside the file,
// such as in the file that a url entry names. Unified Streaming dref MP4 files and files made
// with MP4Box -dref have such entries. The chunk offsets of the samples that use an entry are
// offsets into the data it refers to.
type DataResolver interface {
	// ResolveDataEntry returns the data that entry, a child of a dref box, refers to.
	// It is only called for entries that do not say that the data is in the same file.
	ResolveDataEntry(entry Box) (io.ReaderAt, error)
}

// LocalFileResolver is a DataResolver for url entries whose location is a relative path,
// which it opens as a file below its directory, typically the directory of the file with the moov box.
// It rejects locations with a URL scheme or host, absolute paths, and paths that lead outside the
// directory. The check is lexical, so a symbolic link below the directory is followed.
// Opened files stay open for later calls until Close. It is safe for concurrent use.
type LocalFileResolver struct {
	dir   string
	mu    sync.Mutex
	files map[string]*os.File
}

// NewLocalFileResolver returns a LocalFileResolver for files below dir.
func NewLocalFileResolver(dir string) *LocalFileResolver {
	return &LocalFileResolver{dir: dir}
}

// ResolveDataEntry opens the file that a url entry names, or returns the file opened before.
func (r *LocalFileResolver) ResolveDataEntry(entry Box) (io.ReaderAt, error) {
	u, ok := entry.(*URLBox)
	if !ok {
		return nil, fmt.Errorf("data entry %q not supported", entry.Type())
	}
	name, err := localFilePath(u.Location)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.files[name]; ok {
		return f, nil
	}
	f, err := os.Open(filepath.Join(r.dir, name))
	if err != nil {
		return nil, err
	}
	if r.files == nil {
		r.files = make(map[string]*os.File)
	}
	r.files[name] = f
	return f, nil
}

// Close closes the files that the resolver has opened.
func (r *LocalFileResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for _, f := range r.files {
		errs = append(errs, f.Close())
	}
	r.files = nil
	return errors.Join(errs...)
}

// localFilePath returns the relative file path that the location of a url entry names.
// The location is a URL, so a relative reference is percent-decoded into a path.
func localFilePath(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("location %q: %w", location, err)
	}
	if u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("location %q is not a relative path", location)
	}
	p := filepath.FromSlash(u.Path)
	if !filepath.IsLocal(p) {
		return "", fmt.Errorf("location %q is not a path inside the directory", location)
	}
	return filepath.Clean(p), nil
}
