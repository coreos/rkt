package ioprogress

import (
	"bytes"
	"testing"
)

func TestDrawTerminal(t *testing.T) {
	var buf bytes.Buffer
	fn := DrawTerminal(&buf)
	if err := fn(0, 100); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := fn(20, 100); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := fn(-1, -1); err != nil {
		t.Fatalf("err: %s", err)
	}

	if buf.String() != drawTerminalStr {
		t.Fatalf("bad:\n\n%#v", buf.String())
	}
}

func TestDrawTextFormatBar(t *testing.T) {
	var actual, expected string
	f := DrawTextFormatBar(10)

	actual = f(5, 10)
	expected = "[====    ]"
	if actual != expected {
		t.Fatalf("bad: %s", actual)
	}

	actual = f(2, 10)
	expected = "[=       ]"
	if actual != expected {
		t.Fatalf("bad: %s", actual)
	}

	actual = f(10, 10)
	expected = "[========]"
	if actual != expected {
		t.Fatalf("bad: %s", actual)
	}
}

func TestDrawTextFormatBytes(t *testing.T) {
	cases := []struct {
		P, T   int64
		Output string
	}{
		{
			100, 500,
			"100 B/500 B",
		},
		{
			1500, 5000,
			"1.5 KB/5 KB",
		},
	}

	for _, tc := range cases {
		output := DrawTextFormatBytes(tc.P, tc.T)
		if output != tc.Output {
			t.Fatalf("Input: %d, %d\n\n%s", tc.P, tc.T, output)
		}
	}
}

const drawTerminalStr = "0/100\r20/100\r\n"
