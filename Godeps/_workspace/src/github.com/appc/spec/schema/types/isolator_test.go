package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIsolatorUnmarshal(t *testing.T) {
	tests := []struct {
		msg       string
		werr      bool
		werrvalid bool
	}{
		{
			`{
				"name": "os/linux/capabilities-retain-set",
				"value": {"set": ["CAP_KILL"]}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "os/linux/capabilities-retain-set",
				"value": {"set": ["CAP_PONIES"]}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "os/linux/capabilities-retain-set",
				"value": {"set": []}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "os/linux/capabilities-retain-set",
				"value": {"set": "CAP_PONIES"}
			}`,
			true,
			true,
		},
		{
			`{
				"name": "resource/block-bandwidth",
				"value": {"default": true, "limit": "1G"}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "resource/block-bandwidth",
				"value": {"default": false, "limit": "1G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/block-bandwidth",
				"value": {"request": "30G", "limit": "1G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/block-iops",
				"value": {"default": true, "limit": "1G"}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "resource/block-iops",
				"value": {"default": false, "limit": "1G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/block-iops",
				"value": {"request": "30G", "limit": "1G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/cpu",
				"value": {"request": "30", "limit": "1"}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "resource/memory",
				"value": {"request": "1G", "limit": "2Gi"}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "resource/memory",
				"value": {"default": true, "request": "1G", "limit": "2G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/network-bandwidth",
				"value": {"default": true, "limit": "1G"}
			}`,
			false,
			false,
		},
		{
			`{
				"name": "resource/network-bandwidth",
				"value": {"default": false, "limit": "1G"}
			}`,
			false,
			true,
		},
		{
			`{
				"name": "resource/network-bandwidth",
				"value": {"request": "30G", "limit": "1G"}
			}`,
			false,
			true,
		},
	}

	for i, tt := range tests {
		var ii Isolator
		err := ii.UnmarshalJSON([]byte(tt.msg))
		if gerr := (err != nil); gerr != tt.werr {
			t.Errorf("#%d: gerr=%t, want %t (err=%v)", i, gerr, tt.werr, err)
		}
		err = ii.assertValid()
		if gerrvalid := (err != nil); gerrvalid != tt.werrvalid {
			t.Errorf("#%d: name=%v gerrvalid=%t, want %t (err=%v)", i, ii.Name, gerrvalid, tt.werrvalid, err)
		}
	}
}

func TestIsolatorsGetByName(t *testing.T) {
	ex := `
		[
			{
				"name": "resource/cpu",
				"value": {"request": "30", "limit": "1"}
			},
			{
				"name": "resource/memory",
				"value": {"request": "1G", "limit": "2Gi"}
			},
			{
				"name": "os/linux/capabilities-retain-set",
				"value": {"set": ["CAP_KILL"]}
			},
			{
				"name": "os/linux/capabilities-revoke-set",
				"value": {"set": ["CAP_KILL"]}
			}
		]
	`

	tests := []struct {
		name     string
		wlimit   int64
		wrequest int64
		wset     []LinuxCapability
	}{
		{"resource/cpu", 1, 30, nil},
		{"resource/memory", 2147483648, 1000000000, nil},
		{"os/linux/capabilities-retain-set", 0, 0, []LinuxCapability{"CAP_KILL"}},
		{"os/linux/capabilities-revoke-set", 0, 0, []LinuxCapability{"CAP_KILL"}},
	}

	var is Isolators
	err := json.Unmarshal([]byte(ex), &is)
	if err != nil {
		panic(err)
	}

	if len(is) < 2 {
		t.Fatalf("too few items %v", len(is))
	}

	for i, tt := range tests {
		c := is.GetByName(tt.name)
		if c == nil {
			t.Fatalf("can't find item %v in %v items", tt.name, len(is))
		}
		switch v := c.Value().(type) {
		case Resource:
			var r Resource = v
			glimit := r.Limit()
			grequest := r.Request()
			if glimit.Value() != tt.wlimit || grequest.Value() != tt.wrequest {
				t.Errorf("#%d: glimit=%v, want %v, grequest=%v, want %v", i, glimit.Value(), tt.wlimit, grequest.Value(), tt.wrequest)
			}
		case LinuxCapabilitiesSet:
			var s LinuxCapabilitiesSet = v
			if !reflect.DeepEqual(s.Set(), tt.wset) {
				t.Errorf("#%d: gset=%v, want %v", i, s.Set(), tt.wset)
			}

		default:
			panic("unexecpected type")
		}
	}
}
