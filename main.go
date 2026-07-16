package main

import (
	"flag"
	"log/slog"

	_ "modernc.org/sqlite"

	"github.com/btoll/agent-pete/internal/agent"
	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
)

var (
	convName            string
	currentMsg          string
	model               string
	createDatabase      bool
	debug               bool
	stream              bool
	totalResponseTokens int

	logger   *slog.Logger
	logLevel *slog.LevelVar = new(slog.LevelVar)
)

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
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API (/chat).")
	flag.IntVar(&totalResponseTokens, "tokens", 0, "Total number of response tokens.")
	flag.StringVar(&convName, "conv", "repl", "Conversation ID for grouping related messages.")
	flag.Parse()

	if createDatabase {
		db.CreateDatabase()
		return
	}

	//	if debug {
	//		logLevel.Set(slog.LevelDebug)
	//	}
	//	slog.SetDefault(logger)

	agent.New(
		convName,
		getConfigOptions(),
		logLevel,
	).Loop()
}
