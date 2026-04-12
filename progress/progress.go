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
	"os/signal"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"golang.org/x/term"
)

const (
	// DefaultBarWidth is the number of characters used for the filled/empty
	// portion of the progress bar, excluding the surrounding brackets.
	DefaultBarWidth = 25

	// minBarWidth is the smallest bar fill area before we start shrinking
	// other elements (label) to reclaim space.
	minBarWidth = 10

	// filledChar is the Unicode block element used for completed progress.
	filledChar = "█"

	// emptyChar is the character used for remaining progress. A space
	// provides maximum visual contrast against the filled block.
	emptyChar = " "

	// labelWidth is the fixed width to which labels are padded.
	labelWidth = 24

	// ansiClearLine clears the current line and returns the cursor to column 1.
	ansiClearLine = "\033[2K\r"

	// ansiHideCursor hides the terminal cursor to reduce flicker during redraws.
	ansiHideCursor = "\033[?25l"

	// ansiShowCursor restores the terminal cursor.
	ansiShowCursor = "\033[?25h"

	// ansiClearToEnd clears from the cursor to the end of the screen, removing
	// any orphaned lines caused by external newlines (e.g. keypresses).
	ansiClearToEnd = "\033[J"

	// ansiCursorHome moves the cursor to the top-left corner of the screen.
	ansiCursorHome = "\033[H"

	// ansiAltScreenEnter switches to the alternate screen buffer and clears it.
	// Used by Start() to isolate progress rendering from the main scrollback.
	ansiAltScreenEnter = "\033[?1049h"

	// ansiAltScreenLeave switches back to the main screen buffer, restoring
	// the content that was visible before entering the alternate screen.
	ansiAltScreenLeave = "\033[?1049l"
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

	// rendered is the number of terminal rows written during the last TTY
	// Render. Accounts for line wrapping when a bar's visual width exceeds
	// the terminal width. Used to rewind the cursor before the next TTY render.
	rendered int

	// isTTY controls whether ANSI in-place rewrite or plain line output is used.
	isTTY bool

	// barWidth is the number of characters in the filled/empty portion of each bar.
	barWidth int

	// useAltScreen is true when Start() has entered the alternate screen buffer.
	// In this mode, renderTTY uses cursor-home instead of cursor-up so that
	// no content leaks into the main scrollback.
	useAltScreen bool

	// stopCh signals the background render loop to exit.
	stopCh chan struct{}

	// sigCh receives OS signals (SIGINT) so the display can restore terminal
	// state before the process exits. Nil when no signal handler is registered.
	sigCh chan os.Signal

	// restoreTerminal is called by Stop to restore the terminal's original
	// input settings after echo suppression. Nil when echo was not suppressed.
	restoreTerminal func()

	// writeErr captures the first write error. Once set, subsequent renders
	// are skipped. Check with Err() after Stop().
	writeErr error

	// ttyFile is a /dev/tty file descriptor opened for terminal size queries
	// when the caller-supplied writer is not detected as a TTY. All rendering
	// goes to the original writer. Closed by Stop or Close. Nil when no
	// fallback was needed.
	ttyFile *os.File
}

