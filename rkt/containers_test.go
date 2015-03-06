package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/rocket/pkg/lock"
)

func TestWalkContainers(t *testing.T) {
	tests := [][]*struct {
		uuid      string
		exited    bool
		garbage   bool
		deleting  bool
		expected  bool
		n_matched int
	}{
		{ // nothing
		},
		{ // single executing container
			{
				uuid:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				exited:   false,
				garbage:  false,
				deleting: false,

				expected: true,
			},
		},
		{ // single exited container
			{
				uuid:     "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
				exited:   true,
				garbage:  false,
				deleting: false,

				expected: true,
			},
		},
		{ // single garbage container
			{
				uuid:     "cccccccc-cccc-cccc-cccc-cccccccccccc",
				exited:   true,
				garbage:  true,
				deleting: false,

				expected: true,
			},
		},
		{ // single deleting container
			{
				uuid:     "dddddddd-dddd-dddd-dddd-dddddddddddd",
				exited:   true,
				garbage:  true,
				deleting: true,

				expected: true,
			},
		},
		{ // one of each
			{ // executing
				uuid:     "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
				exited:   false,
				garbage:  false,
				deleting: false,

				expected: true,
			},
			{ // exited
				uuid:     "ffffffff-ffff-ffff-ffff-ffffffffffff",
				exited:   true,
				garbage:  false,
				deleting: false,

				expected: true,
			},
			{ // garbage
				uuid:     "f0f0f0f0-f0f0-f0f0-f0f0-f0f0f0f0f0f0",
				exited:   true,
				garbage:  true,
				deleting: false,

				expected: true,
			},
			{ // deleting
				uuid:     "f1f1f1f1-f1f1-f1f1-f1f1-f1f1f1f1f1f1",
				exited:   true,
				garbage:  true,
				deleting: true,

				expected: true,
			},
		},
		{ // faked rename race, same uuid as both garbage and non-garbage, only expect the garbage
			{ // exited
				uuid:     "f2f2f2f2-f2f2-f2f2-f2f2-f2f2f2f2f2f2",
				exited:   true,
				garbage:  false,
				deleting: false,

				expected: false,
			},
			{ // garbage
				uuid:     "f2f2f2f2-f2f2-f2f2-f2f2-f2f2f2f2f2f2",
				exited:   true,
				garbage:  true,
				deleting: false,

				expected: true,
			},
		},
	}

	for _, tt := range tests {
		// start every test with a clean slate
		d, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatalf("error creating tmpdir: %v", err)
		}
		defer os.RemoveAll(d)

		globalFlags.Dir = d
		if err := os.MkdirAll(containersDir(), 0700); err != nil {
			t.Fatalf("error creating container directory: %v", err)
		}
		if err := os.MkdirAll(garbageDir(), 0700); err != nil {
			t.Fatalf("error creating garbage directory: %v", err)
		}

		var (
			n_expected int
			n_walked   int
			n_matched  int
			included   includeMask
		)

		// create the container dirs as specified by the test
		for _, ct := range tt {
			var cp string
			if ct.garbage {
				cp = filepath.Join(garbageDir(), ct.uuid)
				included |= includeGarbageDir
			} else {
				cp = filepath.Join(containersDir(), ct.uuid)
				included |= includeContainersDir
			}

			if err := os.MkdirAll(cp, 0700); err != nil {
				t.Fatalf("error creating container directory: %v", err)
			}

			if !ct.exited || ct.deleting { // acquire lock to simulate running and deleting containers
				l, err := lock.ExclusiveDirLock(cp)
				if err != nil {
					t.Fatalf("error locking container: %v", err)
				}
				defer l.Close()
			}

			if ct.expected {
				n_expected++
			}
		}

		// match what walk provided against the set in the test
		if err := walkContainers(included, func(ch *container) {
			n_walked++
			for _, ct := range tt {
				if ch.uuid == ct.uuid &&
					ch.isGarbage == ct.garbage &&
					ch.isExited == ct.exited &&
					ch.isDeleting == ct.deleting {

					ct.n_matched++
					if ct.n_matched > 1 {
						t.Errorf("no containers should match multiple times")
					}
					n_matched++
				}
			}
		}); err != nil {
			t.Fatalf("error walking containers: %v", err)
		}

		if n_expected != n_matched {
			t.Errorf("walked: %d expected: %d matched: %d", n_walked, n_expected, n_matched)
		}

		for _, ct := range tt {
			if ct.expected && ct.n_matched == 0 {
				t.Errorf("container %q expected but not matched", ct.uuid)
			}

			if !ct.expected && ct.n_matched != 0 {
				t.Errorf("container %q matched but not expected", ct.uuid)
			}
		}
	}

}
