package exql

import (
	"strings"
)

const (
	stageExpect = iota
	stageCapture
	stageClose
)

// isBlankSymbol returns true if the given byte is either space, tab, carriage
// return or newline.
func isBlankSymbol(in byte) bool {
	return in == ' ' || in == '\t' || in == '\r' || in == '\n'
}

// trimString returns a slice of s with a leading and trailing blank symbols
// (as defined by isBlankSymbol) removed.
func trimString(s string) string {

	// This conversion is rather slow.
	// return string(trimBytes([]byte(s)))

	start, end := 0, len(s)-1

	if end < start {
		return ""
	}

	for isBlankSymbol(s[start]) {
		start++
		if start >= end {
			return ""
		}
	}

	for isBlankSymbol(s[end]) {
		end--
	}

	return s[start : end+1]
}

// trimBytes returns a slice of s with a leading and trailing blank symbols (as
// defined by isBlankSymbol) removed.
func trimBytes(s []byte) []byte {

	start, end := 0, len(s)-1

	if end < start {
		return []byte{}
	}

	for isBlankSymbol(s[start]) {
		start++
		if start >= end {
			return []byte{}
		}
	}

	for isBlankSymbol(s[end]) {
		end--
	}

	return s[start : end+1]
}

/*
// Separates by a comma, ignoring spaces too.
// This was slower than strings.Split.
func separateByComma(in string) (out []string) {

	out = []string{}

	start, lim := 0, len(in)-1

	for start < lim {
		var end int

		for end = start; end <= lim; end++ {
			// Is a comma?
			if in[end] == ',' {
				break
			}
		}

		out = append(out, trimString(in[start:end]))

		start = end + 1
	}

	return
}
*/

// Separates by a comma, ignoring spaces too.
func separateByComma(in string) (out []string) {
	out = strings.Split(in, ",")
	for i := range out {
		out[i] = trimString(out[i])
	}
	return
}

// Separates by spaces, ignoring spaces too.
func separateBySpace(in string) (out []string) {
	if len(in) == 0 {
		return []string{""}
	}

	pre := strings.Split(in, " ")
	out = make([]string, 0, len(pre))

	for i := range pre {
		pre[i] = trimString(pre[i])
		if pre[i] != "" {
			out = append(out, pre[i])
		}
	}

	return
}

func separateByAS(in string) (out []string) {
	out = []string{}

	if len(in) < 6 {
		// The minimum expression with the AS keyword is "x AS y", 6 chars.
		return []string{in}
	}

	start, lim := 0, len(in)-1

	for start <= lim {
		var end int

		for end = start; end <= lim; end++ {
			if end > 3 && isBlankSymbol(in[end]) && isBlankSymbol(in[end-3]) {
				if (in[end-1] == 's' || in[end-1] == 'S') && (in[end-2] == 'a' || in[end-2] == 'A') {
					break
				}
			}
		}

		if end < lim {
			out = append(out, trimString(in[start:end-3]))
		} else {
			out = append(out, trimString(in[start:end]))
		}

		start = end + 1
	}

	return
}
