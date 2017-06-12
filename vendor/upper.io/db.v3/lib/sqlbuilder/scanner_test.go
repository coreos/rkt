package sqlbuilder

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringArrayScanner(t *testing.T) {

	testCases := map[string][]string{
		`{"","","","","\"\""}`:       []string{"", "", "", "", `""`},
		`{""}`:                       []string{""},
		`{}`:                         []string{},
		`{x}`:                        []string{"x"},
		`{x,"y"}`:                    []string{"x", "y"},
		`{x, "y"  }`:                 []string{"x", "y"},
		``:                           []string(nil),
		`{a,bb,"ccc"}`:               []string{"a", "bb", "ccc"},
		`{a,bb,"ccc","\""}`:          []string{"a", "bb", "ccc", `"`},
		`{a, bb,  "c cc","\"", " "}`: []string{"a", "bb", "c cc", `"`, ` `},
	}

	for input, output := range testCases {
		var s stringArray
		err := s.Scan([]byte(input))
		assert.NoError(t, err)

		assert.Equal(t, output, []string(s), fmt.Sprintf("input was: %s", input))
	}

}
