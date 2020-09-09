package utils

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

func WithTimeout(f func() error, timeout time.Duration) error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	select {
	case <-time.After(timeout):
		return fmt.Errorf("timeout reached")
	case e := <-ch:
		return e
	}
}

func Retry(f func() error, n int, retryDelay time.Duration) error {
	var e error
	for n > 0 {
		if e = f(); e == nil {
			return nil
		}
		n--
		log.Debug().Msgf("Retrying, retries left: %v, error: %v", n, e.Error())
		time.Sleep(retryDelay)
	}
	return e
}

func RetryWithTimeout(f func() error, n int, timeout, retryDelay time.Duration) error {
	var e error
	for n > 0 {
		if e = WithTimeout(f, timeout); e == nil {
			return nil
		}
		n--
		log.Debug().Msgf("Retrying, retries left: %v, error: %v", n, e.Error())
		time.Sleep(retryDelay)
	}
	return e
}
