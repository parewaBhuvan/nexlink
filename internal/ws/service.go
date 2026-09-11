package ws

import (
	"context"

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