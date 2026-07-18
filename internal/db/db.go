package db

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type DB struct {
	conn *sql.DB
}

type ToolMessage struct {
	ID         string
	Name       string
	Parameters string
	Result     string
}

type Message struct {
	ID             int
	ConversationID int
	Role           string
	Content        string
	Tools          []ToolMessage
}

func New() *DB {
	return &DB{
		conn: OpenDatabase(),
	}
}

//func CloseDatabase() error {
//	db = GetDatabase()
//	if db != nil {
//		return db.Close()
//	}
//	return nil
//}

func OpenDatabase() *sql.DB {
	var err error
	db, err = sql.Open("sqlite", "messages.db")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	return db
}

func (d *DB) CommitMessage(conversationID int, role, content string) (int, error) {
	stmt, err := d.conn.Prepare("INSERT INTO messages (conversation_id, role, content, status) VALUES (?, ?, ?, ?)")
	if err != nil {
		return -1, &StoreError{
			Op:        "CommitMessage",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	status := "pending"
	if role == "assistant" {
		status = "success"
	}
	result, err := stmt.Exec(conversationID, role, content, status)
	if err != nil {
		return -1, &StoreError{
			Op:        "CommitMessage",
			Retryable: false,
			Err:       fmt.Errorf("DB: stmt.Exec: %w", err),
		}
	}
	i64, err := result.LastInsertId()
	if err != nil {
		return -1, &StoreError{
			Op:        "CommitMessage",
			Retryable: false,
			Err:       fmt.Errorf("DB: LastInsertId: %w", err),
		}
	}
	return int(i64), nil
}

func (d *DB) CommitToolCall(lastID int, toolCallID, funcName, arguments, res string) error {
	stmt, err := d.conn.Prepare("INSERT INTO tool_calls (message_id, tool_call_id, tool_name, parameters, result) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return &StoreError{
			Op:        "CommitToolCall",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	_, err = stmt.Exec(lastID, toolCallID, funcName, arguments, res)
	if err != nil {
		return &StoreError{
			Op:        "CommitToolCall",
			Retryable: false,
			Err:       fmt.Errorf("DB: stmt.Exec: %w", err),
		}
	}
	return nil
}

func CreateDatabase() error {
	db, _ := sql.Open("sqlite", "messages.db")
	createTableSQL := `
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
	`
	_, err := db.Exec(createTableSQL)
	return err
}

func (d *DB) GetConversationID(name string) (int, error) {
	stmt, err := d.conn.Prepare("SELECT id FROM conversations WHERE name = ?")
	if err != nil {
		return -1, &StoreError{
			Op:        "GetConversationID",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	var id int
	row := stmt.QueryRow(name)
	err = row.Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			stmt, err := db.Prepare("INSERT INTO conversations (name) VALUES (?)")
			if err != nil {
				return -1, &StoreError{
					Op:        "GetConversationID",
					Retryable: false,
					Err:       fmt.Errorf("DB: Prepare: %w", err),
				}
			}
			result, err := stmt.Exec(name)
			if err != nil {
				return -1, &StoreError{
					Op:        "GetConversationID",
					Retryable: false,
					Err:       fmt.Errorf("DB: stmt.Exec: %w", err),
				}
			}
			n, err := result.LastInsertId()
			if err != nil {
				return -1, &StoreError{
					Op:        "GetConversationID",
					Retryable: false,
					Err:       fmt.Errorf("DB: LastInsertId: %w", err),
				}
			}
			return int(n), nil
		}
		return -1, &StoreError{
			Op:        "GetConversationID",
			Retryable: false,
			Err:       err,
		}
	}
	return id, nil
}

func (d *DB) GetNRecentMessages(conversationID int, limit int) ([]*Message, error) {
	stmt, err := d.conn.Prepare("SELECT id, role, content FROM messages WHERE conversation_id = ? AND status = 'success' ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, &StoreError{
			Op:        "GetNRecentMessages",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	rows, err := stmt.Query(conversationID, limit)
	if err != nil {
		return nil, &StoreError{
			Op:        "GetNRecentMessages",
			Retryable: false,
			Err:       fmt.Errorf("DB: stmt.Query: %w", err),
		}
	}
	defer rows.Close()
	m := make([]*Message, 0, limit)
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(&msg.ID, &msg.Role, &msg.Content)
		if err != nil {
			return nil, &StoreError{
				Op:        "GetNRecentMessages",
				Retryable: false,
				Err:       fmt.Errorf("DB: rows.Scan: %w", err),
			}
		}
		m = append(m, msg)
	}
	slices.Reverse(m)
	return m, nil
}

func (d *DB) GetToolCallsById(messageID int) ([]ToolMessage, error) {
	stmt, err := d.conn.Prepare("SELECT tool_call_id, tool_name, parameters, result FROM tool_calls WHERE message_id = ? ORDER BY id")
	if err != nil {
		return nil, &StoreError{
			Op:        "GetToolCallsById",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	rows, err := stmt.Query(messageID)
	if err != nil {
		return nil, &StoreError{
			Op:        "GetToolCallsById",
			Retryable: false,
			Err:       fmt.Errorf("DB: stmt.Query: %w", err),
		}
	}
	defer rows.Close()
	m := []ToolMessage{}
	for rows.Next() {
		msg := ToolMessage{}
		err := rows.Scan(&msg.ID, &msg.Name, &msg.Parameters, &msg.Result)
		if err != nil {
			return nil, &StoreError{
				Op:        "GetToolCallsById",
				Retryable: false,
				Err:       fmt.Errorf("DB: rows.Scan: %w", err),
			}
		}
		m = append(m, msg)
	}
	return m, nil
}

func (d *DB) UpdateMessageStatus(rowID int, status string) error {
	stmt, err := d.conn.Prepare("UPDATE messages SET status = ? WHERE id = ?")
	if err != nil {
		return &StoreError{
			Op:        "UpdateMessageStatus",
			Retryable: false,
			Err:       fmt.Errorf("DB: Prepare: %w", err),
		}
	}
	defer stmt.Close()
	_, err = stmt.Exec(status, rowID)
	if err != nil {
		return &StoreError{
			Op:        "UpdateMessageStatus",
			Retryable: false,
			Err:       fmt.Errorf("DB: stmt.Exec: %w", err),
		}
	}
	return nil
}
