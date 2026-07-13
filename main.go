package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

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
	repl                bool
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

func executeAgent(chatRequest *api.ChatRequest, conversationID int) error {
	for {
		toolCalls, lastID, err := processResponse(chatRequest, conversationID)
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

		err = processToolCalls(chatRequest, toolCalls, lastID)
		if err != nil {
			return err
		}
	}
	return nil
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

func processResponse(chatRequest *api.ChatRequest, conversationID int) ([]api.ToolCall, int, error) {
	resp, err := chatRequest.Post()
	if err != nil {
		return nil, -1, fmt.Errorf("processResponse: %w", err)
	}
	msg := &api.AssistantMessage{
		Role:      resp.Role,
		Content:   resp.Content,
		ToolCalls: resp.Message.(*api.AssistantMessage).ToolCalls,
	}
	chatRequest.Messages = append(chatRequest.Messages, msg)
	lastID, err := db.CommitMessage(conversationID, msg.Role, msg.Content)
	if err != nil {
		return nil, -1, err
	}
	return msg.ToolCalls, lastID, nil
}

func processToolCalls(chatRequest *api.ChatRequest, toolCalls []api.ToolCall, lastID int) error {
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
			return err
		}
		err = db.CommitToolCall(lastID, toolCall.ID, toolCall.Function.Name, string(b), res)
		if err != nil {
			return err
		}
	}
	return nil
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
	flag.BoolVar(&repl, "repl", false, "Communicate with the model in a REPL interface.")
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API (/chat).")
	flag.IntVar(&totalResponseTokens, "tokens", 0, "Total number of response tokens.")
	flag.StringVar(&convName, "conv", "repl", "Conversation ID for grouping related messages.")
	flag.Var(&tools, "tool", "The name of a tool (function).  Can accept specified multiple times.  Primarily used for debugging, but it can help limit tokens spent by reducing the request payload.")
	flag.Parse()

	if createDatabase {
		db.CreateDatabase()
		return
	}

	if isZeroValue(oneOff) || len(tools) > 0 {
		conversationID, err := db.GetConversationID(convName)
		if err != nil {
			log.Panicf("let's panic here %v\n", err)
		}

		chatRequest := api.NewChatRequest(getConfigOptions()...)
		if len(tools) > 0 {
			chatRequest.Stream = false
		}
		getPreviousMessages(chatRequest, conversationID)
		if repl {
			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Printf("\nagent-pete > ")
				if !scanner.Scan() {
					break
				}
				if err := scanner.Err(); err != nil {
					log.Fatalf("scanner error: %v\n", err)
					break
				}
				msg := api.UserMessage{
					Role:    "user",
					Content: scanner.Text(),
				}
				chatRequest.Messages = append(chatRequest.Messages, msg)
				lastID, err := db.CommitMessage(conversationID, msg.Role, msg.Content)
				if err != nil {
					panic(err)
				}
				maxRetries := 3
				var loopErr error
			OuterLoop:
				for n := range maxRetries + 1 {
					loopErr = executeAgent(chatRequest, conversationID)
					if loopErr != nil {
						var networkErr *api.NetworkError
						var httpErr *api.HTTPError
						var parseErr *api.ParseError
						var unmarshalErr *api.UnmarshalError
						switch {
						case errors.As(loopErr, &networkErr):
							if !networkErr.Retryable {
								break OuterLoop
							}
						case errors.As(loopErr, &httpErr):
							if !httpErr.Retryable {
								break OuterLoop
							}
						case errors.As(loopErr, &parseErr):
							break OuterLoop
						case errors.As(loopErr, &unmarshalErr):
							break OuterLoop
						default:
							break OuterLoop
						}
						time.Sleep(time.Duration(math.Exp2(float64(n))) * time.Second)
						continue
					}
					break
				}
				status := "success"
				if loopErr != nil {
					status = "failed"
				} else {
				}
				err = db.UpdateMessageStatus(lastID, status)
				if err != nil {
					// TODO
				}
			}
			return
		}
		if isZeroValue(currentMsg) {
			log.Fatalln("User prompt is not optional.")
		}
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
