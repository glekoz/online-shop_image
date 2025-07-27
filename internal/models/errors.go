package models

import (
	"errors"
)

var (
	ErrOSAction        = errors.New("OS action failed")
	ErrOperationAction = errors.New("operation action failed")
	ErrNetworkAction   = errors.New("network action failed")

	ErrDoRetry    = errors.New("it's OK to retry")
	ErrDoNotRetry = errors.New("don't try retrying")
)
