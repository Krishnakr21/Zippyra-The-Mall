package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Outbox struct {
	ID          uuid.UUID       `json:"id"`
	Topic       string          `json:"topic"`
	Payload     json.RawMessage `json:"payload"`
	PublishedAt *time.Time      `json:"published_at"`
	CreatedAt   time.Time       `json:"created_at"`
}