package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func NewGenerateRequest(msg string, opts ...ConfigOption) *GenerateRequest {
	generateRequest := &GenerateRequest{
		Prompt: msg,
		Request: Request{
			Model:  "mistral",
			Stream: true,
			Options: RequestOptions{
				NumPredict:  300, // Limit to ~300 tokens max.
				Temperature: 0.3, // Lower temp = more deterministic, shorter responses.
				TopP:        0.5, // Reduce diversity.
			},
		},
	}
	for _, opt := range opts {
		opt(&generateRequest.Request)
	}
	return generateRequest
}

func (g *GenerateRequest) Post() error {
	b, err := json.Marshal(g)
	if err != nil {
		return fmt.Errorf("POST: %w", errors.Join(ErrMarshal, err))
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, API+"/generate", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST: http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return &NetworkError{
			Op:        "client.Do",
			Retryable: isRetryable(err),
			Err:       err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return &HTTPError{
			Op:         "POST",
			StatusCode: resp.StatusCode,
			Retryable:  resp.StatusCode >= 500,
		}
	}
	return g.ParseStream(resp.Body)
}

func (g *GenerateRequest) ParseStream(body io.ReadCloser) error {
	scanner := bufio.NewScanner(body)
	var modelResponse BaseModelResponse
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &modelResponse); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			return fmt.Errorf("ParseStream: %w", err)
		}
		os.Stdout.WriteString(modelResponse.Response)
	}
	if err := scanner.Err(); err != nil {
		return &ParseError{
			Op:        "scanner.Err",
			Retryable: false,
			Err:       err,
		}
	}
	return nil
}
