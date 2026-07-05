package main

import (
	"net"
	"net/http"
	"time"
)

// Compile-time assertion, ensure that *biplane implements gamepiece without constructing
// a value (no allocation).  Checks method-set compatibililty for *biplane.
// var _ Interface = (*T)(nil)
//var _ gamepiece = (*biplane)(nil)

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
)

type Request struct {
	Model   string         `json:"model"`
	Stream  bool           `json:"stream"`
	Options RequestOptions `json:"options"`
	//	Tools []any
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestOptions struct {
	//	Seed        int     `json:"seed"`
	Temperature float64 `json:"temperature"`
	//	TopK        int     `json:"top_k"`
	TopP float64 `json:"top_p"`
	//	MinP        float64 `json:"min_p"`
	//	Stop        string  `json:"stop"`
	//	NumCtx      int     `json:"num_ctx"`
	NumPredict int `json:"num_predict"`
}

type BaseModelResponse struct {
	Model              string      `json:"model"`
	CreatedAt          string      `json:"created_at"`
	Message            ChatMessage `json:"message"`
	Response           string      `json:"response"`
	Thinking           string      `json:"thinking"`
	Done               bool        `json:"done"`
	DoneReason         string      `json:"done_reason"`
	TotalDuration      int         `json:"total_duration"`
	LoadDuration       int         `json:"load_duration"`
	PromptEvalCount    int         `json:"prompt_eval_count"`
	PromptEvalDuration int         `json:"prompt_eval_duration"`
	EvalCount          int         `json:"eval_count"`
	EvalDuration       int         `json:"eval_duration"`
}
