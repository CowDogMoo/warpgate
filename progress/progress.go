/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

// Package progress provides a lightweight, reusable multi-line progress display
// for tracking concurrent build and operation status. It renders progress bars
// using ANSI escape codes for TTY environments and falls back to simple line
// output for non-TTY destinations such as log files or CI systems.
//
// For concurrent builds, use Start/Stop so callbacks only update bar state:
//
//	display := progress.NewDisplay(os.Stderr)
//	display.Start(500 * time.Millisecond)
//	bar1 := display.AddBar("build-a", 1, 2)
//	bar2 := display.AddBar("build-b", 2, 2)
//	// From goroutines, only call bar.Update() — rendering is automatic.
//	bar1.Update("Building", 0.5, elapsed, remaining)
//	// When all builds finish:
//	display.Stop()
//
// For single-threaded use, call Render() directly:
//
//	display := progress.NewDisplay(os.Stdout)
//	bar := display.AddBar("my-build", 1, 3)
//	bar.Update("Downloading", 0.25, 10*time.Second, 30*time.Second)
//	display.Render()
//	bar.Complete()
//	display.Render()
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

const (
	// DefaultBarWidth is the number of characters used for the filled/empty
	// portion of the progress bar, excluding the surrounding brackets.
	DefaultBarWidth = 25

	// filledChar is the Unicode block element used for completed progress.
	filledChar = "█"

	// emptyChar is the Unicode block element used for remaining progress.
	emptyChar = "░"

	// labelWidth is the fixed width to which labels are padded.
	labelWidth = 24

	// ansiCursorUp moves the cursor up one line.
	ansiCursorUp = "\033[A"

	// ansiClearLine clears the current line and returns the cursor to column 1.
	ansiClearLine = "\033[2K\r"

	// ansiHideCursor hides the terminal cursor to reduce flicker during redraws.
	ansiHideCursor = "\033[?25l"

	// ansiShowCursor restores the terminal cursor.
	ansiShowCursor = "\033[?25h"

	// ansiClearToEnd clears from the cursor to the end of the screen, removing
	// any orphaned lines caused by external newlines (e.g. keypresses).
	ansiClearToEnd = "\033[J"
)

// Bar represents a single tracked build or operation within a Display. It
// holds all rendering state for one row and is safe for concurrent use.
type Bar struct {
	mu sync.Mutex

	// Label is a human-readable identifier for this operation, e.g. "goad-dc-base".
	Label string

	// Index is the 1-based position of this bar among all bars in the display.
	Index int

	// Total is the total number of bars in the display.
	Total int

	// Stage describes the current operation phase, e.g. "Building" or "Pushing".
	Stage string

	// Elapsed is the time spent on this operation so far.
	Elapsed time.Duration

	// EstimatedRemaining is the predicted time left. A zero value signals that
	// the estimate is unknown and no remaining time will be rendered.
	EstimatedRemaining time.Duration

	// Progress is a value in [0.0, 1.0] indicating completion percentage.
	Progress float64

	// Done is true when the operation has finished successfully.
	Done bool

	// Error is true when the operation has terminated with a failure.
	Error bool

	// CompletionMessage is optional text appended after the stage on completion,
	// e.g. an AMI ID or image digest. It is rendered in cyan when set.
	CompletionMessage string

	// lastRendered caches the most recently rendered line so non-TTY displays
	// can skip unchanged bars.
	lastRendered string
}

// Display manages multi-line concurrent progress rendering for a set of Bars.
// It writes to an io.Writer and automatically adapts between TTY and non-TTY
// rendering modes.
//
// For concurrent use, call Start to begin a background render loop and Stop
// when done. Callbacks should only call Bar.Update/Complete/Fail — the render
// loop handles drawing all bars together so output stays clean.
type Display struct {
	mu sync.Mutex

	// writer is the destination for all rendered output.
	writer io.Writer

	// bars is the ordered list of tracked operations.
	bars []*Bar

	// rendered is the number of lines written during the last Render call.
	// Used to rewind the cursor before the next TTY render.
	rendered int

	// isTTY controls whether ANSI in-place rewrite or plain line output is used.
	isTTY bool

	// barWidth is the number of characters in the filled/empty portion of each bar.
	barWidth int

	// stopCh signals the background render loop to exit.
	stopCh chan struct{}
}

// NewDisplay creates a Display that writes to w. The isTTY flag is inferred by
// checking whether w is an *os.File; if so, it is treated as a TTY. Use
// NewDisplayTTY for explicit control.
func NewDisplay(w io.Writer) *Display {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}
	return &Display{
		writer:   w,
		isTTY:    isTTY,
		barWidth: DefaultBarWidth,
	}
}

// NewDisplayTTY creates a Display with an explicitly supplied isTTY value.
// This is primarily useful in tests where a bytes.Buffer should be treated
// as a TTY to exercise ANSI escape-code rendering paths.
func NewDisplayTTY(w io.Writer, isTTY bool) *Display {
	return &Display{
		writer:   w,
		isTTY:    isTTY,
		barWidth: DefaultBarWidth,
	}
}

// AddBar registers a new Bar with the given label, 1-based index, and total
// count, appends it to the display, and returns it so the caller can issue
// subsequent Update, Complete, or Fail calls.
func (d *Display) AddBar(label string, index, total int) *Bar {
	d.mu.Lock()
	defer d.mu.Unlock()

	b := &Bar{
		Label: label,
		Index: index,
		Total: total,
	}
	d.bars = append(d.bars, b)

	return b
}

// Update atomically sets the bar's stage, progress fraction, elapsed time, and
// estimated remaining time. It is safe to call from any goroutine.
func (b *Bar) Update(stage string, progress float64, elapsed, remaining time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Stage = stage
	b.Progress = progress
	b.Elapsed = elapsed
	b.EstimatedRemaining = remaining
}

