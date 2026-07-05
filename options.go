package main

type ConfigOption func(*Request)

func withModel(model string) ConfigOption {
	return func(req *Request) {
		req.Model = model
	}
}

func withStream(stream bool) ConfigOption {
	return func(req *Request) {
		req.Stream = stream
	}
}

func withTotalResponseTokens(totalResponseTokens int) ConfigOption {
	return func(req *Request) {
		req.Options.NumPredict = totalResponseTokens
	}
}
