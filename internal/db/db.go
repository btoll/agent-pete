package db

import (
	"database/sql"
	"errors"
	"slices"
)

// Get DB schema:
// sqlite> SELECT sql FROM sqlite_master WHERE type='table';
// Get table schema:
// sqlite> pragma db_info(TABLE_NAME);

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

func Commit(messages []Message) error {
	db, _ := GetDatabase()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	messagesStmt, err := tx.Prepare("INSERT INTO messages (conversation_id, role, content) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer messagesStmt.Close()

	toolsStmt, err := tx.Prepare("INSERT INTO tool_calls (message_id, tool_call_id, tool_name, parameters) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer toolsStmt.Close()

	for _, message := range messages {
		result, err := messagesStmt.Exec(message.ConversationID, message.Role, message.Content)
		if err != nil {
			return err
		}
		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if len(message.Tools) > 0 {
			for _, tool := range message.Tools {
				_, err := toolsStmt.Exec(lastID, tool.ID, tool.Name, tool.Parameters)
				if err != nil {
					return err
				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func CreateDatabase() error {
	db, _ := GetDatabase()
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY,
		conversation_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
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
	CREATE INDEX idx_tool_calls_status ON tool_calls(status);
	`
	_, err := db.Exec(createTableSQL)
	return err
}

func GetConversationID(name string) (int, error) {
	db, _ := GetDatabase()
	stmt, err := db.Prepare("SELECT id FROM conversations WHERE name = ?")
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	var id int
	row := stmt.QueryRow(name)
	err = row.Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			stmt, err := db.Prepare("INSERT INTO conversations (name) VALUES (?)")
			if err != nil {
				return -1, err
			}
			result, err := stmt.Exec(name)
			if err != nil {
				return -1, err
			}
			n, err := result.LastInsertId()
			if err != nil {
				return -1, err
			}
			return int(n), nil
		}
		return -1, err
	}
	return id, err
}

func GetDatabase() (*sql.DB, error) {
	return sql.Open("sqlite", "messages.db")
}

func GetNRecentMessages(conversationID int, limit int) ([]*Message, error) {
	db, _ := GetDatabase()
	stmt, err := db.Prepare("SELECT id, role, content FROM messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make([]*Message, 0, limit)
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(&msg.ID, &msg.Role, &msg.Content)
		if err != nil {
			return nil, err
		}
		m = append(m, msg)
	}
	slices.Reverse(m)
	return m, nil
}

func GetToolCallsById(messageID int) ([]ToolMessage, error) {
	db, _ := GetDatabase()
	stmt, err := db.Prepare("SELECT tool_call_id, tool_name, parameters FROM tool_calls WHERE message_id = ? ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := []ToolMessage{}
	for rows.Next() {
		msg := ToolMessage{}
		err := rows.Scan(&msg.ID, &msg.Name, &msg.Parameters)
		if err != nil {
			return nil, err
		}
		m = append(m, msg)
	}
	return m, nil
}

//func Inject() {
//	db, err := GetDatabase()
//	if err != nil {
//		panic(err)
//	}
//
//	err = CreateDatabase()
//	if err != nil {
//		panic(err)
//	}
//	defer db.Close()
//
//	b, err := os.ReadFile("/mnt/shared/context.json")
//	if err != nil {
//		panic(err)
//	}
//	var messages []Message
//	err = json.Unmarshal(b, &messages)
//	if err != nil {
//		panic(err)
//	}
//
//	err = Commit(messages)
//	if err != nil {
//		panic(err)
//	}
//}
