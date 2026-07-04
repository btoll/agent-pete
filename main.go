package main

import (
	"flag"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var (
	currentMsg          string
	model               string
	stream              bool
	totalResponseTokens int
)

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	//https://github.com/ollama/ollama/blob/main/docs/modelfile.mdx#valid-parameters-and-values
	Options ChatRequestOptions `json:"options"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequestOptions struct {
	NumPredict  int     `json:"num_predict"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
}

type ConfigOption func(*ChatRequest)

func getChatRequest(opts ...ConfigOption) *ChatRequest {
	chatRequest := &ChatRequest{
		Model:  "mistral",
		Stream: true,
		Options: ChatRequestOptions{
			NumPredict:  300, // Limit to ~300 tokens max.
			Temperature: 0.3, // Lower temp = more deterministic, shorter responses.
			TopP:        0.5, // Reduce diversity.
		},
	}
	for _, opt := range opts {
		opt(chatRequest)
	}
	return chatRequest
}

func isZeroValue[T comparable](v T) bool {
	var zero T
	return zero == v
}

func withModel(model string) ConfigOption {
	return func(cr *ChatRequest) {
		cr.Model = model
	}
}

func withStream(stream bool) ConfigOption {
	return func(cr *ChatRequest) {
		cr.Stream = stream
	}
}

func withTotalResponseTokens(totalResponseTokens int) ConfigOption {
	return func(cr *ChatRequest) {
		fmt.Printf("totalResponseTokens=%#v\n", totalResponseTokens)
		cr.Options.NumPredict = totalResponseTokens
	}
}

func main() {
	flag.StringVar(&currentMsg, "m", "", "The newest message to append to the prompt.")
	flag.StringVar(&model, "model", "mistral", "The model.")
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API.")
	flag.IntVar(&totalResponseTokens, "tokens", 0, "Total number of response tokens.")
	flag.Parse()

	if isZeroValue(currentMsg) {
		log.Fatalln("You must ask a question.")
	}

	var configOptions []ConfigOption
	if !isZeroValue(model) {
		configOptions = append(configOptions, withModel(model))
	}
	if isZeroValue(stream) {
		configOptions = append(configOptions, withStream(stream))
	}
	if !isZeroValue(totalResponseTokens) {
		configOptions = append(configOptions, withTotalResponseTokens(totalResponseTokens))
	}
	chatRequest := getChatRequest(configOptions...)

	db, err := getDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()
	recentMessages, err := getRecentMessages(db, 30)
	if err != nil {
		panic(err)
	}
	recentMessages = append(recentMessages, Message{
		Role:    "user",
		Content: currentMsg,
	})
	chatRequest.Messages = recentMessages
	resp, err := postChat(chatRequest)
	if err != nil {
		panic(err)
	}
	err = commit(db, []DBMessage{
		{
			Timestamp: "1",
			Role:      "user",
			Content:   currentMsg,
		},
		{
			Timestamp: resp.Timestamp,
			Role:      resp.Role,
			Content:   resp.Content,
		},
	})
	if err != nil {
		panic(err)
	}
}
