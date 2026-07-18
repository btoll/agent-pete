package db

import "fmt"

type StoreError struct {
	Op        string
	Retryable bool
	Err       error
}

func (se *StoreError) Error() string {
	return fmt.Sprintf("store error on %s op: %v", se.Op, se.Err)
}
