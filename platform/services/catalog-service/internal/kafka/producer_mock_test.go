package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"
)

type mockKafkaWriter struct {
	writeMessagesFunc func(ctx context.Context, msgs ...kafka.Message) error
	closeFunc          func() error
}

func (m *mockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return m.writeMessagesFunc(ctx, msgs...)
}

func (m *mockKafkaWriter) Close() error {
	return m.closeFunc()
}
