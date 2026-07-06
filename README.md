# agent-pete

This is a learning project.  I am using the [Ollama project] to build an AI Agent.

Currenty, `agent-pete` supports the [`generate`](https://docs.ollama.com/api/generate) and [`chat`](https://docs.ollama.com/api/chat) REST APIs.  `agent-pete` supports both streaming (the default) and non-streaming.

There is a limited number CLI options that are supported.  The default is to hit the `chat` endpoint unless `--one-off` is passed as a flag, at which point `agent-pete` will call the `generate` endpoint.

```bash
Usage of ./agent-pete:
  -conv string
        Conversation ID for grouping related messages. (default "default")
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
```

This will change!  The code will change!  You will change!  Change is inevitable!

## The Difference Between `generate` and `chat`

The `generate` API does not include any previous messages (context) in its prompt when `POST`ed to the Ollama server (model?).  It is a one-off, with the question and the response NOT being persisted.

The `chat` API, on the other hand, will include previous messages in its prompt.  It default to 30, which is low, but the context window for the `mistral` model is quite small.  In addition, it will persist both the question ("role": "user") and the response ("role": "assistant") to a local [SQLite](https://sqlite.org/index.html) database.

## Database Schema

```sql
sqlite> pragma table_info(messages);
0|id|INTEGER|0||1
1|timestamp|TEXT|1||0
2|role|TEXT|1||0
3|content|TEXT|1||0
```

DROP INDEX idx_conversation_id;
ALTER TABLE messages DROP COLUMN conversation_id;
ALTER TABLE messages ADD COLUMN conversation_id TEXT NOT NULL;
CREATE INDEX idx_conversation_id ON messages(conversation_id);

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

## Reference

- [Ollama project]
- [Ollama API Docs](https://docs.ollama.com/api/introduction)
- [Go `database/sql` package docs](https://pkg.go.dev/database/sql)

## License

[GPLv3](COPYING)

## Author

[Benjamin Toll](https://benjamintoll.com)

[Ollama project]: https://ollama.com/
