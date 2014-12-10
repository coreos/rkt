package cas

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"

	"github.com/appc/spec/aci"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/peterbourgon/diskv"
)

// TODO(philips): use a database for the secondary indexes like remoteType and
// appType. This is OK for now though.
const (
	blobType int64 = iota
	remoteType
	tmpType
)

var otmap = [...]string{
	"blob",
	"remote",
	"tmp",
}

type Store struct {
	stores []*diskv.Diskv
}

func NewStore(base string) *Store {
	ds := &Store{}
	ds.stores = make([]*diskv.Diskv, len(otmap))

	for i, p := range otmap {
		ds.stores[i] = diskv.New(diskv.Options{
			BasePath:     filepath.Join(base, "cas", p),
			Transform:    blockTransform,
			CacheSizeMax: 1024 * 1024, // 1MB
		})
	}

	return ds
}

func (ds Store) ReadStream(key string) (io.ReadCloser, error) {
	return ds.stores[blobType].ReadStream(key, false)
}

func (ds Store) WriteStream(key string, r io.Reader) error {
	return ds.stores[blobType].WriteStream(key, r, true)
}

func (ds Store) WriteACI(tmpKey string, orig io.Reader) (string, error) {
	// Peek at the first 512 bytes of the reader to detect filetype
	br := bufio.NewReaderSize(orig, 512)
	hd, err := br.Peek(512)
	switch err {
	case nil:
	case io.EOF: // We may have still peeked enough to guess some types, so fall through
	default:
		return "", err
	}
	typ, err := aci.DetectFileType(bytes.NewBuffer(hd))
	if err != nil {
		return "", err
	}
	dr, err := decompress(br, typ)
	if err != nil {
		return "", err
	}
	// Write the uncompressed image (tar) into the store, and tee so we can generate the hash
	hash := sha256.New()
	tr := io.TeeReader(dr, hash)
	err = ds.stores[tmpType].WriteStream(tmpKey, tr, true)
	if err != nil {
		return "", err
	}

	// Store the decompressed tar using the hash as the real key
	rs, err := ds.stores[tmpType].ReadStream(tmpKey, false)
	if err != nil {
		return "", err
	}
	defer rs.Close()

	key := fmt.Sprintf("sha256-%x", hash.Sum(nil))
	err = ds.stores[blobType].WriteStream(key, rs, true)
	if err != nil {
		return "", err
	}

	ds.stores[tmpType].Erase(tmpKey)

	return key, nil
}

type Index interface {
	Hash() string
	Marshal() []byte
	Unmarshal([]byte)
	Type() int64
}

func (ds Store) WriteIndex(i Index) {
	ds.stores[i.Type()].Write(i.Hash(), i.Marshal())
}

func (ds Store) ReadIndex(i Index) error {
	buf, err := ds.stores[i.Type()].Read(i.Hash())
	if err != nil {
		return err
	}

	i.Unmarshal(buf)

	return nil
}

func (ds Store) Dump(hex bool) {
	for _, s := range ds.stores {
		var keyCount int
		for key := range s.Keys() {
			val, err := s.Read(key)
			if err != nil {
				panic(fmt.Sprintf("key %s had no value", key))
			}
			if len(val) > 128 {
				val = val[:128]
			}
			out := string(val)
			if hex {
				out = fmt.Sprintf("%x", val)
			}
			fmt.Printf("%s/%s: %s\n", s.BasePath, key, out)
			keyCount++
		}
		fmt.Printf("%d total keys\n", keyCount)
	}
}
