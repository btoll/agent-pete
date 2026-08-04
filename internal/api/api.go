package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/btoll/agent-pete/internal/tool"
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

	fastProfile = Profile{
		Temperature: 0.3,
		TopK:        20,
		TopP:        0.9,
		MinP:        0.1,
		NumPredict:  256,
		NumCtx:      2048,
	}

	accurateProfile = Profile{
		Temperature: 0.1,
		TopK:        40,
		TopP:        0.95,
		MinP:        0.05,
		NumPredict:  2048,
		NumCtx:      8192,
	}

	balancedProfile = Profile{
		Temperature: 0.5,
		TopK:        30,
		TopP:        0.9,
		MinP:        0.05,
		NumPredict:  512,
		NumCtx:      4096,
	}
)

type Request struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Options  Profile         `json:"options"`
	Messages []ServerMessage `json:"messages"`
	Tools    []tool.Tool     `json:"tools"`
	Logger   *slog.Logger
}

type Profile struct {
	NumCtx      int      `json:"num_ctx"`
	NumPredict  int      `json:"num_predict"`
	TopK        int      `json:"top_k"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	MinP        float64  `json:"min_p"`
	Stop        []string `json:"stop"` // Defining stop was preventing .content from being populated, it was only .thinking (when enabled).
}

type PostResponse struct {
	Role    string
	Content string
	Message ServerMessage
}

type BaseModelResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ServerMessage `json:"message"`
	Response           string        `json:"response"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason"`
	TotalDuration      int           `json:"total_duration"`
	LoadDuration       int           `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration int           `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       int           `json:"eval_duration"`
}

func (b *BaseModelResponse) UnmarshalJSON(data []byte) error {
	type Alias BaseModelResponse
	aux := struct {
		RawMessage json.RawMessage `json:"message"`
		*Alias
	}{
		Alias: (*Alias)(b),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return &UnmarshalError{
			Op:   "BaseModelResponse.UnmarshalJSON",
			Type: getType(aux),
			Err:  err,
		}
	}
	var msgMap map[string]any
	if err := json.Unmarshal(aux.RawMessage, &msgMap); err != nil {
		return &UnmarshalError{
			Op:   "BaseModelResponse.UnmarshalJSON",
			Type: getType(msgMap),
			Err:  err,
		}
	}
	role, ok := msgMap["role"].(string)
	if !ok {
		return &ParseError{
			Op:        "BaseModelResponse.UnmarshalJSON",
			Retryable: false,
			Err:       fmt.Errorf("no `role` field in message: %s", role),
		}
	}
	switch role {
	case "system":
		sm := &SystemMessage{}
		if err := json.Unmarshal(aux.RawMessage, sm); err != nil {
			return &UnmarshalError{
				Op:   "BaseModelResponse.UnmarshalJSON",
				Type: getType(sm),
				Err:  err,
			}
		}
		b.Message = sm
	case "user":
		um := &UserMessage{}
		if err := json.Unmarshal(aux.RawMessage, um); err != nil {
			return &UnmarshalError{
				Op:   "BaseModelResponse.UnmarshalJSON",
				Type: getType(um),
				Err:  err,
			}
		}
		b.Message = um
	case "assistant":
		am := &AssistantMessage{}
		if err := json.Unmarshal(aux.RawMessage, am); err != nil {
			return &UnmarshalError{
				Op:   "BaseModelResponse.UnmarshalJSON",
				Type: getType(am),
				Err:  err,
			}
		}
		b.Message = am
	case "tool":
		tm := &ToolMessage{}
		if err := json.Unmarshal(aux.RawMessage, tm); err != nil {
			return &UnmarshalError{
				Op:   "BaseModelResponse.UnmarshalJSON",
				Type: getType(tm),
				Err:  err,
			}
		}
		b.Message = tm
	default:
		return &ParseError{
			Op:        "BaseModelResponse.UnmarshalJSON",
			Retryable: false,
			Err:       fmt.Errorf("unknown role: %s", role),
		}
	}
	return nil
}

// Compile-time assertion, ensure that *Message implements ServerMessage without constructing
// a value (no allocation).  Checks method-set compatibililty for *Message.
// var _ Interface = (*T)(nil)
var _ ServerMessage = (*SystemMessage)(nil)
var _ ServerMessage = (*UserMessage)(nil)
var _ ServerMessage = (*AssistantMessage)(nil)
var _ ServerMessage = (*ToolMessage)(nil)

type ServerMessage interface {
	GetContent() string
	GetRole() string
}

type SystemMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (s SystemMessage) GetContent() string {
	return s.Content
}

func (s SystemMessage) GetRole() string {
	return "system"
}

type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (u UserMessage) GetContent() string {
	return u.Content
}

func (u UserMessage) GetRole() string {
	return "user"
}

type AssistantMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

func (a AssistantMessage) GetContent() string {
	return a.Content
}

func (a AssistantMessage) GetRole() string {
	return "assistant"
}

type ToolMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

func (t ToolMessage) GetContent() string {
	return t.Content
}

func (t ToolMessage) GetRole() string {
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
