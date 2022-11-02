// Package awaitility provides a simple mechanism to poll for conditions with a general timeout.
//
// ported from: https://github.com/ecodia/golang-awaitility/blob/fb55e59708c71e8c4186a9310a2c233ca5474146/awaitility/awaitility.go,
// original license (MIT): https://github.com/ecodia/golang-awaitility/blob/fb55e59708c71e8c4186a9310a2c233ca5474146/LICENSE
// It is inspired by the great jvm lib "awaitility" (see https://github.com/awaitility/awaitility)
package awaitility

import (
	"fmt"
	"github.com/pkg/errors"
	"runtime/debug"
	"strings"
	"time"
)

const (
	// DefaultPollInterval The default poll interval (100 ms)
	DefaultPollInterval = 100 * time.Millisecond

	// DefaultAtMost The default maximum timeout (10 secs)
	DefaultAtMost = 10 * time.Second

	// TimeoutError The timeout error's prefix
	TimeoutError = "Await condition did not return true, limit reached"
)

func untilWrapper(until func() bool, result chan bool) {
	result <- until()
}

// Await calls the "until" function initially and in the specified "pollInterval" until
// the total time spent exceeds the "atMost" limit. In this case an error is returned.
// There is no way of forcing a go routine to stop, so if the "until" function is long
// running it will continue to run in the background, even despite the Await function
// exiting after the atMost timeout.
func Await(pollInterval time.Duration, atMost time.Duration, until func() bool) error {

	if pollInterval <= 0 {
		return fmt.Errorf("PollInterval cannot be 0 or below, got: %d", pollInterval)
	}

	if atMost <= 0 {
		return fmt.Errorf("AtMost timeout cannot be 0 or below, got: %d", atMost)
	}

	if pollInterval > atMost {
		return fmt.Errorf("PollInterval must be smaller than atMost timeout, got: pollInterval=%d, atMost=%d", pollInterval, atMost)
	}

	startTime := time.Now()
	timeLeft := atMost

	resultChan := make(chan bool)

	go untilWrapper(until, resultChan)

	for {
		select {
		case conditionOk := <-resultChan:
			if conditionOk {
				return nil
			} else {
				timeLeft = atMost - time.Now().Sub(startTime)

				if timeLeft <= 0 {
					stackTrace := string(debug.Stack())
					return errors.New(fmt.Sprintf("%s: %d ms\n%s", TimeoutError, atMost/time.Millisecond, stackTrace))
				} else {
					go untilWrapper(until, resultChan)
				}
			}
		case <-time.After(timeLeft):
			stackTrace := string(debug.Stack())
			return errors.New(fmt.Sprintf("%s: %d ms\n%s", TimeoutError, atMost/time.Millisecond, stackTrace))
		}
		time.Sleep(pollInterval)
	}
}

// AwaitDefault calls the "Await" function with a default pollInterval of 100 ms and a default atMost timeout
// of 10 seconds.
func AwaitDefault(until func() bool) error {
	return Await(DefaultPollInterval, DefaultAtMost, until)
}

// IsAwaitTimeoutError checks if an error starts with the "TIMEOUT_ERROR" prefix.
func IsAwaitTimeoutError(err error) bool {
	return strings.HasPrefix(err.Error(), TimeoutError)
}

// AwaitPanic calls Await but instead of returning an error it panics.
func AwaitPanic(pollInterval time.Duration, atMost time.Duration, until func() bool) {
	err := Await(pollInterval, atMost, until)

	if err != nil {
		panic(err)
	}
}

func AwaitPanicDefault(until func() bool) {
	AwaitPanic(DefaultPollInterval, DefaultAtMost, until)
}

// AwaitBlocking It runs the "until" function inforeground so if the function runs longer
// than the atMost timeout, await does NOT abort.
// This is a tradeoff to have a determined state, the downside is that the function will
// not time out guranteed.
func AwaitBlocking(pollInterval time.Duration, atMost time.Duration, until func() bool) error {

	if pollInterval <= 0 {
		return fmt.Errorf("PollInterval cannot be 0 or below, got: %d", pollInterval)
	}

	if atMost <= 0 {
		return fmt.Errorf("AtMost timeout cannot be 0 or below, got: %d", atMost)
	}

	if pollInterval > atMost {
		return fmt.Errorf("PollInterval must be smaller than atMost timeout, got: pollInterval=%d, atMost=%d", pollInterval, atMost)
	}

	startTime := time.Now()
	timeLeft := atMost

	for {

		if until() {
			return nil
		} else {
			timeLeft = atMost - time.Now().Sub(startTime)

			if timeLeft <= 0 {
				stackTrace := string(debug.Stack())
				return errors.New(fmt.Sprintf("%s: %d ms\n%s", TimeoutError, atMost/time.Millisecond, stackTrace))
			}
		}

		time.Sleep(pollInterval)
	}
}
