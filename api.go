package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	API = "http://localhost:11434/api"

	client = &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).DialContext,
			//			ResponseHeaderTimeout: 3 * time.Second,
			IdleConnTimeout:     30 * time.Second,
			MaxIdleConns:        40,
			MaxConnsPerHost:     20,
			MaxIdleConnsPerHost: 20,
			//			TLSHandshakeTimeout:   5 * time.Second,
		},
	}
)

type ModelResponse struct {
	Model              string  `json:"model"`
	CreatedAt          string  `json:"created_at"`
	Message            Message `json:"message"`
	Done               bool    `json:"done"`
	DoneReason         string  `json:"done_reason"`
	TotalDuration      int     `json:"total_duration"`
	LoadDuration       int     `json:"load_duration"`
	PromptEvalCount    int     `json:"prompt_eval_count"`
	PromptEvalDuration int     `json:"prompt_eval_duration"`
	EvalCount          int     `json:"eval_count"`
	EvalDuration       int     `json:"eval_duration"`
}

type ChatResponse struct {
	Timestamp string
	Role      string
	Content   string
}

func postChat(chatRequest *ChatRequest) (*ChatResponse, error) {
	b, err := json.Marshal(chatRequest)
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

	scanner := bufio.NewScanner(resp.Body)
	var modelResponse ModelResponse
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
			return &ChatResponse{
				Timestamp: modelResponse.CreatedAt,
				Role:      modelResponse.Message.Role,
				Content:   builder.String(),
			}, nil
		}
	}
	return nil, scanner.Err()
}
