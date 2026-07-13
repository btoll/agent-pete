# agent-pete

This is a learning project.  I am using the [Ollama project] to build an AI Agent.

Currenty, `agent-pete` supports the [`generate`](https://docs.ollama.com/api/generate) and [`chat`](https://docs.ollama.com/api/chat) REST APIs.  `agent-pete` supports both streaming (the default) and non-streaming.  Retries and exponential backoff is supported.

Tools are fully supported.

There is a limited number CLI options that are supported.  The default is to hit the `chat` endpoint unless `--one-off` is passed as a flag, at which point `agent-pete` will call the `generate` endpoint.

```bash
$ ./agent-pete -h
Usage of ./agent-pete:
  -conv string
        Conversation ID for grouping related messages. (default "default")
  -create-database
        Create the database.  Useful for debugging.
  -m string
        The newest message to append to the prompt.
  -model string
        The model. (default "mistral")
  -one-off
        Don't include previous messages in the prompt (/generate).
  -stream
        True to use the streaming API (/chat). (default true)
  -tokens int
        Total number of response tokens.
  -tool value
        The name of a tool (function).  Can accept specified multiple times.  Primarily used for debugging, but it can help limit tokens spent by reducing the request payload.
```

This will change!  The code will change!  You will change!  Change is inevitable!

## The Difference Between `generate` and `chat`

The `generate` API does not include any previous messages (context) in its prompt when `POST`ed to the Ollama server (model?).  It is a one-off, with the question and the response NOT being persisted.

The `chat` API, on the other hand, will include previous messages in its prompt.  It default to 30, which is low, but the context window for the `mistral` model is quite small.  In addition, it will persist both the question ("role": "user") and the response ("role": "assistant") to a local [SQLite](https://sqlite.org/index.html) database.

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

## Reference

- [Ollama project]
- [Ollama API Docs](https://docs.ollama.com/api/introduction)
- [Go `database/sql` package docs](https://pkg.go.dev/database/sql)

## License

[GPLv3](COPYING)

## Author

[Benjamin Toll](https://benjamintoll.com)

[Ollama project]: https://ollama.com/
