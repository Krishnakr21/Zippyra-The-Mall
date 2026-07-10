package mqtt

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

func NewMQTTClient(brokerURL, clientID, username, password string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(username).
		SetPassword(password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetOnConnectHandler(func(c mqtt.Client) {
			log.Info().Msg("MQTT connected")
		}).
		SetConnectionLostHandler(func(c mqtt.Client, err error) {
			log.Error().Err(err).Msg("MQTT connection lost")
		})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	return client, token.Error()
}
