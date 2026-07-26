package agent

import (
	"log/slog"
	"os"
)

type AgentOptions func(*Agent)

func WithDebug(debug bool) AgentOptions {
	return func(a *Agent) {
		logLevel := new(slog.LevelVar)
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		}))
		if debug {
			logLevel.Set(slog.LevelDebug)
		}
		slog.SetDefault(logger)
		a.SetLogger(logger)
	}
}

func WithSessionName(sessionName string) AgentOptions {
	return func(a *Agent) {
		if sessionName == "" {
			a.SetSessionName(sessionName)
			return
		}
		a.SetSessionName("repl")
	}
}

//func WithSkillsDir(dirName string) AgentOptions {
//	return func(a *Agent) {
//	}
//}
