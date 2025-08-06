package models

import (
	"errors"
	"fmt"
)

const (
	// Class 02 — No Data (this is also a warning class per the SQL standard)
	NoData                                = "02000"
	NoAdditionalDynamicResultSetsReturned = "02001"
	// Class 08 — Connection Exception
	ConnectionException                           = "08000"
	ConnectionDoesNotExist                        = "08003"
	ConnectionFailure                             = "08006"
	SQLClientUnableToEstablishSQLConnection       = "08001"
	SQLServerRejectedEstablishmentOfSQLConnection = "08004"
	TransactionResolutionUnknown                  = "08007"
	ProtocolViolation                             = "08P01"
	// Class 23 — Integrity Constraint Violation
	IntegrityConstraintViolation = "23000"
	RestrictViolation            = "23001"
	NotNullViolation             = "23502"
	UniqueViolation              = "23505"
)

var (
	ErrNotFound        = errors.New("no result found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrFS              = errors.New("file system misbehaved")
	ErrDoNotRetry      = errors.New("do not retry")
	ErrUniqueViolation = errors.New("unique violation")
)

type Error struct {
	Location   string
	FailedData string
	Err        error
}

func NewError(location string, fd string, err error) Error {
	return Error{Location: location, FailedData: fd, Err: err}
}

func (e Error) Error() string {
	return fmt.Sprintf("\nIn |%s| occured |%s| with following data: %s", e.Location, e.Err.Error(), e.FailedData)
}

func (e Error) Unwrap() error {
	return e.Err
}
