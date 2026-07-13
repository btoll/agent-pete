package api

import (
	"errors"
	"fmt"
	"net"
)

var (
	ErrMarshal   = errors.New("json marshal error")
	ErrUnmarshal = errors.New("json unmarshal error")
	ErrParse     = errors.New("parse error")
)

func getType(v any) string {
	return fmt.Sprintf("%T", v)
}

func isRetryable(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

type InferenceError struct {
	Backend   string // e.g., "ollama"
	Model     string // e.g., "llama2"
	Op        string // e.g., "model_load", "generate", "pull"
	Retryable bool   // Hint: is this transient (retry-worthy)?
	Err       error  // Wrapped error (could be *NetworkError or domain-specific)
}

func (ie *InferenceError) Error() string {
	return fmt.Sprintf("inference error on %s (model: %s, op: %s): %v", ie.Backend, ie.Model, ie.Op, ie.Err)
}

func (ie *InferenceError) Unwrap() error {
	return ie.Err
}

type HTTPError struct {
	Op         string
	StatusCode int
	Retryable  bool
}

func (he *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error on %s with status code %d", he.Op, he.StatusCode)
}

type NetworkError struct {
	Op        string
	Retryable bool
	Err       error
}

func (ne *NetworkError) Error() string {
	return fmt.Sprintf("network error on %s: %v", ne.Op, ne.Err)
}

type ParseError struct {
	Op        string
	Retryable bool
	Err       error
}

func (pe *ParseError) Error() string {
	return fmt.Sprintf("parse error on %s: %v", pe.Op, pe.Err)
}

type UnmarshalError struct {
	Op        string
	Type      string
	Retryable bool
	Err       error
}

func (ue *UnmarshalError) Error() string {
	return fmt.Sprintf("unmarshal error on %s: %v", ue.Op, ue.Err)
}
