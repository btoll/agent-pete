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
	"strings"

	"github.com/btoll/agent-pete/internal/tool"
)

func NewChatRequest(opts ...ConfigOption) *ChatRequest {
	chatRequest := &ChatRequest{
		Messages: []OllamaMessage{
			&SystemMessage{
				Role:    "system",
				Content: "You are a helpful assistant with access to tools. When a user asks you to perform a task that matches an available tool, you must call that tool by providing the tool name and parameters in the specified format.",
			},
		},
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
	// Only include all of the tool definitions if the user has NOT specified any
	// on the CLI (`-tool`).
	// Why only include all when nothing has been specified?  Because sending all
	// of the tools could potentially greatly increase the token use versus only
	// sending a few.
	// (Of course, this doesn't matter with ollama and public agents allow you to
	// "cache" the system prompt and tool definitions.)
	if chatRequest.Tools == nil {
		for _, tool := range tool.Tools {
			chatRequest.Tools = append(chatRequest.Tools, tool)
		}
	}
	return chatRequest
}

func (c *ChatRequest) Post() (*PostResponse, error) {
	//	fmt.Println()
	//	fmt.Printf("c=%#v\n", c)
	//	fmt.Println()
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
	var builder strings.Builder
	var allToolCalls []ToolCall
	for scanner.Scan() {
		var modelResponse BaseModelResponse
		if err := json.Unmarshal(scanner.Bytes(), &modelResponse); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			continue
		}

		if assistantMsg, ok := modelResponse.Message.(*AssistantMessage); ok {
			allToolCalls = append(allToolCalls, assistantMsg.ToolCalls...)
		}
		//		allToolCalls = append(allToolCalls, modelResponse.Message.ToolCalls...)

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
	return nil, scanner.Err()
}
