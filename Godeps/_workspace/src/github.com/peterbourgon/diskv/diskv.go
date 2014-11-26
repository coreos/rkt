// Diskv (disk-vee) is a simple, persistent, key-value store.
// It stores all data flatly on the filesystem.

package diskv

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
)

const (
	defaultBasePath             = "diskv"
	defaultFilePerm os.FileMode = 0666
	defaultPathPerm os.FileMode = 0777
)

var (
	defaultTransform = func(s string) []string { return []string{} }
)

// TransformFunction transforms a key into a slice of strings, with each
// element in the slice representing a directory in the file path where the
// key's entry will eventually be stored.
//
// For example, if TransformFunc transforms "abcdef" to ["ab", "cde", "f"],
// the final location of the data file will be <basedir>/ab/cde/f/abcdef
type TransformFunction func(s string) []string

// Options define a set of properties that dictate Diskv behavior.
// All values are optional.
type Options struct {
	BasePath     string
	Transform    TransformFunction
	CacheSizeMax uint64 // bytes
	PathPerm     os.FileMode
	FilePerm     os.FileMode

	Index     Index
	IndexLess LessFunction

	Compression Compression
}

// Diskv implements the Diskv interface. You shouldn't construct Diskv
// structures directly; instead, use the New constructor.
type Diskv struct {
	sync.RWMutex
	Options
	cache     map[string][]byte
	cacheSize uint64
}

// New returns an initialized Diskv structure, ready to use.
// If the path identified by baseDir already contains data,
// it will be accessible, but not yet cached.
func New(options Options) *Diskv {
	if options.BasePath == "" {
		options.BasePath = defaultBasePath
	}
	if options.Transform == nil {
		options.Transform = defaultTransform
	}
	if options.PathPerm == 0 {
		options.PathPerm = defaultPathPerm
	}
	if options.FilePerm == 0 {
		options.FilePerm = defaultFilePerm
	}

	d := &Diskv{
		Options:   options,
		cache:     map[string][]byte{},
		cacheSize: 0,
	}

	if d.Index != nil && d.IndexLess != nil {
		d.Index.Initialize(d.IndexLess, d.Keys())
	}

	return d
}

// Write synchronously writes the key-value pair to disk, making it immediately
// available for reads. Write relies on the filesystem to perform an eventual
// sync to physical media. If you need stronger guarantees, see WriteStream.
func (d *Diskv) Write(key string, val []byte) error {
	return d.write(key, bytes.NewBuffer(val), false)
}

// WriteStream writes the data represented by the io.Reader to the disk, under
// the provided key. If sync is true, WriteStream performs an explicit sync on
// the file as soon as it's written.
//
// bytes.Buffer provides io.Reader semantics for basic data types.
func (d *Diskv) WriteStream(key string, r io.Reader, sync bool) error {
	return d.write(key, r, sync)
}