// Complete marks the bar as successfully finished, setting Done to true,
// Stage to "Complete", and Progress to 1.0.
func (b *Bar) Complete() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Done = true
	b.Stage = "Complete"
	b.Progress = 1.0
}

// CompleteWithMessage marks the bar as finished and attaches a message (e.g.
// an AMI ID) that is rendered in cyan after the stage text.
func (b *Bar) CompleteWithMessage(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Done = true
	b.Stage = "Complete"
	b.Progress = 1.0
	b.CompletionMessage = msg
}

// Fail marks the bar as having encountered an error, setting Error to true and
// Stage to "Failed".
func (b *Bar) Fail() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Error = true
	b.Stage = "Failed"
}

// SetTotal updates the Total field on every bar in the display.
// This is useful when the total number of operations is not known upfront.
func (d *Display) SetTotal(total int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, b := range d.bars {
		b.mu.Lock()
		b.Total = total
		b.mu.Unlock()
	}
}

// Start begins a background goroutine that calls Render at the given interval.
// Use this when multiple goroutines update bars concurrently — they should only
// call Bar.Update/Complete/Fail, and the render loop draws all bars together.
// Call Stop to halt the loop and perform a final render.
func (d *Display) Start(interval time.Duration) {
	d.mu.Lock()
	d.stopCh = make(chan struct{})
	ch := d.stopCh
	d.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.Render()
			case <-ch:
				return
			}
		}
	}()
}

// Stop halts the background render loop started by Start and performs a final
// render to ensure the terminal shows the latest state.
func (d *Display) Stop() {
	d.mu.Lock()
	ch := d.stopCh
	d.stopCh = nil
	d.mu.Unlock()

	if ch != nil {
		close(ch)
	}
	d.Render()
}

// IsFinished returns true if the bar is done or has an error.
func (b *Bar) IsFinished() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Done || b.Error
}

// Render redraws all bars. In TTY mode the cursor is rewound by the number of
// lines written in the previous render so that output appears to update in
// place. In non-TTY mode only bars whose rendered representation has changed
// since the last Render call are printed, avoiding duplicate log lines.
func (d *Display) Render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isTTY {
		d.renderTTY()
	} else {
		d.renderPlain()
	}
}

// renderTTY performs in-place rewrite of all bar lines using ANSI escape codes.
// It hides the cursor during redraws and clears any orphaned lines below the
// bar block (e.g. from user keypresses injecting newlines). It must be called
// with d.mu held.
func (d *Display) renderTTY() {
	_, _ = fmt.Fprint(d.writer, ansiHideCursor)

	// Rewind cursor to the start of the previously rendered block.
	for i := 0; i < d.rendered; i++ {
		_, _ = fmt.Fprint(d.writer, ansiCursorUp)
	}

	// Clear from cursor to end of screen to wipe orphaned lines caused by
	// external newlines (keypresses, other output).
	_, _ = fmt.Fprint(d.writer, ansiClearToEnd)

	for _, b := range d.bars {
		line := d.formatBar(b)
		_, _ = fmt.Fprint(d.writer, ansiClearLine+line+"\n")
	}

	_, _ = fmt.Fprint(d.writer, ansiShowCursor)

	d.rendered = len(d.bars)
}

// renderPlain emits only changed bar lines without any ANSI cursor movement,
// suitable for log files and CI pipelines. It must be called with d.mu held.
func (d *Display) renderPlain() {
	for _, b := range d.bars {
		line := d.formatBar(b)

		b.mu.Lock()
		changed := line != b.lastRendered
		if changed {
			b.lastRendered = line
		}
		b.mu.Unlock()

		if changed {
			_, _ = fmt.Fprintln(d.writer, line)
		}
	}
}

// formatBar produces the single-line string representation of b. It snapshots
// b's state under b.mu so callers need not hold any lock.
func (d *Display) formatBar(b *Bar) string {
	b.mu.Lock()
	label := b.Label
	index := b.Index
	total := b.Total
	stage := b.Stage
	elapsed := b.Elapsed
	remaining := b.EstimatedRemaining
	progress := b.Progress
	done := b.Done
	hasErr := b.Error
	completionMsg := b.CompletionMessage
	b.mu.Unlock()

	// Clamp progress to a valid range.
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	// Build the progress bar characters.
	filled := int(progress * float64(d.barWidth))
	empty := d.barWidth - filled
	bar := "[" + strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, empty) + "]"

	// Pad label to a fixed width so columns stay aligned.
	paddedLabel := label
	if len(label) < labelWidth {
		paddedLabel = label + strings.Repeat(" ", labelWidth-len(label))
	} else if len(label) > labelWidth {
		paddedLabel = label[:labelWidth]
	}

	// Format elapsed time using the natural duration string.
	elapsedStr := elapsed.Truncate(time.Second).String()
	if elapsed == 0 {
		elapsedStr = "0s"
	}

	// Build the stage/status suffix.
	stageStr := stage
	if done {
		stageStr = "Complete"
	} else if hasErr {
		stageStr = "Failed"
	}

	// Assemble the line.
	line := fmt.Sprintf("[%d/%d] %s %s %-14s %s",
		index, total, paddedLabel, bar, stageStr, elapsedStr)

	if remaining > 0 && !done && !hasErr {
		line += fmt.Sprintf("  ~%s remaining", remaining.Truncate(time.Second).String())
	}

	// Colorize the entire line for terminal states.
	if done {
		line = color.GreenString(line)
		if completionMsg != "" {
			line += "  " + color.CyanString(completionMsg)
		}
	} else if hasErr {
		line = color.RedString(line)
	}

	return line
}
