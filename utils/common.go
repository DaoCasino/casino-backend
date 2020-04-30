package utils

import (
    "fmt"
    "time"
)

func WithTimeout(f func() error, timeout time.Duration) error {
    ch := make(chan error)
    go func() {
        ch <- f()
    }()
    select {
    case <-time.After(timeout):
        return fmt.Errorf("timeout reached")
    case <-ch:
        return nil
    }
}