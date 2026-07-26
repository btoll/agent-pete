package api

type RequestOptions func(*Request)

func WithModel(model string) RequestOptions {
	return func(req *Request) {
		if model == "" {
			req.Model = "mistral"
			return
		}
		req.Model = model
	}
}

func WithProfile(profile string) RequestOptions {
	return func(req *Request) {
		switch profile {
		case "accurate":
			req.Options = accurateProfile
		case "balanced":
			req.Options = balancedProfile
		default:
			req.Options = fastProfile
		}
	}
}

func WithStream(stream bool) RequestOptions {
	return func(req *Request) {
		req.Stream = stream
	}
}
