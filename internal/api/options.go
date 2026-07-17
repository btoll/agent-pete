package api

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

func WithTotalResponseTokens(totalResponseTokens int) ConfigOption {
	return func(req *Request) {
		req.Options.NumPredict = totalResponseTokens
	}
}
