// Copyright (c) 2020 Armory, Inc.
// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package stacktrace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureAsString(t *testing.T) {
	trace := CaptureAsString(0)
	lines := strings.Split(trace, "\n")
	require.NotEmpty(t, lines, "Expected stacktrace to have at least one frame.")
	assert.Contains(
		t,
		lines[0],
		"go.uber.org/zap.TestTakeStacktrace",
		"Expected stacktrace to start with the test.",
	)
}

func TestCaptureAsStringWithSkip(t *testing.T) {
	trace := CaptureAsString(1)
	lines := strings.Split(trace, "\n")
	require.NotEmpty(t, lines, "Expected stacktrace to have at least one frame.")
	assert.Contains(
		t,
		lines[0],
		"testing.",
		"Expected stacktrace to start with the test runner (skipping our own frame).",
	)
}

func TestCaptureAsStringWithSkipInnerFunc(t *testing.T) {
	var trace string
	func() {
		trace = CaptureAsString(2)
	}()
	lines := strings.Split(trace, "\n")
	require.NotEmpty(t, lines, "Expected stacktrace to have at least one frame.")
	assert.Contains(
		t,
		lines[0],
		"testing.",
		"Expected stacktrace to start with the test function (skipping the test function).",
	)
}

func TestCaptureAsStringDeepStack(t *testing.T) {
	const (
		N                  = 500
		withStackDepthName = "go.uber.org/zap.withStackDepth"
	)
	withStackDepth(N, func() {
		trace := CaptureAsString(0)
		for found := 0; found < N; found++ {
			i := strings.Index(trace, withStackDepthName)
			if i < 0 {
				t.Fatalf(`expected %v occurrences of %q, found %d`,
					N, withStackDepthName, found)
			}
			trace = trace[i+len(withStackDepthName):]
		}
	})
}

func BenchmarkCaptureAsString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CaptureAsString(0)
	}
}

func withStackDepth(depth int, f func()) {
	var recurse func(rune) rune
	recurse = func(r rune) rune {
		if r > 0 {
			bytes.Map(recurse, []byte(string([]rune{r - 1})))
		} else {
			f()
		}
		return 0
	}
	recurse(rune(depth))
}
