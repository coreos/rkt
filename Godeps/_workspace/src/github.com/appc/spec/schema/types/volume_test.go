package types

import (
	"reflect"
	"testing"
)

func TestVolumeFromString(t *testing.T) {
	tests := []struct {
		s string
		v Volume
	}{
		{
			"foobar,kind=host,source=/tmp",
			Volume{
				Name:     "foobar",
				Kind:     "host",
				Source:   "/tmp",
				ReadOnly: false,
			},
		},
		{
			"foobar,kind=host,source=/tmp,readOnly=true",
			Volume{
				Name:     "foobar",
				Kind:     "host",
				Source:   "/tmp",
				ReadOnly: true,
			},
		},
		{
			"foobar,kind=empty",
			Volume{
				Name: "foobar",
				Kind: "empty",
			},
		},
		{
			"foobar,kind=empty,readOnly=true",
			Volume{
				Name:     "foobar",
				Kind:     "empty",
				ReadOnly: true,
			},
		},
	}
	for i, tt := range tests {
		v, err := VolumeFromString(tt.s)
		if err != nil {
			t.Errorf("#%d: got err=%v, want nil", i, err)
		}
		if !reflect.DeepEqual(*v, tt.v) {
			t.Errorf("#%d: v=%v, want %v", i, *v, tt.v)
		}
	}
}

func TestVolumeFromStringBad(t *testing.T) {
	tests := []string{
		"#foobar,kind=host,source=/tmp",
		"foobar,kind=host,source=/tmp,readOnly=true,asdf=asdf",
		"foobar,kind=empty,source=/tmp",
	}
	for i, in := range tests {
		l, err := VolumeFromString(in)
		if l != nil {
			t.Errorf("#%d: got l=%v, want nil", i, l)
		}
		if err == nil {
			t.Errorf("#%d: got err=nil, want non-nil", i)
		}
	}
}
