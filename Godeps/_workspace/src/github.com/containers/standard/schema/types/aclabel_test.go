package types

import "testing"

func TestNewACLabel(t *testing.T) {
	tests := []string{
		"asdf",
		"foo-bar-baz",
		"database",
		"example.com/database",
		"example.com/ourapp-1.0.0",
		"sub-domain.example.com/org/product/release-1.0.0",
	}
	for i, in := range tests {
		l, err := NewACLabel(in)
		if err != nil {
			t.Errorf("#%d: got err=%v, want nil", i, err)
		}
		if l == nil {
			t.Errorf("#%d: got l=nil, want non-nil", i)
		}
	}
}

func TestNewACLabelBad(t *testing.T) {
	tests := []string{
		"foo#",
		"EXAMPLE.com",
		"foo.com/BAR",
		"example.com/app_1",
	}
	for i, in := range tests {
		l, err := NewACLabel(in)
		if l != nil {
			t.Errorf("#%d: got l=%v, want nil", i, l)
		}
		if err == nil {
			t.Errorf("#%d: got err=nil, want non-nil", i)
		}
	}
}
