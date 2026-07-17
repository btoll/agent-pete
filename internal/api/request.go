package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/btoll/agent-pete/internal/tool"
)

func NewRequest(tools map[string]tool.Tool, logger *slog.Logger, opts ...ConfigOption) *Request {
	request := &Request{
		Logger:   logger,
		Messages: []ServerMessage{},
		Model:    "mistral",
		Stream:   true,
		Options: RequestOptions{
			NumPredict:  300, // Limit to ~300 tokens max.
			Temperature: 0.3, // Lower temp = more deterministic, shorter responses.
			TopP:        0.5, // Reduce diversity.
		},
	}
	for _, opt := range opts {
		opt(request)
	}
	for _, tool := range tool.Tools {
		request.Tools = append(request.Tools, tool)
	}
	return request
}

func (c *Request) Post() (*PostResponse, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("POST: %w", errors.Join(ErrMarshal, err))
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, API+"/chat", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("POST: http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, &NetworkError{
			Op:        "client.Do",
			Retryable: isRetryable(err),
			Err:       err,
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, &HTTPError{
			Op:         "POST",
			StatusCode: resp.StatusCode,
			Retryable:  resp.StatusCode >= 500,
		}
	}
	return c.ParseStream(resp.Body)
}

func (c *Request) ParseStream(body io.ReadCloser) (*PostResponse, error) {
	scanner := bufio.NewScanner(body)
	var builder strings.Builder
	var allToolCalls []ToolCall
	for scanner.Scan() {
		modelResponse := &BaseModelResponse{}
		if err := modelResponse.UnmarshalJSON(scanner.Bytes()); err != nil {
			return nil, fmt.Errorf("ParseStream: %w", err)
		}

		if assistantMsg, ok := modelResponse.Message.(*AssistantMessage); ok {
			allToolCalls = append(allToolCalls, assistantMsg.ToolCalls...)
		}
		os.Stdout.WriteString(modelResponse.Message.GetContent())
		builder.WriteString(modelResponse.Message.GetContent())

		// Both streaming and non-streaming APIs will work as long as we capture before
		// Done: true (actually, non-streaming is the one that requires this.
		if modelResponse.Done {
			if assistantMsg, ok := modelResponse.Message.(*AssistantMessage); ok {
				assistantMsg.ToolCalls = allToolCalls
			}
			return &PostResponse{
				Role:    modelResponse.Message.GetRole(),
				Content: builder.String(),
				Message: modelResponse.Message,
			}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, &ParseError{
			Op:        "scanner.Err",
			Retryable: false,
			Err:       err,
		}
	}

	return nil, &ParseError{
		Op:        "ParseStream",
		Retryable: true,
		Err:       ErrParse,
	}
}
