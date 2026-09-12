package ws

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) GetParticipants(ctx context.Context, conversationID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT user_id FROM participants WHERE conversation_id = $1`,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participantIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		participantIDs = append(participantIDs, userID)
	}

	return participantIDs, rows.Err()
}

func (s *Service) SaveMessage(ctx context.Context, conversationID, senderID, content string) (string, error) {
	var messageID string
	err := s.db.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, content) VALUES ($1, $2, $3) RETURNING message_id`,
		conversationID, senderID, content,
	).Scan(&messageID)
	if err != nil {
		return "", err
	}
	return messageID, nil
}

var ErrNotParticipant = errors.New("user is not a participant in this conversation")

func (s *Service) IsParticipant(ctx context.Context, conversationID, userID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(ctx,
		`SELECT 1 FROM participants WHERE conversation_id = $1 AND user_id = $2`,
		conversationID, userID,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type Message struct {
	MessageID     string    `json:"message_id"`
	SenderID      string    `json:"sender_id"`
	ContentType   string    `json:"content_type"`
	Content       string    `json:"content"`
	AttachmentURL *string   `json:"attachment_url"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Service) GetMessages(ctx context.Context, conversationID string, before *time.Time, limit int) ([]Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT message_id, sender_id, content_type, content, attachment_url, created_at
		 FROM messages
		 WHERE conversation_id = $1
		   AND deleted_at IS NULL
		   AND ($2::timestamptz IS NULL OR created_at < $2)
		 ORDER BY created_at DESC
		 LIMIT $3`,
		conversationID, before, limit,
		//for next version if no of user increase, update query as (WHERE (created_at, message_id) < ($cursor_time, $cursor_id) ORDER BY created_at DESC, message_id DESC)
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.MessageID, &m.SenderID, &m.ContentType, &m.Content, &m.AttachmentURL, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}
