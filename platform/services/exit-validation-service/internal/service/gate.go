package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type GateCommander struct {
	mqttClient mqtt.Client // paho.mqtt.golang
	qos        byte        // QoS 1 — at least once delivery
	isLocal    bool
}

type GateCommand struct {
	Command    string `json:"cmd"` // "OPEN" or "DENY"
	CustomerID string `json:"cust"`
	OrderID    string `json:"order"`
	Timestamp  int64  `json:"ts"`
	RequestID  string `json:"req_id"`
}

// NewGateCommander creates a new production GateCommander.
func NewGateCommander(mqttClient mqtt.Client, qos byte) *GateCommander {
	return &GateCommander{
		mqttClient: mqttClient,
		qos:        qos,
		isLocal:    false,
	}
}

// NewMockGateCommander for local dev (no MQTT broker)
func NewMockGateCommander() *GateCommander {
	return &GateCommander{
		isLocal: true,
	}
}

// SendCommand publishes OPEN or DENY to the gate's MQTT topic.
// Topic: zippyra/store/{store_id}/gate/{gate_id}/command
// If MQTT unavailable: log error + fallback (gate stays closed)
// Never block the HTTP response waiting for gate acknowledgment
func (g *GateCommander) SendCommand(ctx context.Context, storeID, gateID, command string, customerID, orderID string) error {
	topic := fmt.Sprintf("zippyra/store/%s/gate/%s/command", storeID, gateID)
	reqID := middleware.GetReqID(ctx)

	payloadStruct := GateCommand{
		Command:    command,
		CustomerID: customerID,
		OrderID:    orderID,
		Timestamp:  time.Now().Unix(),
		RequestID:  reqID,
	}

	if g.isLocal || g.mqttClient == nil {
		log.Info().
			Str("topic", topic).
			Str("command", command).
			Str("customer_id", customerID).
			Str("order_id", orderID).
			Str("req_id", reqID).
			Msg("Local dev / Mock Gate Command Logged")
		return nil
	}

	payload, err := json.Marshal(payloadStruct)
	if err != nil {
		return fmt.Errorf("marshal gate command payload: %w", err)
	}

	token := g.mqttClient.Publish(topic, g.qos, false, payload)
	// Non-blocking: don't wait for MQTT ack in HTTP response path
	go func() {
		token.Wait()
		if token.Error() != nil {
			log.Error().Err(token.Error()).
				Str("topic", topic).
				Str("command", command).
				Msg("MQTT gate command failed")
		}
	}()

	return nil
}
