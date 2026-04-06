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

package progress_test

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cowdogmoo/warpgate/v3/progress"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDisplay verifies that NewDisplay returns a non-nil Display backed by
// the supplied writer.
func TestNewDisplay(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplay(&buf)
	require.NotNil(t, d)
}

// TestAddBar verifies that AddBar returns a non-nil Bar and that the bar's
// label, index, and total are set correctly.
func TestAddBar(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("my-service", 1, 3)
	require.NotNil(t, bar)
	assert.Equal(t, "my-service", bar.Label)
	assert.Equal(t, 1, bar.Index)
	assert.Equal(t, 3, bar.Total)
}

// TestBarUpdate verifies that Update correctly sets all mutable fields on the
// bar.
func TestBarUpdate(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)
	bar := d.AddBar("svc", 1, 2)

	bar.Update("Building", 0.5, 30*time.Second, 60*time.Second)

	assert.Equal(t, "Building", bar.Stage)
	assert.InDelta(t, 0.5, bar.Progress, 1e-9)
	assert.Equal(t, 30*time.Second, bar.Elapsed)
	assert.Equal(t, 60*time.Second, bar.EstimatedRemaining)
	assert.False(t, bar.Done)
	assert.False(t, bar.Error)
}

// TestBarComplete verifies that Complete transitions the bar into a finished
// state with Done true, Stage "Complete", and Progress 1.0.
func TestBarComplete(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)
	bar := d.AddBar("svc", 1, 1)
	bar.Update("Building", 0.8, 10*time.Second, 5*time.Second)

	bar.Complete()

	assert.True(t, bar.Done)
	assert.Equal(t, "Complete", bar.Stage)
	assert.InDelta(t, 1.0, bar.Progress, 1e-9)
}

// TestBarFail verifies that Fail transitions the bar into an error state with
// Error true and Stage "Failed".
func TestBarFail(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)
	bar := d.AddBar("svc", 1, 1)
	bar.Update("Pushing", 0.4, 5*time.Second, 10*time.Second)

	bar.Fail()

	assert.True(t, bar.Error)
	assert.Equal(t, "Failed", bar.Stage)
}

// TestRenderTTY verifies that TTY rendering writes expected progress bar
// structure including the index/total prefix, block characters, stage, and
// elapsed time.
func TestRenderTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, true)

	bar1 := d.AddBar("goad-dc-base", 1, 2)
	bar2 := d.AddBar("goad-dc-app", 2, 2)

	bar1.Update("Building", 0.6, 18*time.Minute+14*time.Second, 19*time.Minute)
	bar2.Update("Pushing", 0.2, 2*time.Minute, 8*time.Minute)

	d.Render()

	output := buf.String()

	// Both bars should appear.
	assert.Contains(t, output, "[1/2]")
	assert.Contains(t, output, "[2/2]")

	// Labels should appear.
	assert.Contains(t, output, "goad-dc-base")
	assert.Contains(t, output, "goad-dc-app")

	// Progress bar block characters must be present.
	assert.Contains(t, output, "█")
	assert.Contains(t, output, "░")

	// Stages should appear.
	assert.Contains(t, output, "Building")
	assert.Contains(t, output, "Pushing")

	// Elapsed times should appear.
	assert.Contains(t, output, "18m14s")
	assert.Contains(t, output, "2m0s")
}

// TestRenderNonTTY verifies that non-TTY rendering emits plain text lines
// without ANSI escape codes and still contains the expected bar content.
func TestRenderNonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("plain-svc", 1, 1)
	bar.Update("Downloading", 0.3, 5*time.Second, 0)

	d.Render()

	output := buf.String()

	assert.Contains(t, output, "[1/1]")
	assert.Contains(t, output, "plain-svc")
	assert.Contains(t, output, "Downloading")
	assert.Contains(t, output, "5s")

	// No ANSI cursor-up sequences in plain output.
	assert.NotContains(t, output, "\033[A")
}

