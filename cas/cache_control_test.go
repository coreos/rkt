package cas

import (
	"testing"
	"time"
)

func TestCacheControl(t *testing.T) {
	cc1 := NewCache("max-age=10")
	if cc1.MaxAge != 10 {
		t.Errorf("expected max-age header argument for cache-control")
	}
	cc1 = NewCache("no-cache")
	if cc1.MaxAge != 0 {
		t.Errorf("expected max-age to be 0")
	}
	cc1 = NewCache("no-store")
	if cc1.MaxAge != 0 {
		t.Errorf("expected max-age to be 0")
	}

	cc1 = NewCache("max-age=10")
	// Sleep during 11 seconds
	time.Sleep(11000 * time.Millisecond)
	if cc1.UseCachedImage() {
		t.Errorf("expected max-age header argument to be expired after sleep time")
	}
}
