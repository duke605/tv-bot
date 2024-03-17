package main

import (
	"context"
	"strconv"
	"time"
)

func TryAll(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

func PP[T any](t T) *T {
	return &t
}

func Omit(m map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		delete(m, key)
	}

	return m
}

// Send waits until t is successfully sent through the channel or until the context is done. If t was
// sent successfully true is returned. If the context was cancelled before t could be sent false is returned
func Send[T any](ctx context.Context, ch chan<- T, t T) bool {
	select {
	case ch <- t:
		return true
	case <-ctx.Done():
		return false
	}
}

func HumanDuration(d time.Duration) string {
	str := ""
	if h := d / time.Hour; h >= 1 {
		str += strconv.FormatInt(int64(h), 10) + "h"
		d = d % time.Hour
	}

	if m := d / time.Minute; m >= 1 {
		str += strconv.FormatInt(int64(m), 10) + "m"
		d = d % time.Minute
	}

	if s := d / time.Second; s >= 1 {
		str += strconv.FormatInt(int64(s), 10) + "s"
	}

	return str
}