// TestRenderNonTTYSkipsDuplicates verifies that a second Render call in non-TTY
// mode does not re-emit a bar whose state has not changed.
func TestRenderNonTTYSkipsDuplicates(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("svc", 1, 1)
	bar.Update("Building", 0.5, 10*time.Second, 20*time.Second)

	d.Render()
	firstLen := buf.Len()

	// Render again without updating — output should not grow.
	d.Render()
	assert.Equal(t, firstLen, buf.Len(), "non-TTY render should not repeat unchanged bars")

	// Update the bar and render once more — output should grow again.
	bar.Update("Building", 0.7, 14*time.Second, 16*time.Second)
	d.Render()
	assert.Greater(t, buf.Len(), firstLen, "non-TTY render should emit updated bar")
}

// TestRenderMultipleBars verifies that all four bars appear in a single render
// pass.
func TestRenderMultipleBars(t *testing.T) {
	t.Parallel()

	labels := []string{"alpha", "beta", "gamma", "delta"}

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, true)

	for i, lbl := range labels {
		b := d.AddBar(lbl, i+1, len(labels))
		b.Update("Building", float64(i)*0.25, time.Duration(i)*time.Minute, 0)
	}

	d.Render()

	output := buf.String()

	for _, lbl := range labels {
		assert.Contains(t, output, lbl)
	}

	assert.Contains(t, output, "[1/4]")
	assert.Contains(t, output, "[4/4]")
}

// TestRenderTTYCursorRewind verifies that subsequent TTY renders emit the
// correct number of cursor-up sequences to rewind the previously rendered
// block.
func TestRenderTTYCursorRewind(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, true)

	bar := d.AddBar("svc", 1, 1)
	bar.Update("Building", 0.5, 5*time.Second, 10*time.Second)

	// First render: no rewind because nothing was previously rendered.
	d.Render()
	firstOutput := buf.String()
	assert.NotContains(t, firstOutput, "\033[A",
		"first render should not contain cursor-up sequence")

	// Second render: one cursor-up sequence expected (one bar rendered previously).
	buf.Reset()
	bar.Update("Building", 0.6, 6*time.Second, 9*time.Second)
	d.Render()
	secondOutput := buf.String()
	assert.Contains(t, secondOutput, "\033[A",
		"second render should contain cursor-up sequence to rewind")
}

// TestRenderComplete verifies that a completed bar shows "Complete" in the output.
func TestRenderComplete(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("finished-svc", 1, 1)
	bar.Update("Building", 0.9, 30*time.Second, 3*time.Second)
	bar.Complete()

	d.Render()

	output := buf.String()
	assert.Contains(t, output, "Complete")
}

// TestRenderFailed verifies that a failed bar shows "Failed" in the output.
func TestRenderFailed(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("broken-svc", 1, 1)
	bar.Update("Building", 0.3, 10*time.Second, 20*time.Second)
	bar.Fail()

	d.Render()

	output := buf.String()
	assert.Contains(t, output, "Failed")
}

// TestRenderRemainingTime verifies that estimated remaining time is included
// in rendered output when it is non-zero and the bar is not done or failed.
func TestRenderRemainingTime(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("svc", 1, 1)
	bar.Update("Building", 0.5, 10*time.Second, 19*time.Minute)

	d.Render()

	output := buf.String()
	assert.Contains(t, output, "remaining")
	assert.Contains(t, output, "19m0s")
}

// TestConcurrentUpdates verifies that concurrent Bar.Update and Display.Render
// calls do not cause data races. Run with `go test -race` to detect issues.
func TestConcurrentUpdates(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Use a mutex-protected writer to avoid races on the buffer itself.
	w := &syncWriter{buf: &buf}
	d := progress.NewDisplayTTY(w, false)

	const numBars = 4
	const updatesPerBar = 50
	const renders = 20

	bars := make([]*progress.Bar, numBars)
	for i := range bars {
		bars[i] = d.AddBar(
			strings.Repeat("x", i+5),
			i+1,
			numBars,
		)
	}

	var wg sync.WaitGroup

	// Spawn one goroutine per bar issuing rapid updates.
	for _, b := range bars {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < updatesPerBar; j++ {
				b.Update("Building", float64(j)/float64(updatesPerBar),
					time.Duration(j)*time.Second, time.Duration(updatesPerBar-j)*time.Second)
			}
			b.Complete()
		}()
	}

	// Spawn a goroutine that renders repeatedly while updates are in flight.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < renders; i++ {
			d.Render()
		}
	}()

	wg.Wait()
}

