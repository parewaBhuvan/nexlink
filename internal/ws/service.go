package ws

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// error message for invalid entry and non existing participant
var (
	ErrInvalidParticipantCount = errors.New("invalid number of participants for this conversation type")
	ErrParticipantNotFound     = errors.New("one or more participants do not exist")
)

type Conversation struct {
	ConversationID string  `json:"conversation_id"`
	Type           string  `json:"type"`
	Name           *string `json:"name"`
	Existing       bool    `json:"existing"`
}

// find existing current direct conversation
func (s *Service) FindDirectConversation(ctx context.Context, userA, userB string) (string, error) {
	var conversationID string
	err := s.db.QueryRow(ctx,
		`SELECT c.conversation_id
		 FROM conversations c
		 JOIN participants p1 ON p1.conversation_id = c.conversation_id AND p1.user_id = $1
		 JOIN participants p2 ON p2.conversation_id = c.conversation_id AND p2.user_id = $2
		 WHERE c.type = 'direct'`,
		userA, userB,
	).Scan(&conversationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return "", ErrParticipantNotFound
		}
		return "", err
	}
	return conversationID, nil
}

// Creating conversations
func (s *Service) CreateConversation(ctx context.Context, convType string, name *string, participantIDs []string) (*Conversation, error) {
	if convType == "direct" && len(participantIDs) != 2 {
		return nil, ErrInvalidParticipantCount
	}
	if convType == "group" && len(participantIDs) < 2 {
		return nil, ErrInvalidParticipantCount
	}

	if convType == "direct" {
		existingID, err := s.FindDirectConversation(ctx, participantIDs[0], participantIDs[1])
		if err != nil {
			return nil, err
		}
		if existingID != "" {
			return &Conversation{
				ConversationID: existingID,
				Type:           "direct",
				Existing:       true,
			}, nil
		}
	}

	txn, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer txn.Rollback(ctx)

	var conversationID string
	err = txn.QueryRow(ctx,
		`INSERT INTO conversations (type, name) VALUES ($1, $2) RETURNING conversation_id`,
		convType, name,
	).Scan(&conversationID)
	if err != nil {
		return nil, err
	}

	for _, uid := range participantIDs {
		_, err := txn.Exec(ctx,
			`INSERT INTO participants (conversation_id, user_id) VALUES ($1, $2)`,
			conversationID, uid,
		)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "22P02") {
				return nil, ErrParticipantNotFound
			}
			return nil, err
		}
	}

	if err := txn.Commit(ctx); err != nil {
		return nil, err
	}

	return &Conversation{
		ConversationID: conversationID,
		Type:           convType,
		Name:           name,
		Existing:       false,
	}, nil
}
