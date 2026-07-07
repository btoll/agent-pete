package api

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