// syncWriter wraps a bytes.Buffer with a mutex so concurrent goroutines can
// safely write to it during race-detection tests.
type syncWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.Write(p)
}

// TestStartStop verifies the background render loop draws bars without
// manual Render calls, and that Stop performs a final render.
func TestStartStop(t *testing.T) {
	t.Parallel()

	sw := &syncWriter{buf: &bytes.Buffer{}}
	d := progress.NewDisplayTTY(sw, false) // non-TTY so output appends
	bar := d.AddBar("bg-test", 1, 1)
	bar.Update("Working", 0.5, 5*time.Second, 5*time.Second)

	d.Start(50 * time.Millisecond)
	time.Sleep(200 * time.Millisecond) // let a few ticks fire
	bar.Complete()
	d.Stop()

	sw.mu.Lock()
	output := sw.buf.String()
	sw.mu.Unlock()

	assert.Contains(t, output, "Working")
	assert.Contains(t, output, "Complete")
}

// TestStartStopConcurrent verifies that multiple goroutines updating bars
// while the render loop is running produces no races.
func TestStartStopConcurrent(t *testing.T) {
	t.Parallel()

	sw := &syncWriter{buf: &bytes.Buffer{}}
	d := progress.NewDisplayTTY(sw, true)

	const numBars = 4
	bars := make([]*progress.Bar, numBars)
	for i := 0; i < numBars; i++ {
		bars[i] = d.AddBar(fmt.Sprintf("build-%d", i+1), i+1, numBars)
	}

	d.Start(20 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < numBars; i++ {
		wg.Add(1)
		go func(b *progress.Bar, idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				pct := float64(j) / 20.0
				b.Update("Building", pct, time.Duration(j)*time.Second, time.Duration(20-j)*time.Second)
				time.Sleep(5 * time.Millisecond)
			}
			b.Complete()
		}(bars[i], i)
	}

	wg.Wait()
	d.Stop()

	sw.mu.Lock()
	output := sw.buf.String()
	sw.mu.Unlock()

	// All bars should have rendered at least once.
	for i := 1; i <= numBars; i++ {
		assert.Contains(t, output, fmt.Sprintf("[%d/%d]", i, numBars))
	}
}

// TestCompleteWithMessage verifies that CompleteWithMessage sets the
// CompletionMessage and that it appears in the rendered output.
func TestCompleteWithMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := progress.NewDisplayTTY(&buf, false)

	bar := d.AddBar("my-ami", 1, 1)
	bar.Update("Building", 0.9, 30*time.Second, 3*time.Second)
	bar.CompleteWithMessage("ami-0abcdef1234567890")

	d.Render()

	output := buf.String()
	assert.Contains(t, output, "Complete")
	assert.Contains(t, output, "ami-0abcdef1234567890")
}

// TestColorOutput verifies that completed bars contain ANSI color codes
// (green for success, red for failure).
func TestColorOutput(t *testing.T) {
	// fatih/color disables color for non-TTY writers. Force it on for testing.
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	t.Run("green on complete", func(t *testing.T) {
		var buf bytes.Buffer
		d := progress.NewDisplayTTY(&buf, false)
		bar := d.AddBar("svc", 1, 1)
		bar.Complete()
		d.Render()
		// fatih/color green escape: \033[32m
		assert.Contains(t, buf.String(), "\033[32m")
	})

	t.Run("red on failure", func(t *testing.T) {
		var buf bytes.Buffer
		d := progress.NewDisplayTTY(&buf, false)
		bar := d.AddBar("svc", 1, 1)
		bar.Fail()
		d.Render()
		// fatih/color red escape: \033[31m
		assert.Contains(t, buf.String(), "\033[31m")
	})

	t.Run("cyan completion message", func(t *testing.T) {
		var buf bytes.Buffer
		d := progress.NewDisplayTTY(&buf, false)
		bar := d.AddBar("svc", 1, 1)
		bar.CompleteWithMessage("ami-123")
		d.Render()
		// fatih/color cyan escape: \033[36m
		assert.Contains(t, buf.String(), "\033[36m")
		assert.Contains(t, buf.String(), "ami-123")
	})
}
