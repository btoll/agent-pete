package api

import (
	"github.com/btoll/agent-pete/internal/tool"
)

type ConfigOption func(*Request)

func WithModel(model string) ConfigOption {
	return func(req *Request) {
		req.Model = model
	}
}

func WithStream(stream bool) ConfigOption {
	return func(req *Request) {
		req.Stream = stream
	}
}

func WithTools(toolsNames []string) ConfigOption {
	return func(req *Request) {
		for _, toolName := range toolsNames {
			if v, found := tool.Tools[toolName]; found {
				req.Tools = append(req.Tools, v)
			}
		}
	}
}

func WithTotalResponseTokens(totalResponseTokens int) ConfigOption {
	return func(req *Request) {
		req.Options.NumPredict = totalResponseTokens
	}
}
