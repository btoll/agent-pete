package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type PostResponse struct {
	Timestamp string
	Role      string
	Content   string
}

type ChatRequest struct {
	Request
	Messages []ChatMessage `json:"messages"`
}

func NewChatRequest(opts ...ConfigOption) *ChatRequest {
	chatRequest := &ChatRequest{
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
		opt(&chatRequest.Request)
	}
	return chatRequest
}

func (c *ChatRequest) GetRecentMessages(conversationID string, limit int) ([]ChatMessage, error) {
	return getNRecentMessages(conversationID, limit)
}

func (c *ChatRequest) Post() (*PostResponse, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, API+"/chat", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Bad request, status code %d\n", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	var modelResponse BaseModelResponse
	var builder strings.Builder
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &modelResponse); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			continue
		}
		os.Stdout.WriteString(modelResponse.Message.Content)
		builder.WriteString(modelResponse.Message.Content)
		// Both streaming and non-streaming APIs will work as long as we capture before
		// Done: true (actually, non-streaming is the one that requires this.
		if modelResponse.Done {
			return &PostResponse{
				Timestamp: modelResponse.CreatedAt,
				Role:      modelResponse.Message.Role,
				Content:   builder.String(),
			}, nil
		}
	}
	return nil, scanner.Err()
}
