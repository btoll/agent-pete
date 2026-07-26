package main

import (
	"flag"
	"log"

	_ "modernc.org/sqlite"

	"github.com/btoll/agent-pete/internal/agent"
	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
)

var (
	currentMsg     string
	model          string
	profile        string
	sessionName    string
	createDatabase bool
	debug          bool
	stream         bool
)

func getAgentOptions() []agent.AgentOptions {
	return append([]agent.AgentOptions{},
		agent.WithDebug(debug),
		agent.WithSessionName(sessionName),
		//		agent.WithSkillsDir(dirName),
	)
}

func getRequestOptions() []api.RequestOptions {
	return append([]api.RequestOptions{},
		api.WithModel(model),
		api.WithProfile(profile),
		api.WithStream(stream),
	)
}

//func isZeroValue[T comparable](v T) bool {
//	var zero T
//	return zero == v
//}

func main() {
	// TODO: I don't like the responsibility of closing the db to be here.
	defer func() {
		if err := db.CloseDatabase(); err != nil {
			log.Fatalf("error closing database: %v\n", err)
		}
	}()

	flag.StringVar(&currentMsg, "m", "", "The newest message to append to the prompt.")
	flag.StringVar(&model, "model", "", "The model.")
	flag.StringVar(&profile, "profile", "", "The profile tunes the runtime options that control text generation (fast|accurate|balanced).")
	flag.StringVar(&sessionName, "name", "", "The name of the session, used for grouping related messages.")
	flag.BoolVar(&createDatabase, "create-database", false, "Create the database.  Useful for debugging.")
	flag.BoolVar(&debug, "debug", false, "Turn on verbose logging.")
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API (/chat).")
	flag.Parse()

	if createDatabase {
		db.CreateDatabase()
		return
	}

	err := agent.New(
		getAgentOptions(),
		getRequestOptions(),
	).Loop()
	if err != nil {
		panic(err)
	}
}
