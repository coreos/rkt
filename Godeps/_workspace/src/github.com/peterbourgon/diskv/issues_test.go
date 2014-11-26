package diskv

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"
)

// ReadStream from cache shouldn't panic on a nil dereference from a nonexistent
// Compression :)
func TestIssue2A(t *testing.T) {
	d := New(Options{
		BasePath:     "test-issue-2a",
		Transform:    func(string) []string { return []string{} },
		CacheSizeMax: 1024,
	})
	defer d.EraseAll()

	input := "abcdefghijklmnopqrstuvwxy"
	key, writeBuf, sync := "a", bytes.NewBufferString(input), false
	if err := d.WriteStream(key, writeBuf, sync); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		began := time.Now()
		rc, err := d.ReadStream(key, false)
		if err != nil {
			t.Fatal(err)
		}
		buf, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if !cmpBytes(buf, []byte(input)) {
			t.Fatalf("read #%d: '%s' != '%s'", i+1, string(buf), input)
		}
		rc.Close()
		t.Logf("read #%d in %s", i+1, time.Since(began))
	}
}

// ReadStream on a key that resolves to a directory should return an error.
func TestIssue2B(t *testing.T) {
	blockTransform := func(s string) []string {
		transformBlockSize := 3
		sliceSize := len(s) / transformBlockSize
		pathSlice := make([]string, sliceSize)
		for i := 0; i < sliceSize; i++ {
			from, to := i*transformBlockSize, (i*transformBlockSize)+transformBlockSize
			pathSlice[i] = s[from:to]
		}
		return pathSlice
	}

	d := New(Options{
		BasePath:     "test-issue-2b",
		Transform:    blockTransform,
		CacheSizeMax: 0,
	})
	defer d.EraseAll()

	v := []byte{'1', '2', '3'}
	if err := d.Write("abcabc", v); err != nil {
		t.Fatal(err)
	}

	_, err := d.ReadStream("abc", false)
	if err == nil {
		t.Fatal("ReadStream('abc') should return error")
	}
	t.Logf("ReadStream('abc') returned error: %v", err)
}
