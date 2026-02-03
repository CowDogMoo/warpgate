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

package ami

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatEC2StatusString(t *testing.T) {
	t.Parallel()

	t.Run("nil status", func(t *testing.T) {
		t.Parallel()
		var status *EC2InstanceStatus
		assert.Equal(t, "No build instance found", status.FormatEC2StatusString())
	})

	t.Run("basic status", func(t *testing.T) {
		t.Parallel()
		status := &EC2InstanceStatus{
			InstanceID:   "i-1234567890abcdef0",
			State:        "running",
			InstanceType: "t3.medium",
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "i-1234567890abcdef0")
		assert.Contains(t, result, "running")
		assert.Contains(t, result, "t3.medium")
	})

	t.Run("with private IP", func(t *testing.T) {
		t.Parallel()
		status := &EC2InstanceStatus{
			InstanceID:   "i-abc",
			State:        "running",
			InstanceType: "t3.micro",
			PrivateIP:    "10.0.1.5",
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "10.0.1.5")
	})

	t.Run("with launch time", func(t *testing.T) {
		t.Parallel()
		launchTime := time.Now().Add(-5 * time.Minute)
		status := &EC2InstanceStatus{
			InstanceID:   "i-abc",
			State:        "running",
			InstanceType: "t3.micro",
			LaunchTime:   &launchTime,
		}
		result := status.FormatEC2StatusString()
		assert.Contains(t, result, "Uptime:")
	})
}

func TestNewBuildMonitor(t *testing.T) {
	t.Parallel()

	config := MonitorConfig{
		StreamLogs:    true,
		ShowEC2Status: true,
	}

	monitor := NewBuildMonitor(nil, "test-image", config)
	assert.NotNil(t, monitor)
	assert.Equal(t, "test-image", monitor.imageName)
	assert.True(t, monitor.streamLogs)
	assert.True(t, monitor.showEC2Status)
	assert.Equal(t, "/aws/imagebuilder/test-image", monitor.logGroupName)
}
