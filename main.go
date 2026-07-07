package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
	"github.com/btoll/agent-pete/internal/tool"
	_ "modernc.org/sqlite"
)

func isZeroValue[T comparable](v T) bool {
	var zero T
	return zero == v
}

type ToolNames []string

func (t *ToolNames) Set(name string) error {
	*t = append(*t, name)
	return nil
}

func (t *ToolNames) String() string {
	return strings.Join(*t, ",")
}

func convertMessages(conversationID int, messages []api.Message) []db.Message {
	m := []db.Message{}
	for _, message := range messages {
		// Tool messages are not stored in the database.  However, if an assistant message
		// has associated tool calls, those are stored in the database isn the tool_calls
		// table with a foreign key (message_id) pointing to the assistant message.
		// See db.Commit()
		if message.Role == "tool" {
			continue
		}
		var toolMessages []db.ToolMessage
		if len(message.ToolCalls) > 0 {
			toolMessages = make([]db.ToolMessage, len(message.ToolCalls))
			for i, toolCall := range message.ToolCalls {
				b, _ := json.Marshal(toolCall.Function.Arguments)
				toolMessages[i] = db.ToolMessage{
					ID:         toolCall.ID,
					Name:       toolCall.Function.Name,
					Parameters: string(b),
				}
			}
		}
		m = append(m, db.Message{
			ConversationID: conversationID,
			Role:           message.Role,
			Content:        message.Content,
			Tools:          toolMessages,
		})
	}
	return m
}

func convertTools(toolMessages []db.ToolMessage) []api.ToolCall {
	tc := make([]api.ToolCall, len(toolMessages))
	for i, toolMessage := range toolMessages {
		var m map[string]any
		_ = json.Unmarshal([]byte(toolMessage.Parameters), &m)
		tc[i] = api.ToolCall{
			ID: toolMessage.ID,
			Function: api.Function2{
				Name:      toolMessage.Name,
				Arguments: m,
			},
		}
	}
	return tc
}

func callTool(toolCall api.ToolCall) (string, error) {
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
		}
	}
	return "", fmt.Errorf("tool `%s` not found", funcName)
}

func main() {
	var (
		convName            string
		currentMsg          string
		model               string
		createDatabase      bool
		oneOff              bool
		stream              bool
		totalResponseTokens int
		tools               ToolNames
	)

	flag.StringVar(&currentMsg, "m", "", "The newest message to append to the prompt.")
	flag.StringVar(&model, "model", "mistral", "The model.")
	flag.BoolVar(&createDatabase, "create-database", false, "Create the database.  Useful for debugging.")
	flag.BoolVar(&oneOff, "one-off", false, "Don't include previous messages in the prompt (/generate).")
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API (/chat).")
	flag.IntVar(&totalResponseTokens, "tokens", 0, "Total number of response tokens.")
	flag.StringVar(&convName, "conv", "default", "Conversation ID for grouping related messages.")
	flag.Var(&tools, "tool", "The name of a tool (function).  Can accept specified multiple times.  Primarily used for debugging, but it can help limit tokens spent by reducing the request payload.")
	flag.Parse()

	if createDatabase {
		db.CreateDatabase()
		return
	}

	if isZeroValue(currentMsg) {
		log.Fatalln("You must ask a question.")
	}

	conversationID, err := db.GetConversationID(convName)
	if err != nil {
		panic(err)
	}
	var configOptions []api.ConfigOption
	if !isZeroValue(model) {
		configOptions = append(configOptions, api.WithModel(model))
	}
	if isZeroValue(stream) {
		configOptions = append(configOptions, api.WithStream(stream))
	}
	if !isZeroValue(totalResponseTokens) {
		configOptions = append(configOptions, api.WithTotalResponseTokens(totalResponseTokens))
	}
	if len(tools) > 0 {
		t := make([]string, 0, len(tools))
		for _, name := range tools {
			t = append(t, name)
		}
		// I'm changing this to a generic type b/c I don't want the api
		// package to have to import ToolNames (I want to keep it in main).
		configOptions = append(configOptions, api.WithTools(t))
	}

	if isZeroValue(oneOff) || len(tools) > 0 {
		chatRequest := api.NewChatRequest(configOptions...)
		if len(tools) > 0 {
			chatRequest.Stream = false
		}
		recentMessages, err := db.GetNRecentMessages(conversationID, 30)
		if err != nil {
			panic(err)
		}
		for _, recentMessage := range recentMessages {
			if recentMessage.Role == "assistant" {
				m, err := db.GetToolCallsById(recentMessage.ID)
				if err != nil {
					panic(err)
				}
				fmt.Printf("m=%#v\n", m)
				recentMessage.Tools = m
			}
		}
		for _, msg := range recentMessages {
			toolCalls := convertTools(msg.Tools)
			chatRequest.Messages = append(chatRequest.Messages, api.Message{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: toolCalls,
			})
			if len(toolCalls) > 0 {
				m := make([]api.Message, len(toolCalls))
				for i, toolCall := range toolCalls {
					m[i] = api.Message{
						Role:       "tool",
						Content:    "",
						ToolCallID: toolCall.ID,
					}
				}
				chatRequest.Messages = append(chatRequest.Messages, m...)
			}
		}
		chatRequest.Messages = append(chatRequest.Messages, api.Message{
			Role:    "user",
			Content: currentMsg,
		})
		seed := len(chatRequest.Messages)
		fmt.Printf("chatRequest=%#v\n", chatRequest)

		for {
			resp, err := chatRequest.Post()
			if err != nil {
				panic(err)
			}
			chatRequest.Messages = append(chatRequest.Messages, api.Message{
				Role:      resp.Role,
				Content:   resp.Content,
				ToolCalls: resp.Message.ToolCalls,
			})
			if len(resp.Message.ToolCalls) == 0 {
				err := db.Commit(
					convertMessages(
						conversationID,
						chatRequest.Messages[seed-1:],
					),
				)
				if err != nil {
					panic(err)
				}
				break
			}
			for _, toolCall := range resp.Message.ToolCalls {
				var content string
				res, err := callTool(toolCall)
				if err != nil {
					content = err.Error()
				} else {
					content = res
				}
				chatRequest.Messages = append(chatRequest.Messages, api.Message{
					Role:       "tool",
					ToolCallID: toolCall.ID,
					Content:    content,
				})
			}
		}

		return
	}

	generateRequest := api.NewGenerateRequest(currentMsg, configOptions...)
	err = generateRequest.Post()
	if err != nil {
		panic(err)
	}
}
