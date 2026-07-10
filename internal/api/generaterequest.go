package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, API+"/generate", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("Bad request, status code %d\n", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	var modelResponse BaseModelResponse
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &modelResponse); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			continue
		}
		os.Stdout.WriteString(modelResponse.Response)
	}
	return scanner.Err()
}
