package ioprogress

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
	"time"
)

func TestReader(t *testing.T) {
	testR := &testReader{
		Data: [][]byte{
			[]byte("ab"),
			[]byte("cd"),
			[]byte("ef"),
		},
	}

	var buf bytes.Buffer
	r := &Reader{
		Reader:       testR,
		Size:         testR.Size(),
		DrawFunc:     DrawTerminal(&buf),
		DrawInterval: time.Microsecond,
	}
	io.Copy(ioutil.Discard, r)

	if buf.String() != drawReaderStr {
		t.Fatalf("bad:\n\n%#v", buf.String())
	}
}

const drawReaderStr = "0/6\r2/6\r4/6\r6/6\r6/6\r\n"

// testReader is a test structure to help with testing the Reader by
// returning fixed slices of data.
type testReader struct {
	Data [][]byte
	i    int
}

func (r *testReader) Read(p []byte) (int, error) {
	// This is just so that our interval will fire properly
	time.Sleep(5 * time.Microsecond)

	if r.i == len(r.Data) {
		return 0, io.EOF
	}

	copy(p, r.Data[r.i])
	r.i += 1
	return len(r.Data[r.i-1]), nil
}

func (r *testReader) Size() (n int64) {
	for _, d := range r.Data {
		n += int64(len(d))
	}

	return
}