// NewDisplay creates a Display that writes to w. The isTTY flag is inferred by
// checking whether w is an *os.File backed by a terminal. When the check fails
// (common under macOS/tmux where stderr loses its TTY association), NewDisplay
// opens /dev/tty to confirm a controlling terminal exists and uses it for
// terminal size queries, while continuing to write all output to w so that ANSI
// cursor tracking stays consistent with other output on the same fd. Use
// NewDisplayTTY for explicit control.
func NewDisplay(w io.Writer) *Display {
	isTTY := false
	var ttyFile *os.File

	if f, ok := w.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	// Fallback: open /dev/tty to detect that a controlling terminal exists.
	// This handles environments (macOS + tmux, certain CI runners) where
	// stderr's fd fails the IsTerminal check even though a controlling
	// terminal exists. The ttyFile is used for terminal size queries only —
	// all rendering still goes to the original writer so that ANSI cursor
	// tracking stays consistent with other output on the same fd.
	if !isTTY {
		if tty := openTTY(); tty != nil {
			ttyFile = tty
			isTTY = true
		}
	}

	return &Display{
		writer:   w,
		isTTY:    isTTY,
		barWidth: DefaultBarWidth,
		ttyFile:  ttyFile,
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

// Err returns the first write error encountered during rendering, or nil.
// Check this after Stop() to determine whether output was written successfully.
func (d *Display) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writeErr
}

// terminalWidth returns the current terminal width in columns, or 0 if the
// width cannot be determined (non-TTY writer, error, etc.). When a /dev/tty
// fallback fd is available it is preferred, since it reliably reports
// dimensions even when the writer's fd fails GetSize (common in tmux).
func (d *Display) terminalWidth() int {
	// Prefer the /dev/tty fallback fd for size queries.
	if d.ttyFile != nil {
		w, _, err := term.GetSize(int(d.ttyFile.Fd()))
		if err == nil && w > 0 {
			return w
		}
	}
	f, ok := d.writer.(*os.File)
	if !ok {
		return 0
	}
	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return w
}

// write writes s to the display's writer. On the first error it records
// the error in writeErr; subsequent calls after an error are no-ops.
// It must be called with d.mu held.
func (d *Display) write(s string) {
	if d.writeErr != nil {
		return
	}
	if _, err := fmt.Fprint(d.writer, s); err != nil {
		d.writeErr = err
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
//
// On TTY outputs, Start switches to the alternate screen buffer so progress
// rendering is fully isolated from the main scrollback. It also hides the
// cursor and suppresses terminal echo. A signal handler is registered to
// restore the terminal on SIGINT. The original screen and terminal state
// are restored by Stop.
func (d *Display) Start(interval time.Duration) {
	d.mu.Lock()
	d.stopCh = make(chan struct{})
	ch := d.stopCh
	if d.isTTY {
		d.write(ansiAltScreenEnter)
		d.useAltScreen = true
		d.write(ansiHideCursor)
		d.restoreTerminal = suppressEcho()

		// Register a signal handler to restore terminal state if the
		// process is interrupted before Stop() is called.
		d.sigCh = make(chan os.Signal, 1)
		sigCh := d.sigCh
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			select {
			case <-sigCh:
				d.mu.Lock()
				restore := d.restoreTerminal
				d.restoreTerminal = nil
				d.write(ansiShowCursor)
				if d.useAltScreen {
					d.write(ansiAltScreenLeave)
					d.useAltScreen = false
				}
				d.mu.Unlock()
				if restore != nil {
					restore()
				}
			case <-ch:
			}
		}()
	}
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
// render to ensure the terminal shows the latest state. On TTY displays it
// leaves the alternate screen buffer and prints the final bar state on the
// main screen, then restores the terminal's original input settings. It also
// closes any internally-opened /dev/tty fallback file.
func (d *Display) Stop() {
	d.mu.Lock()
	ch := d.stopCh
	d.stopCh = nil
	restore := d.restoreTerminal
	d.restoreTerminal = nil
	sigCh := d.sigCh
	d.sigCh = nil
	d.mu.Unlock()

	if sigCh != nil {
		signal.Stop(sigCh)
	}

	if ch != nil {
		close(ch)
	}

	// Final render on the alt screen, then restore main screen.
	d.Render()
	d.mu.Lock()
	d.write(ansiShowCursor)
	if d.useAltScreen {
		d.write(ansiAltScreenLeave)
		d.useAltScreen = false
	}
	d.mu.Unlock()

	if restore != nil {
		restore()
	}

	d.Close()
}

// Close releases any internally-opened resources (e.g. the /dev/tty fallback
// file). It is safe to call multiple times. For displays that use Start/Stop,
// Stop calls Close automatically; for single-threaded callers that never call
// Start, call Close when done to avoid leaking the file descriptor.
func (d *Display) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ttyFile != nil {
		_ = d.ttyFile.Close()
		d.ttyFile = nil
	}
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

	if d.writeErr != nil {
		return
	}

	if d.isTTY {
		d.renderTTY()
	} else {
		d.renderPlain()
	}
}

// renderTTY performs in-place rewrite of all bar lines using ANSI escape codes.
// It uses cursor-up rewind to overwrite previously rendered lines, combined with
// clear-to-end to remove any orphaned content. To minimize scrollback pollution,
// it skips the rewrite entirely when all formatted lines are identical to the
// previous frame. The cursor-up count accounts for line wrapping so that lines
// exceeding the terminal width do not cause the display to scroll. It must be
// called with d.mu held.
func (d *Display) renderTTY() {
	termW := d.terminalWidth()

	// Build all lines first to check for changes.
	lines := make([]string, len(d.bars))
	changed := false
	for i, b := range d.bars {
		lines[i] = d.formatBar(b, termW)
		if !changed {
			b.mu.Lock()
			if lines[i] != b.lastRendered {
				changed = true
			}
			b.mu.Unlock()
		}
	}

	// Detect structural changes: new bars added, terminal resized causing
	// different row counts due to line wrapping, etc.
	rows := d.rowCount(lines, termW)
	if !changed && d.rendered != rows {
		changed = true
	}
	if !changed {
		return
	}

	// Buffer the entire frame and write it as a single string to prevent
	// interleaved output from other goroutines corrupting the display.
	var buf strings.Builder
	if d.useAltScreen {
		buf.WriteString(ansiCursorHome)
	} else if d.rendered > 0 {
		fmt.Fprintf(&buf, "\033[%dA", d.rendered)
	}
	buf.WriteString(ansiClearToEnd)
	for _, line := range lines {
		buf.WriteString(ansiClearLine)
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	d.write(buf.String())

	for i, b := range d.bars {
		b.mu.Lock()
		b.lastRendered = lines[i]
		b.mu.Unlock()
	}

	d.rendered = rows
}

// renderPlain emits all bars as a block when any bar has changed, without ANSI
// cursor movement. A blank line separates successive blocks so log output stays
// readable. It must be called with d.mu held.
func (d *Display) renderPlain() {
	lines := make([]string, len(d.bars))
	changed := false
	for i, b := range d.bars {
		lines[i] = d.formatBar(b, 0)
		if !changed {
			b.mu.Lock()
			if lines[i] != b.lastRendered {
				changed = true
			}
			b.mu.Unlock()
		}
	}
	if !changed {
		return
	}

	if d.rendered > 0 {
		d.write("\n")
	}

	for i, b := range d.bars {
		d.write(lines[i] + "\n")
		b.mu.Lock()
		b.lastRendered = lines[i]
		b.mu.Unlock()
	}

	d.rendered = len(d.bars)
}

// rowCount returns the total number of terminal rows that lines would occupy,
// accounting for line wrapping when a line's visual width exceeds termW.
// When termW is zero or negative, each line counts as one row.
func (d *Display) rowCount(lines []string, termW int) int {
	if termW <= 0 {
		return len(lines)
	}
	rows := 0
	for _, line := range lines {
		vw := visualWidth(line)
		if vw > termW {
			rows += (vw + termW - 1) / termW
		} else {
			rows++
		}
	}
	return rows
}

// visualWidth returns the number of terminal columns s would occupy after
// stripping ANSI escape sequences. Standard-width UTF-8 runes (including
// box-drawing characters like █) count as one column each.
func visualWidth(s string) int {
	n := 0
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\033' && i+1 < len(s) && s[i+1] == '[':
			i += 2
			for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == ';') {
				i++
			}
			if i < len(s) {
				i++
			}
		case s[i] < 0x80:
			n++
			i++
		default:
			_, size := utf8.DecodeRuneInString(s[i:])
			n++
			i += size
		}
	}
	return n
}

// barLayout holds the resolved widths and suffix text produced by
// adaptBarLayout so that formatBar can assemble the final line.
type barLayout struct {
	barWidth        int
	labelWidth      int
	remainingSuffix string
}

// adaptBarLayout computes effective bar and label widths plus the remaining-
// time suffix for a given maxWidth budget. When maxWidth is zero or negative
// width adaptation is disabled and the display defaults are returned unchanged.
func (d *Display) adaptBarLayout(maxWidth, indexLen int, elapsedStr, remainingSuffix string, showRemaining bool) barLayout {
	effectiveBarWidth := d.barWidth
	effectiveLabelWidth := labelWidth

	if maxWidth <= 0 {
		return barLayout{effectiveBarWidth, effectiveLabelWidth, remainingSuffix}
	}

	// Fixed overhead: [I/T] + " " + label + " " + "[]" + " " + stage(14) + " " + elapsed
	overhead := indexLen + 1 + effectiveLabelWidth + 1 + 2 + 1 + 14 + 1 + len(elapsedStr)
	budget := maxWidth - overhead - len(remainingSuffix)

	// If bar won't fit at minimum size, drop remaining time first.
	if budget < minBarWidth && showRemaining {
		budget += len(remainingSuffix)
		remainingSuffix = ""
	}

	// If still too narrow, shrink the label.
	if budget < minBarWidth {
		shrink := minBarWidth - budget
		effectiveLabelWidth -= shrink
		if effectiveLabelWidth < 8 {
			effectiveLabelWidth = 8
		}
		overhead = indexLen + 1 + effectiveLabelWidth + 1 + 2 + 1 + 14 + 1 + len(elapsedStr)
		budget = maxWidth - overhead
	}

	if budget < 5 {
		budget = 5
	}
	if budget < effectiveBarWidth {
		effectiveBarWidth = budget
	}

	return barLayout{effectiveBarWidth, effectiveLabelWidth, remainingSuffix}
}

// formatBar produces the single-line string representation of b. maxWidth is
// the terminal width in columns; when positive the output is adapted to fit
// without wrapping (bar shrinks, then remaining time is dropped, then label
// shrinks). A zero or negative maxWidth disables width adaptation.
func (d *Display) formatBar(b *Bar, maxWidth int) string {
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

	// Build the stage/status suffix.
	stageStr := stage
	if done {
		stageStr = "Complete"
	} else if hasErr {
		stageStr = "Failed"
	}

	// Format elapsed time using the natural duration string.
	elapsedStr := elapsed.Truncate(time.Second).String()
	if elapsed == 0 {
		elapsedStr = "0s"
	}

	showRemaining := remaining > 0 && !done && !hasErr
	remainingSuffix := ""
	if showRemaining {
		remainingSuffix = fmt.Sprintf("  ~%s remaining", remaining.Truncate(time.Second).String())
	}

	// Determine effective widths based on terminal size.
	indexStr := fmt.Sprintf("[%d/%d]", index, total)
	layout := d.adaptBarLayout(maxWidth, len(indexStr), elapsedStr, remainingSuffix, showRemaining)

	// Build the progress bar characters.
	filled := int(progress * float64(layout.barWidth))
	empty := layout.barWidth - filled
	bar := "[" + strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, empty) + "]"

	// Pad label to a fixed width so columns stay aligned.
	paddedLabel := label
	if len(label) < layout.labelWidth {
		paddedLabel = label + strings.Repeat(" ", layout.labelWidth-len(label))
	} else if len(label) > layout.labelWidth {
		paddedLabel = label[:layout.labelWidth]
	}

	// Assemble the line.
	line := fmt.Sprintf("[%d/%d] %s %s %-14s %s",
		index, total, paddedLabel, bar, stageStr, elapsedStr)

	line += layout.remainingSuffix

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
