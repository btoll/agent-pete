package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"github.com/btoll/agent-pete/internal/agent"
	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
	_ "modernc.org/sqlite"
)

var (
	convName            string
	currentMsg          string
	model               string
	createDatabase      bool
	debug               bool
	oneOff              bool
	repl                bool
	stream              bool
	totalResponseTokens int
	tools               ToolNames

	logger   *slog.Logger
	logLevel *slog.LevelVar = new(slog.LevelVar)
)

type ToolNames []string

func (t *ToolNames) Set(name string) error {
	*t = append(*t, name)
	return nil
}

func (t *ToolNames) String() string {
	return strings.Join(*t, ",")
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

func isZeroValue[T comparable](v T) bool {
	var zero T
	return zero == v
}

func main() {
	// TODO: I don't like the responsibility of closing the db to be here.
	//	defer func() {
	//		if err := db.CloseDatabase(); err != nil {
	//			log.Fatalf("error closing database: %v\n", err)
	//		}
	//	}()

	flag.StringVar(&currentMsg, "m", "", "The newest message to append to the prompt.")
	flag.StringVar(&model, "model", "mistral", "The model.")
	flag.BoolVar(&createDatabase, "create-database", false, "Create the database.  Useful for debugging.")
	flag.BoolVar(&debug, "debug", false, "Turn on verbose logging.")
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

	agent := agent.New()

	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	if debug {
		logLevel.Set(slog.LevelDebug)
	}
	slog.SetDefault(logger)

	if isZeroValue(oneOff) || len(tools) > 0 {
		conversationID, err := agent.GetConversationID(convName)
		if err != nil {
			log.Panicf("let's panic here %v\n", err)
		}

		chatRequest := api.NewChatRequest(
			logger.WithGroup("api").With(slog.String("type", "ChatRequest")),
			getConfigOptions()...,
		)
		//		if len(tools) > 0 {
		//			chatRequest.Stream = false
		//		}
		agent.GetPreviousMessages(chatRequest, conversationID)
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
				lastID, err := agent.CommitMessage(conversationID, msg)
				if err != nil {
					panic(err)
				}
				maxRetries := 3
				var loopErr error
			OuterLoop:
				for n := range maxRetries + 1 {
					loopErr = agent.ExecuteAgent(chatRequest, conversationID)
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
				err = agent.UpdateMessageStatus(lastID, status)
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
		_, err = agent.CommitMessage(conversationID, msg)
		if err != nil {
			panic(err)
		}
		agent.ExecuteAgent(chatRequest, conversationID)
		return
	}

	generateRequest := api.NewGenerateRequest(
		currentMsg,
		logger.WithGroup("api").With(slog.String("type", "GenerateRequest")),
		getConfigOptions()...,
	)
	err := generateRequest.Post()
	if err != nil {
		panic(err)
	}
}
