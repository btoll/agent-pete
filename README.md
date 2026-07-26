# agent-pete

This is a work in progress.  I am using the [Ollama project] to build an AI Agent.

Currenty, `agent-pete` supports the [`chat`](https://docs.ollama.com/api/chat) REST API.  `agent-pete` supports both streaming (the default) and non-streaming.  Retries and exponential backoff is supported.

`agent-pete` currently supports:

- tools
- skills
- profiles that help control control text generation
- streaming and non-streaming
- retries and exponential backoff
- structured error logs for debugging
- good feelings

> Note: Free agents that can run locally are notoriously bad at multi-step inference.  They aren't large enough to be truly useful, and this is due to technical limitations.
>
> Unfortunately, this means that most people will then get a subscription to an AI provider.  This is all part of the fucking scam of AI.  So, don't be a loser and get a subscription.  Instead, use [`duck.ai`](https://duck.ai/) or an open source AI coding agent like [`OpenCode`](https://opencode.ai/) and their free models.

There is a limited number CLI options that are supported:

```bash
$ agent-pete -h
Usage of agent-pete:
  -create-database
        Create the database.  Useful for debugging.
  -debug
        Turn on verbose logging.
  -m string
        The newest message to append to the prompt.
  -model string
        The model.
  -name string
        The name of the session, used for grouping related messages.
  -profile string
        The profile tunes the runtime options that control text generation (fast|accurate|balanced).
  -stream
        True to use the streaming API (/chat). (default true)
```

This will change!  The code will change!  You will change!  Change is inevitable!

## Profiles

```go
type Profile struct {
	NumCtx      int      `json:"num_ctx"`
	NumPredict  int      `json:"num_predict"`
	TopK        int      `json:"top_k"`
	Temperature float64  `json:"temperature"`
	TopP        float64  `json:"top_p"`
	MinP        float64  `json:"min_p"`
	Stop        []string `json:"stop"`
}

var (
    fastProfile = Profile{
        Temperature: 0.3,
        TopK:        20,
        TopP:        0.9,
        MinP:        0.1,
        NumPredict:  256,
        NumCtx:      2048,
        Stop:        []string{"\n\n"},
    }

    accurateProfile = Profile{
        Temperature: 0.1,
        TopK:        40,
        TopP:        0.95,
        MinP:        0.05,
        NumPredict:  2048,
        NumCtx:      8192,
    }

    balancedProfile = Profile{
        Temperature: 0.5,
        TopK:        30,
        TopP:        0.9,
        MinP:        0.05,
        NumPredict:  512,
        NumCtx:      4096,
        Stop:        []string{"\n\n"},
    }
)
```

## On `chat`

The `chat` API will include previous messages in its prompt.  It default to 30, which is low, but the context window for the `mistral` model is quite small.  In addition, it will persist both the question ("role": "user") and the response ("role": "assistant") to a local [SQLite](https://sqlite.org/index.html) database.

```bash
$ ollama list
NAME                   ID              SIZE      MODIFIED
mistral-nemo:latest    e7e06d107c6c    7.1 GB    18 minutes ago
llama3.1:latest        46e0c10c039e    4.9 GB    36 minutes ago
neural-chat:latest     89fa737d3b85    4.1 GB    14 hours ago
mistral:latest         6577803aa9a0    4.4 GB    2 days ago
```

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY,
    conversation_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    content TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id)
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'btoll',
    name TEXT UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tool_calls (
    id INTEGER PRIMARY KEY,
    message_id INTEGER NOT NULL,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    parameters TEXT NOT NULL,  -- JSON string, keep for auditing and debugging
    result TEXT,               -- JSON string, null if not yet executed
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(message_id) REFERENCES messages(id)
);

CREATE INDEX idx_tool_calls_message ON tool_calls(message_id);
```

## Model Responses

When streaming, the model will respond with chunks that need to be aggregated by the client.  This is an example of a chunk that contains a string (probably a string encoded token).  There will be many of them, and the server will keep responded until it sends `"done": true`.

```json
{
  "model": "mistral",
  "created_at": "2026-07-04T03:57:55.833644117Z",
  "message": {
    "role": "assistant",
    "content": "al"
  },
  "done": false
}
```

This is the last chunk sent by the server (because `"done": true`).  Note that it contains metadata that previous chunks do not.

```json
{
  "model": "mistral",
  "created_at": "2026-07-04T03:57:55.833976869Z",
  "message": {
    "role": "assistant",
    "content": ""
  },
  "done": true,
  "done_reason": "length",
  "total_duration": 193152790461,
  "load_duration": 11704765626,
  "prompt_eval_count": 629,
  "prompt_eval_duration": 88708208000,
  "eval_count": 300,
  "eval_duration": 92728962000
}
```

//&api.PostResponse{
//	Role:    "assistant",
//	Content: "",
//	Message: api.Message{
//		Role:    "assistant",
//		Content: "",
//		ToolCalls: []api.ToolCall{
//			{
//				ID: "call_tjtt6c2r",
//				Function: api.Function2{
//					Index:       0,
//					Name:        "ReadFile",
//					Description: "",
//					Arguments: map[string]interface{}{
//						"filename": "testy.txt",
//					},
//				},
//			},
//		},
//	},
//}

//api.ToolCall{
//    ID: "call_yjo7vyfb",
//    Function: api.Function2{
//        Index:       0,
//        Name:        "Add",
//        Description: "",
//        Arguments: map[string]interface{}{
//            "a": 2,
//            "b": 2,
//        },
//    },
//}

//api.ToolCall{
//	ID: "call_4wvxtmue",
//	Function: api.Function2{
//		Index:       0,
//		Name:        "ReadFile",
//		Description: "",
//		Arguments: map[string]interface{}{
//			"filename": "testy.txt",
//		},
//	},
//}

<!--
https://github.com/ArneJanning/local-skills-agent
-->

## Reference

- [Ollama project]
- [Ollama API Docs](https://docs.ollama.com/api/introduction)
- [Agent Skills](https://agentskills.io/)
- [Go `database/sql` package docs](https://pkg.go.dev/database/sql)

## License

[GPLv3](COPYING)

## Author

[Benjamin Toll](https://benjamintoll.com)

[Ollama project]: https://ollama.com/

