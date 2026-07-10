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

type ToolNames []string

func (t *ToolNames) Set(name string) error {
	*t = append(*t, name)
	return nil
}

func (t *ToolNames) String() string {
	return strings.Join(*t, ",")
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

func convertTools(toolMessages []db.ToolMessage) []api.ToolCall {
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

func executeAgent(chatRequest *api.ChatRequest, conversationID int) {
	for {
		resp, err := chatRequest.Post()
		if err != nil {
			panic(err)
		}
		toolCalls := resp.Message.(*api.AssistantMessage).ToolCalls
		msg := &api.AssistantMessage{
			Role:      resp.Role,
			Content:   resp.Content,
			ToolCalls: toolCalls,
		}
		chatRequest.Messages = append(chatRequest.Messages, msg)
		lastID, err := db.CommitMessage(conversationID, msg.Role, msg.Content)
		if len(toolCalls) == 0 {
			break
		}
		for _, toolCall := range toolCalls {
			var content string
			res, err := callTool(toolCall)
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
			b, err := json.Marshal(toolCall.Function.Arguments)
			if err != nil {
				panic(err)
			}
			db.CommitToolCall(lastID, toolCall.ID, toolCall.Function.Name, string(b), res)
		}
	}
}

func getConfigOptions() []api.ConfigOption {
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
	return configOptions
}

func getPreviousMessages(chatRequest *api.ChatRequest, conversationID int) {
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
			recentMessage.Tools = m
		}
	}
	for _, msg := range recentMessages {
		chatRequest.Messages = append(chatRequest.Messages, api.AssistantMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: convertTools(msg.Tools),
		})
		if len(msg.Tools) > 0 {
			for _, tool := range msg.Tools {
				chatRequest.Messages = append(chatRequest.Messages, api.ToolMessage{
					Role:       "tool",
					Content:    tool.Result,
					ToolCallID: tool.ID,
				})
			}
		}
	}
}

func isZeroValue[T comparable](v T) bool {
	var zero T
	return zero == v
}

func main() {
	// TODO: I don't like the responsibility of closing the db to be here.
	defer func() {
		if err := db.CloseDatabase(); err != nil {
			log.Fatalf("error closing database: %v\n", err)
		}
	}()

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
		log.Fatalln("User prompt is not optional.")
	}

	if isZeroValue(oneOff) || len(tools) > 0 {
		conversationID, err := db.GetConversationID(convName)
		if err != nil {
			panic(err)
		}
		chatRequest := api.NewChatRequest(getConfigOptions()...)
		if len(tools) > 0 {
			chatRequest.Stream = false
		}
		getPreviousMessages(chatRequest, conversationID)
		msg := api.UserMessage{
			Role:    "user",
			Content: currentMsg,
		}
		chatRequest.Messages = append(chatRequest.Messages, msg)
		_, err = db.CommitMessage(conversationID, msg.Role, msg.Content)
		if err != nil {
			panic(err)
		}
		executeAgent(chatRequest, conversationID)
		return
	}

	generateRequest := api.NewGenerateRequest(currentMsg, getConfigOptions()...)
	err := generateRequest.Post()
	if err != nil {
		panic(err)
	}
}
