package main

import (
	"flag"
	"log"

	_ "modernc.org/sqlite"
)

func isZeroValue[T comparable](v T) bool {
	var zero T
	return zero == v
}

func main() {
	var (
		conversationID      string
		currentMsg          string
		model               string
		oneOff              bool
		stream              bool
		totalResponseTokens int
	)

	flag.StringVar(&currentMsg, "m", "", "The newest message to append to the prompt.")
	flag.StringVar(&model, "model", "mistral", "The model.")
	flag.BoolVar(&oneOff, "one-off", false, "Don't include previous messages in the prompt (/generate).")
	flag.BoolVar(&stream, "stream", true, "True to use the streaming API (/chat).")
	flag.IntVar(&totalResponseTokens, "tokens", 0, "Total number of response tokens.")
	flag.StringVar(&conversationID, "conv", "default", "Conversation ID for grouping related messages.")
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

	db, err := getDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// If /chat...
	if isZeroValue(oneOff) {
		chatRequest := NewChatRequest(configOptions...)
		recentChatMessages, err := chatRequest.GetRecentMessages(conversationID, 30)
		if err != nil {
			panic(err)
		}
		chatRequest.Messages = append(recentChatMessages, ChatMessage{
			Role:    "user",
			Content: currentMsg,
		})
		resp, err := chatRequest.Post()
		if err != nil {
			panic(err)
		}
		err = commit(db, []DBMessage{
			{
				Timestamp:      "1",
				Role:           "user",
				Content:        currentMsg,
				ConversationID: conversationID,
			},
			{
				Timestamp:      resp.Timestamp,
				Role:           resp.Role,
				Content:        resp.Content,
				ConversationID: conversationID,
			},
		})
		if err != nil {
			panic(err)
		}
		return
	}

	generateRequest := NewGenerateRequest(configOptions...)
	generateRequest.Prompt = currentMsg
	err = generateRequest.Post()
	if err != nil {
		panic(err)
	}
}
