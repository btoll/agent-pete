# agent-pete

## Database Schema

```sql
sqlite> pragma table_info(messages);
0|id|INTEGER|0||1
1|timestamp|TEXT|1||0
2|role|TEXT|1||0
3|content|TEXT|1||0
```

## Response

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

- [Ollama API Docs](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [Go `database/sql` package docs](https://pkg.go.dev/database/sql)

## License

[GPLv3](COPYING)

## Author

[Benjamin Toll](https://benjamintoll.com)

