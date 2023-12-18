package util

import (
	"time"

	"yunion.io/x/pkg/errors"
)

func Wait(interval time.Duration, timeout time.Duration, callback func() (bool, error)) error {
	startTime := time.Now()
	for time.Now().Sub(startTime) < timeout {
		ok, err := callback()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(interval)
	}
	return errors.ErrTimeout
}