// write synchronously writes the key-value pair to disk,
// making it immediately available for reads. write optionally
// performs a Sync on the relevant file descriptor.
func (d *Diskv) write(key string, r io.Reader, sync bool) error {
	if len(key) <= 0 {
		return fmt.Errorf("empty key")
	}

	// TODO use atomic FS ops in write()

	d.Lock()
	defer d.Unlock()
	if err := d.ensurePath(key); err != nil {
		return fmt.Errorf("ensure path: %s", err)
	}

	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC // overwrite if exists
	f, err := os.OpenFile(d.completeFilename(key), mode, d.FilePerm)
	if err != nil {
		return fmt.Errorf("open file: %s", err)
	}

	var wc = io.WriteCloser(&nopWriteCloser{f})
	if d.Compression != nil {
		wc, err = d.Compression.Writer(f)
		if err != nil {
			f.Close() // error deliberately ignored
			return fmt.Errorf("compression writer: %s", err)
		}
	}

	if _, err := io.Copy(wc, r); err != nil {
		f.Close() // error deliberately ignored
		return fmt.Errorf("i/o copy: %s", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("compression close: %s", err)
	}

	if sync {
		if err := f.Sync(); err != nil {
			f.Close() // error deliberately ignored
			return fmt.Errorf("file sync: %s", err)
		}
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("file close: %s", err)
	}

	if d.Index != nil {
		d.Index.Insert(key)
	}

	delete(d.cache, key) // cache only on read
	return nil
}

// Read reads the key and returns the value.
// If the key is available in the cache, Read won't touch the disk.
// If the key is not in the cache, Read will have the side-effect of
// lazily caching the value.
func (d *Diskv) Read(key string) ([]byte, error) {
	rc, err := d.ReadStream(key, false)
	if err != nil {
		return []byte{}, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

// ReadStream reads the key and returns the value (data) as an io.ReadCloser.
// If the value is cached from a previous read, and direct is false,
// ReadStream will use the cached value. Otherwise, it will return a handle to
// the file on disk, and cache the data on read.
//
// If direct is true, ReadStream will always delete any cached value for the
// key, and return a direct handle to the file on disk.
//
// ReadStream taps into the io.Reader stream prior to decompression, and
// caches the compressed data.
func (d *Diskv) ReadStream(key string, direct bool) (io.ReadCloser, error) {
	d.RLock()
	defer d.RUnlock()

	if val, ok := d.cache[key]; ok {
		if direct {
			d.cacheSize -= uint64(len(val))
			delete(d.cache, key)
		} else {
			buf := bytes.NewBuffer(val)
			if d.Compression != nil {
				return d.Compression.Reader(buf)
			}
			return ioutil.NopCloser(buf), nil
		}
	}

	return d.read(key)
}

// read ignores the cache, and returns an io.ReadCloser representing the
// decompressed data for the given key, streamed from the disk. Clients should
// acquire a read lock on the Diskv and check the cache themselves before
// calling read.
func (d *Diskv) read(key string) (io.ReadCloser, error) {
	filename := d.completeFilename(key)

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, os.ErrNotExist
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	r := newSiphon(f, d, key)

	var rc = io.ReadCloser(ioutil.NopCloser(r))
	if d.Compression != nil {
		rc, err = d.Compression.Reader(r)
		if err != nil {
			return nil, err
		}
	}

	return rc, nil
}

// siphon is like a TeeReader: it copies all data read through it to an
// internal buffer, and moves that buffer to the cache at EOF.
type siphon struct {
	f   *os.File
	d   *Diskv
	key string
	buf *bytes.Buffer
}

// newSiphon constructs a siphoning reader that represents the passed file.
// When a successful series of reads ends in an EOF, the siphon will write
// the buffered data to Diskv's cache under the given key.
func newSiphon(f *os.File, d *Diskv, key string) io.Reader {
	return &siphon{
		f:   f,
		d:   d,
		key: key,
		buf: &bytes.Buffer{},
	}
}

// Read implements the io.Reader interface for siphon.
func (s *siphon) Read(p []byte) (int, error) {
	n, err := s.f.Read(p)

	if err == nil {
		return s.buf.Write(p[0:n]) // Write must succeed for Read to succeed
	}

	if err == io.EOF {
		s.d.cacheWithoutLock(s.key, s.buf.Bytes()) // cache may fail
		if closeErr := s.f.Close(); closeErr != nil {
			return n, closeErr // close must succeed for Read to succeed
		}
		return n, err
	}

	return n, err
}

// Erase synchronously erases the given key from the disk and the cache.
func (d *Diskv) Erase(key string) error {
	d.Lock()
	defer d.Unlock()

	// erase from cache
	if val, ok := d.cache[key]; ok {
		d.cacheSize -= uint64(len(val))
		delete(d.cache, key)
	}

	// erase from index
	if d.Index != nil {
		d.Index.Delete(key)
	}

	// erase from disk
	filename := d.completeFilename(key)
	if s, err := os.Stat(filename); err == nil {
		if !!s.IsDir() {
			return fmt.Errorf("bad key")
		}
		if err = os.Remove(filename); err != nil {
			return err
		}
	} else {
		return err
	}

	// clean up and return
	d.pruneDirs(key)
	return nil
}

// EraseAll will delete all of the data from the store, both in the cache and on
// the disk. Note that EraseAll doesn't distinguish diskv-related data from non-
// diskv-related data. Care should be taken to always specify a diskv base
// directory that is exclusively for diskv data.
func (d *Diskv) EraseAll() error {
	d.Lock()
	defer d.Unlock()
	d.cache = make(map[string][]byte)
	d.cacheSize = 0
	return os.RemoveAll(d.BasePath)
}

// Has returns true if the given key exists.
func (d *Diskv) Has(key string) bool {
	d.Lock()
	defer d.Unlock()

	if _, ok := d.cache[key]; ok {
		return true
	}

	filename := d.completeFilename(key)
	s, err := os.Stat(filename)
	if err != nil {
		return false
	}
	if s.IsDir() {
		return false
	}

	return true
}

// Keys returns a channel that will yield every key accessible by the store in
// undefined order.
func (d *Diskv) Keys() <-chan string {
	c := make(chan string)
	go func() {
		filepath.Walk(d.BasePath, walker(c))
		close(c)
	}()
	return c
}

// walker returns a function which satisfies the filepath.WalkFunc interface.
// It sends every non-directory file entry down the channel c.
func walker(c chan string) func(path string, info os.FileInfo, err error) error {
	return func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			c <- info.Name()
		}
		return nil // "pass"
	}
}

// pathFor returns the absolute path for location on the filesystem where the
// data for the given key will be stored.
func (d *Diskv) pathFor(key string) string {
	return path.Join(d.BasePath, path.Join(d.Transform(key)...))
}

// ensureDir is a helper function that generates all necessary directories on
// the filesystem for the given key.
func (d *Diskv) ensurePath(key string) error {
	return os.MkdirAll(d.pathFor(key), d.PathPerm)
}

// completeFilename returns the absolute path to the file for the given key.
func (d *Diskv) completeFilename(key string) string {
	return fmt.Sprintf("%s%c%s", d.pathFor(key), os.PathSeparator, key)
}

// cacheWithLock attempts to cache the given key-value pair in the store's
// cache. It can fail if the value is larger than the cache's maximum size.
func (d *Diskv) cacheWithLock(key string, val []byte) error {
	valueSize := uint64(len(val))
	if err := d.ensureCacheSpaceFor(valueSize); err != nil {
		return fmt.Errorf("%s; not caching", err)
	}

	// be very strict about memory guarantees
	if (d.cacheSize + valueSize) > d.CacheSizeMax {
		panic(
			fmt.Sprintf(
				"failed to make room for value (%d/%d)",
				valueSize,
				d.CacheSizeMax,
			),
		)
	}

	d.cache[key] = val
	d.cacheSize += valueSize
	return nil
}

// cacheWithoutLock acquires the store's (write) mutex and calls cacheWithLock.
func (d *Diskv) cacheWithoutLock(key string, val []byte) error {
	d.Lock()
	defer d.Unlock()
	return d.cacheWithLock(key, val)
}

// pruneDirs deletes empty directories in the path walk leading to the key k.
// Typically this function is called after an Erase is made.
func (d *Diskv) pruneDirs(key string) error {
	pathlist := d.Transform(key)
	for i := range pathlist {
		pslice := pathlist[:len(pathlist)-i]
		dir := path.Join(d.BasePath, path.Join(pslice...))

		// thanks to Steven Blenkinsop for this snippet
		switch fi, err := os.Stat(dir); true {
		case err != nil:
			return err
		case !fi.IsDir():
			panic(fmt.Sprintf("corrupt dirstate at %s", dir))
		}

		nlinks, err := filepath.Glob(fmt.Sprintf("%s%c*", dir, os.PathSeparator))
		if err != nil {
			return err
		} else if len(nlinks) > 0 {
			return nil // has subdirs -- do not prune
		}
		if err = os.Remove(dir); err != nil {
			return err
		}
	}

	return nil
}

// ensureCacheSpaceFor deletes entries from the cache in arbitrary order until
// the cache has at least valueSize bytes available.
func (d *Diskv) ensureCacheSpaceFor(valueSize uint64) error {
	if valueSize > d.CacheSizeMax {
		return fmt.Errorf(
			"value size (%d bytes) too large for cache (%d bytes)",
			valueSize,
			d.CacheSizeMax,
		)
	}

	safe := func() bool { return (d.cacheSize + valueSize) <= d.CacheSizeMax }
	for key, val := range d.cache {
		if safe() {
			break
		}
		delete(d.cache, key)            // delete is safe, per spec
		d.cacheSize -= uint64(len(val)) // len should return uint :|
	}
	if !safe() {
		panic(fmt.Sprintf(
			"%d bytes still won't fit in the cache! (max %d bytes)",
			valueSize,
			d.CacheSizeMax,
		))
	}

	return nil
}

// nopWriteCloser wraps an io.Writer and provides a no-op Close method to
// satisfy the io.WriteCloser interface.
type nopWriteCloser struct {
	w io.Writer
}

func (wc *nopWriteCloser) Write(p []byte) (int, error) { return wc.w.Write(p) }
func (wc *nopWriteCloser) Close() error                { return nil }
