package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-chi/chi/v5/middleware"
)

func TestGateCommander_SendCommand_OPEN(t *testing.T) {
	var mu sync.Mutex
	var pubTopic string
	var pubPayload string

	mqttClient := &mockMQTTClient{
		publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
			mu.Lock()
			pubTopic = topic
			if b, ok := payload.([]byte); ok {
				pubPayload = string(b)
			}
			mu.Unlock()
			return &mockToken{}
		},
	}

	gateCommander := NewGateCommander(mqttClient, 1)

	storeID := "store-123"
	gateID := "gate-456"
	customerID := "cust-789"
	orderID := "order-999"

	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "test-req-id")
	err := gateCommander.SendCommand(ctx, storeID, gateID, "OPEN", customerID, orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for async publish to complete
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expectedTopic := fmt.Sprintf("zippyra/store/%s/gate/%s/command", storeID, gateID)
	if pubTopic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, pubTopic)
	}

	var cmd GateCommand
	err = json.Unmarshal([]byte(pubPayload), &cmd)
	if err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if cmd.Command != "OPEN" || cmd.CustomerID != customerID || cmd.OrderID != orderID || cmd.RequestID != "test-req-id" {
		t.Errorf("unexpected payload structure: %+v", cmd)
	}
}

func TestGateCommander_SendCommand_DENY(t *testing.T) {
	var mu sync.Mutex
	var pubTopic string
	var pubPayload string

	mqttClient := &mockMQTTClient{
		publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
			mu.Lock()
			pubTopic = topic
			if b, ok := payload.([]byte); ok {
				pubPayload = string(b)
			}
			mu.Unlock()
			return &mockToken{}
		},
	}

	gateCommander := NewGateCommander(mqttClient, 1)

	storeID := "store-123"
	gateID := "gate-456"
	customerID := "cust-789"
	orderID := "order-999"

	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "test-req-id-deny")
	err := gateCommander.SendCommand(ctx, storeID, gateID, "DENY", customerID, orderID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expectedTopic := fmt.Sprintf("zippyra/store/%s/gate/%s/command", storeID, gateID)
	if pubTopic != expectedTopic {
		t.Errorf("expected topic %s, got %s", expectedTopic, pubTopic)
	}

	var cmd GateCommand
	err = json.Unmarshal([]byte(pubPayload), &cmd)
	if err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if cmd.Command != "DENY" || cmd.CustomerID != customerID || cmd.OrderID != orderID || cmd.RequestID != "test-req-id-deny" {
		t.Errorf("unexpected payload structure: %+v", cmd)
	}
}

func TestGateCommander_MQTTUnavailable_NoError(t *testing.T) {
	// If MQTT is unavailable, we should log the error and not block or crash.
	mqttClient := &mockMQTTClient{
		publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
			return &mockToken{err: errors.New("connection timed out")}
		},
	}

	gateCommander := NewGateCommander(mqttClient, 1)

	ctx := context.Background()
	err := gateCommander.SendCommand(ctx, "store-1", "gate-1", "OPEN", "cust-1", "order-1")
	if err != nil {
		t.Fatalf("expected SendCommand to return nil on MQTT error (fire-and-forget), got: %v", err)
	}

	// Ensure the async routine runs and handles the token error without crashing
	time.Sleep(15 * time.Millisecond)
}
