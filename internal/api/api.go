package api

import (
	"net"
	"net/http"
	"time"

	"github.com/btoll/agent-pete/internal/tool"
)

// Compile-time assertion, ensure that *biplane implements gamepiece without constructing
// a value (no allocation).  Checks method-set compatibililty for *biplane.
// var _ Interface = (*T)(nil)
//var _ gamepiece = (*biplane)(nil)

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

type Request struct {
	Model   string         `json:"model"`
	Stream  bool           `json:"stream"`
	Options RequestOptions `json:"options"`
	Tools   []tool.Tool    `json:"tools"`
}

type RequestOptions struct {
	//	Seed        int     `json:"seed"`
	Temperature float64 `json:"temperature"`
	//	TopK        int     `json:"top_k"`
	TopP float64 `json:"top_p"`
	//	MinP        float64 `json:"min_p"`
	//	Stop        string  `json:"stop"`
	//	NumCtx      int     `json:"num_ctx"`
	NumPredict int `json:"num_predict"`
}

type ChatRequest struct {
	Request
	Messages []OllamaMessage `json:"messages"`
}

type GenerateRequest struct {
	Request
	Prompt string `json:"prompt"`
	Think  bool   `json:"think"`
}

type PostResponse struct {
	Role    string
	Content string
	Message OllamaMessage
}

type BaseModelResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            OllamaMessage `json:"message"`
	Response           string        `json:"response"`
	Thinking           string        `json:"thinking"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason"`
	TotalDuration      int           `json:"total_duration"`
	LoadDuration       int           `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration int           `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       int           `json:"eval_duration"`
}

type OllamaMessage interface {
	GetContent() string
	GetRole() string
}

type SystemMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (s *SystemMessage) GetContent() string {
	return s.Content
}

func (s *SystemMessage) GetRole() string {
	return "system"
}

type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (u *UserMessage) GetContent() string {
	return u.Content
}

func (u *UserMessage) GetRole() string {
	return "user"
}

type AssistantMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

func (a *AssistantMessage) GetContent() string {
	return a.Content
}

func (a *AssistantMessage) GetRole() string {
	return "assistant"
}

type ToolMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

func (t *ToolMessage) GetContent() string {
	return t.Content
}

func (t *ToolMessage) GetRole() string {
	return "tool"
}

type ToolCall struct {
	ID       string           `json:"id"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Arguments   map[string]any `json:"arguments"`
}
