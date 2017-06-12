package awstesting

// ZeroReader is a io.Reader which will always write zeros to the byte slice provided.
type ZeroReader struct{}

// Read fills the provided byte slice with zeros returning the number of bytes written.
func (r *ZeroReader) Read(b []byte) (int, error) {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
	return len(b), nil
}
