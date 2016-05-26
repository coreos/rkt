// Copyright 2016 CoreOS Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package progressutil

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh/terminal"
)

var (
	// ErrorProgressOutOfBounds is returned if the progress is set to a value
	// not between 0 and 1.
	ErrorProgressOutOfBounds = fmt.Errorf("progress is out of bounds (0 to 1)")

	// ErrorNoBarsAdded is returned when no progress bars have been added to a
	// ProgressBarPrinter before PrintAndWait is called.
	ErrorNoBarsAdded = fmt.Errorf("AddProgressBar hasn't been called yet")
)

// ProgressBar represents one progress bar in a ProgressBarPrinter. Should not
// be created directly, use the AddProgressBar on a ProgressBarPrinter to
// create these.
type ProgressBar struct {
	currentProgress float64
	printBefore     string
	printAfter      string
	lock            sync.Mutex
	done            bool
}

// SetCurrentProgress sets the progress of this ProgressBar. The progress must
// be between 0 and 1 inclusive.
func (pb *ProgressBar) SetCurrentProgress(progress float64) error {
	if progress < 0 || progress > 1 {
		return ErrorProgressOutOfBounds
	}
	pb.lock.Lock()
	pb.currentProgress = progress
	pb.lock.Unlock()
	return nil
}

// SetPrintBefore sets the text printed on the line before the progress bar.
func (pb *ProgressBar) SetPrintBefore(before string) {
	pb.lock.Lock()
	pb.printBefore = before
	pb.lock.Unlock()
}

// SetPrintAfter sets the text printed on the line after the progress bar.
func (pb *ProgressBar) SetPrintAfter(after string) {
	pb.lock.Lock()
	pb.printAfter = after
	pb.lock.Unlock()
}

// ProgressBarPrinter will print out the progress of some number of
// ProgressBars.
type ProgressBarPrinter struct {
	// DisplayWidth can be set to influence how large the progress bars are.
	// The bars will be scaled to attempt to produce lines of this number of
	// characters, but lines of different lengths may still be printed. When
	// this value is 0 (aka unset), 80 character columns are assumed.
	DisplayWidth int
	// PadToBeEven, when set to true, will make Print pad the printBefore text
	// with trailing spaces and the printAfter text with leading spaces to make
	// the progress bars the same length.
	PadToBeEven         bool
	numLinesInLastPrint int
	progressBars        []*ProgressBar
	lock                sync.Mutex
}

// AddProgressBar will create a new ProgressBar, register it with this
// ProgressBarPrinter, and return it. This must be called at least once before
// PrintAndWait is called.
func (pbp *ProgressBarPrinter) AddProgressBar() *ProgressBar {
	pb := &ProgressBar{}
	pbp.lock.Lock()
	pbp.progressBars = append(pbp.progressBars, pb)
	pbp.lock.Unlock()
	return pb
}

// Print will print out progress information for each ProgressBar that has been
// added to this ProgressBarPrinter. The progress will be written to printTo,
// and if printTo is a terminal it will draw progress bars.  AddProgressBar
// must be called at least once before Print is called. If printing to a
// terminal, all draws after the first one will move the cursor up to draw over
// the previously printed bars.
func (pbp *ProgressBarPrinter) Print(printTo io.Writer) (bool, error) {
	pbp.lock.Lock()
	bars := pbp.progressBars
	numColumns := pbp.DisplayWidth
	pbp.lock.Unlock()

	if len(bars) == 0 {
		return false, ErrorNoBarsAdded
	}

	if numColumns == 0 {
		numColumns = 80
	}

	if isTerminal(printTo) {
		moveCursorUp(printTo, pbp.numLinesInLastPrint)
	}

	maxBefore := 0
	maxAfter := 0
	for _, bar := range bars {
		bar.lock.Lock()
		beforeSize := len(bar.printBefore)
		afterSize := len(bar.printAfter)
		bar.lock.Unlock()
		if beforeSize > maxBefore {
			maxBefore = beforeSize
		}
		if afterSize > maxAfter {
			maxAfter = afterSize
		}
	}

	allDone := false
	for _, bar := range bars {
		if isTerminal(printTo) {
			bar.printToTerminal(printTo, numColumns, pbp.PadToBeEven, maxBefore, maxAfter)
		} else {
			bar.printToNonTerminal(printTo)
		}
		allDone = allDone || bar.currentProgress == 1
	}

	pbp.numLinesInLastPrint = len(bars)

	return allDone, nil
}

// moveCursorUp moves the cursor up numLines in the terminal
func moveCursorUp(printTo io.Writer, numLines int) {
	if numLines > 0 {
		fmt.Fprintf(printTo, "\033[%dA", numLines)
	}
}

func (pb *ProgressBar) printToTerminal(printTo io.Writer, numColumns int, padding bool, maxBefore, maxAfter int) {
	pb.lock.Lock()
	before := pb.printBefore
	after := pb.printAfter

	if padding {
		before = before + strings.Repeat(" ", maxBefore-len(before))
		after = strings.Repeat(" ", maxAfter-len(after)) + after
	}

	progressBarSize := numColumns - (len(fmt.Sprintf("%s [] %s", before, after)))
	progressBar := ""
	if progressBarSize > 0 {
		currentProgress := int(pb.currentProgress * float64(progressBarSize))
		progressBar = fmt.Sprintf("[%s%s] ",
			strings.Repeat("=", currentProgress),
			strings.Repeat(" ", progressBarSize-currentProgress))
	} else {
		// If we can't fit the progress bar, better to not pad the before/after.
		before = pb.printBefore
		after = pb.printAfter
	}

	fmt.Fprintf(printTo, "%s %s%s\n", before, progressBar, after)
	pb.lock.Unlock()
}

func (pb *ProgressBar) printToNonTerminal(printTo io.Writer) {
	pb.lock.Lock()
	if !pb.done {
		fmt.Fprintf(printTo, "%s %s\n", pb.printBefore, pb.printAfter)
		if pb.currentProgress == 1 {
			pb.done = true
		}
	}
	pb.lock.Unlock()
}

// isTerminal returns True when w is going to a tty, and false otherwise.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return terminal.IsTerminal(int(f.Fd()))
	}
	return false
}
