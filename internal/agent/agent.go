package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
	"github.com/btoll/agent-pete/internal/tool"
	_ "modernc.org/sqlite"
)

type MessageStore interface {
	CommitMessage(int, api.ServerMessage) (int, error)
	GetPreviousMessages(*api.ChatRequest, int)
	GetSkills()
}

type Agent struct {
	store *db.DB
}

func New() *Agent {
	return &Agent{
		store: db.New(),
	}
}

func (a *Agent) CallTool(toolCall api.ToolCall) (string, error) {
	funcName := toolCall.Function.Name
	if t, found := tool.Tools[funcName]; found {
		switch t.Function.Name {
		case "Add":
			a, found := toolCall.Function.Arguments["a"].(float64)
			if !found {
				return "", errors.New("argument `a` not found")
			}
			b, found := toolCall.Function.Arguments["b"].(float64)
			if !found {
				return "", errors.New("argument `b` not found")
			}
			return fmt.Sprintf("%v", tool.Add(a, b)), nil
		case "ReadFile":
			if filename, found := toolCall.Function.Arguments["filename"]; found {
				return tool.ReadFile(filename.(string))
			}
			return "", errors.New("argument `filename` not found")
		case "WriteFile":
			filename, found := toolCall.Function.Arguments["filename"]
			if !found {
				return "", errors.New("argument `filename` not found")
			}
			data, found := toolCall.Function.Arguments["data"]
			if !found {
				return "", errors.New("argument `data` not found")
			}
			return tool.WriteFile(filename.(string), data.(string))
		}
	}
	return "", fmt.Errorf("tool `%s` not found", funcName)
}

func (a *Agent) CommitMessage(conversationID int, msg api.ServerMessage) (int, error) {
	return a.store.CommitMessage(conversationID, msg.GetRole(), msg.GetContent())
}

func (a *Agent) ConvertTools(toolMessages []db.ToolMessage) []api.ToolCall {
	tc := make([]api.ToolCall, len(toolMessages))
	for i, toolMessage := range toolMessages {
		var m map[string]any
		_ = json.Unmarshal([]byte(toolMessage.Parameters), &m)
		tc[i] = api.ToolCall{
			ID: toolMessage.ID,
			Function: api.ToolCallFunction{
				Name:      toolMessage.Name,
				Arguments: m,
			},
		}
	}
	return tc
}

func (a *Agent) ExecuteAgent(chatRequest *api.ChatRequest, conversationID int) error {
	for {
		toolCalls, lastID, err := a.ProcessResponse(chatRequest, conversationID)
		if err != nil {
			return &api.InferenceError{
				Backend: "ollama",
				Model:   chatRequest.Model,
				Op:      "processResponse",
				Err:     err,
			}
		}

		if len(toolCalls) == 0 {
			break
		}

		err = a.ProcessToolCalls(chatRequest, toolCalls, lastID)
		if err != nil {
			return err
		}
	}
	return nil
}
func (a *Agent) GetConversationID(convName string) (int, error) {
	return a.store.GetConversationID(convName)
}

func (a *Agent) GetSkills() {
}

func (a *Agent) GetPreviousMessages(request *api.ChatRequest, conversationID int) {
	recentMessages, err := a.store.GetNRecentMessages(conversationID, 30)
	if err != nil {
		panic(err)
	}
	for _, recentMessage := range recentMessages {
		if recentMessage.Role == "assistant" {
			m, err := a.store.GetToolCallsById(recentMessage.ID)
			if err != nil {
				panic(err)
			}
			recentMessage.Tools = m
		}
	}
	for _, msg := range recentMessages {
		request.Messages = append(request.Messages, api.AssistantMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: a.ConvertTools(msg.Tools),
		})
		if len(msg.Tools) > 0 {
			for _, tool := range msg.Tools {
				request.Messages = append(request.Messages, api.ToolMessage{
					Role:       "tool",
					Content:    tool.Result,
					ToolCallID: tool.ID,
				})
			}
		}
	}
}

func (a *Agent) ProcessResponse(chatRequest *api.ChatRequest, conversationID int) ([]api.ToolCall, int, error) {
	resp, err := chatRequest.Post()
	if err != nil {
		return nil, -1, fmt.Errorf("processResponse: %w", err)
	}
	assistantMsg := &api.AssistantMessage{
		Role:      resp.Role,
		Content:   resp.Content,
		ToolCalls: resp.Message.(*api.AssistantMessage).ToolCalls,
	}
	chatRequest.Logger.Debug("processResponse",
		slog.Any("AssistantMessage", assistantMsg),
	)
	chatRequest.Messages = append(chatRequest.Messages, assistantMsg)
	lastID, err := a.store.CommitMessage(conversationID, assistantMsg.Role, assistantMsg.Content)
	if err != nil {
		return nil, -1, err
	}
	return assistantMsg.ToolCalls, lastID, nil
}

func (a *Agent) ProcessToolCalls(chatRequest *api.ChatRequest, toolCalls []api.ToolCall, lastID int) error {
	for _, toolCall := range toolCalls {
		var content string
		res, err := a.CallTool(toolCall)
		if err != nil {
			content = err.Error()
		} else {
			content = res
		}
		chatRequest.Messages = append(chatRequest.Messages, &api.ToolMessage{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Content:    content,
		})
		chatRequest.Logger.Debug("processToolCalls",
			slog.Group("tool",
				slog.String("name", toolCall.Function.Name),
				slog.Any("arguments", toolCall.Function.Arguments),
			),
		)
		b, err := json.Marshal(toolCall.Function.Arguments)
		if err != nil {
			return err
		}
		err = a.store.CommitToolCall(lastID, toolCall.ID, toolCall.Function.Name, string(b), res)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) UpdateMessageStatus(lastID int, status string) error {
	return a.store.UpdateMessageStatus(lastID, status)
}
