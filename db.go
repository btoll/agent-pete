package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"slices"
)

//type ChatHistory struct {
//	ConversationMetadata struct {
//		Platform  string `json:"platform"`
//		DateRange string `json:"date_range"`
//		Note      string `json:"note"`
//	} `json:"conversation_metadata"`
//	Messages []DBMessage `json:"messages"`
//}

type DBMessage struct {
	Turn           int    `json:"turn"`
	Timestamp      string `json:"timestamp"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	ConversationID string `json:"conversation_id"`
}

func commit(db *sql.DB, messages []DBMessage) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO messages (timestamp, role, content, conversation_id) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, message := range messages {
		_, err := stmt.Exec(message.Timestamp, message.Role, message.Content, message.ConversationID)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func createTable(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		conversation_id TEXT NOT NULL
	);
	`
	_, err := db.Exec(createTableSQL)
	return err
}

func getDatabase() (*sql.DB, error) {
	return sql.Open("sqlite", "conversation.db")
}

func getNRecentMessages(conversationID string, limit int) ([]ChatMessage, error) {
	db, _ := getDatabase()
	stmt, err := db.Prepare("SELECT role,content FROM messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make([]ChatMessage, 0, limit)
	for rows.Next() {
		msg := ChatMessage{}
		err := rows.Scan(&msg.Role, &msg.Content)
		if err != nil {
			return nil, err
		}
		m = append(m, msg)
	}
	slices.Reverse(m)
	return m, nil
}

func inject() {
	db, err := sql.Open("sqlite", "conversation.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = createTable(db)
	if err != nil {
		panic(err)
	}

	b, err := os.ReadFile("/mnt/shared/context.json")
	if err != nil {
		panic(err)
	}
	var messages []DBMessage
	err = json.Unmarshal(b, &messages)
	if err != nil {
		panic(err)
	}

	err = commit(db, messages)
	if err != nil {
		panic(err)
	}
}
