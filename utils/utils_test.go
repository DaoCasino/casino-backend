package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithTimeout(t *testing.T) {
	assert := assert.New(t)
	slowFunction := func() error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}
	err := WithTimeout(slowFunction, time.Millisecond)
	assert.NotNil(err)
	assert.Equal("timeout reached", err.Error())

	err = WithTimeout(slowFunction, 3*time.Millisecond)
	assert.Nil(err)
}

func TestRetry(t *testing.T) {
	assert := assert.New(t)
	failer := func(times int) func() error {
		return func() error {
			if times == 0 {
				return nil
			}
			times--
			return fmt.Errorf("fail amount is more than zero")
		}
	}
	err := Retry(failer(3), 3, time.Millisecond)
	assert.NotNil(err)
	assert.Equal("fail amount is more than zero", err.Error())

	err = Retry(failer(3), 4, time.Millisecond)
	assert.Nil(err)
}

func TestRetryWithTimeout(t *testing.T) {
	assert := assert.New(t)
	failer := func(times int, d time.Duration) func() error {
		return func() error {
			if times == 0 {
				return nil
			}
			times--
			time.Sleep(d)
			return fmt.Errorf("fail amount is more than zero")
		}
	}
	assert.NotNil(RetryWithTimeout(failer(3, 2*time.Millisecond), 3, time.Millisecond, time.Millisecond))
	assert.Nil(RetryWithTimeout(failer(3, 2*time.Millisecond), 4, time.Millisecond, time.Millisecond))
	assert.NotNil(RetryWithTimeout(failer(3, time.Millisecond), 1, 3*time.Millisecond, time.Millisecond))
}
