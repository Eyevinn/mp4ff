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
// directory. It opens files through an [os.Root], so a symbolic link that leads outside the directory
// is rejected too. The directory and the opened files stay open for later calls until Close.
// It is safe for concurrent use.
type LocalFileResolver struct {
	dir   string
	mu    sync.Mutex
	root  *os.Root
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
	if r.root == nil {
		root, err := os.OpenRoot(r.dir)
		if err != nil {
			return nil, err
		}
		r.root = root
	}
	f, err := r.root.Open(name)
	if err != nil {
		return nil, err
	}
	if r.files == nil {
		r.files = make(map[string]*os.File)
	}
	r.files[name] = f
	return f, nil
}

// Close closes the files and the directory that the resolver has opened.
func (r *LocalFileResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for _, f := range r.files {
		errs = append(errs, f.Close())
	}
	r.files = nil
	if r.root != nil {
		errs = append(errs, r.root.Close())
		r.root = nil
	}
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
