package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/btoll/agent-pete/internal/tool"
)

func NewChatRequest(opts ...ConfigOption) *ChatRequest {
	chatRequest := &ChatRequest{
		Messages: []ServerMessage{
			&SystemMessage{
				Role:    "system",
				Content: "You are an agentic coding assistant with access to tools: ReadFile, WriteFile, and Add.\n\nCRITICAL: You must call tools to complete tasks. Do not narrate or describe what you would do — actually call the tools.\n\nWhen asked to run a skill:\n1. Call ReadFile with the skill definition file path (e.g., \"skills/problem-checker/SKILL.md\")\n2. Read and parse the exact content returned from that tool call\n3. Execute the steps described in the skill file using ReadFile and WriteFile\n4. Do not assume or hallucinate file contents — only use what tool calls return\n5. STOP after completing the requested skill. Do not read or execute any other skills unless explicitly asked.\n\nAvailable skills:\n - skills/problem-checker/SKILL.md: Evaluates problem.txt against 4 guidelines and writes results to problem_checker_results.md\n - skills/test-generation/SKILL.md: Generates a test suite in da_training_project_tests/ based on problem.txt with 2-3 intentional misalignments",
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

func (c *ChatRequest) ParseStream(body io.ReadCloser) (*PostResponse, error) {
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
