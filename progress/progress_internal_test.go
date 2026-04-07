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

package progress

import (
	"bytes"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStopRestoresTerminal verifies that Stop() calls the restoreTerminal
// function when it is set. In test environments suppressEcho returns nil
// because stdin is not a terminal, so this branch is otherwise uncovered.
func TestStopRestoresTerminal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := NewDisplayTTY(&buf, true)

	var restoreCalled atomic.Int32

	// Manually set the unexported restoreTerminal field to a tracking function.
	d.restoreTerminal = func() {
		restoreCalled.Add(1)
	}

	// Also set up a stopCh so Stop closes it cleanly.
	d.stopCh = make(chan struct{})

	// Add a bar so Stop's final Render has something to draw.
	bar := d.AddBar("test-svc", 1, 1)
	bar.Update("Building", 0.5, 5*time.Second, 5*time.Second)

	d.Stop()

	assert.Equal(t, int32(1), restoreCalled.Load(),
		"restoreTerminal should be called exactly once by Stop")

	// Calling Stop again should not call restore a second time (it was nil-ed out).
	d.Stop()
	assert.Equal(t, int32(1), restoreCalled.Load(),
		"restoreTerminal should not be called again after being cleared")
}

// TestStopWithNilRestoreTerminal verifies that Stop() works correctly when
// restoreTerminal is nil (the common case in non-TTY environments).
func TestStopWithNilRestoreTerminal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := NewDisplayTTY(&buf, false)
	bar := d.AddBar("test-svc", 1, 1)
	bar.Update("Working", 0.5, 5*time.Second, 5*time.Second)

	d.Start(50 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	bar.Complete()

	// Should not panic when restoreTerminal is nil.
	d.Stop()

	assert.Contains(t, buf.String(), "Complete")
}

// TestSetTotal verifies that SetTotal updates the Total field on all bars.
func TestSetTotal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := NewDisplayTTY(&buf, false)

	bar1 := d.AddBar("svc-1", 1, 1)
	bar2 := d.AddBar("svc-2", 2, 1)

	d.SetTotal(5)

	bar1.mu.Lock()
	assert.Equal(t, 5, bar1.Total)
	bar1.mu.Unlock()

	bar2.mu.Lock()
	assert.Equal(t, 5, bar2.Total)
	bar2.mu.Unlock()
}

// TestIsFinished verifies the IsFinished method for Done and Error states.
func TestIsFinished(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	d := NewDisplayTTY(&buf, false)

	t.Run("not finished", func(t *testing.T) {
		bar := d.AddBar("active", 1, 1)
		bar.Update("Building", 0.5, 5*time.Second, 5*time.Second)
		assert.False(t, bar.IsFinished())
	})

	t.Run("done", func(t *testing.T) {
		bar := d.AddBar("done", 2, 2)
		bar.Complete()
		assert.True(t, bar.IsFinished())
	})

	t.Run("error", func(t *testing.T) {
		bar := d.AddBar("failed", 3, 3)
		bar.Fail()
		assert.True(t, bar.IsFinished())
	})
}
