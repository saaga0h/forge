package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

type Client struct {
	client mqtt.Client
	config *Config
}

type Config struct {
	Broker   string
	Username string
	Password string
	ClientID string
	QoS      byte
}

// MessageHandler is called when a message is received
type MessageHandler func(topic string, payload []byte) error

func NewClient(config *Config) (*Client, error) {
	if config.ClientID == "" {
		config.ClientID = fmt.Sprintf("go-orch-%s", uuid.New().String())
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetClientID(config.ClientID)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetCleanSession(false)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(1 * time.Minute)
	opts.SetKeepAlive(30 * time.Second)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("MQTT connected to %s", config.Broker)
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	opts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Println("MQTT reconnecting...")
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()

	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("mqtt connect timeout")
	}

	if token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect failed: %w", token.Error())
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

func (c *Client) Publish(topic string, payload interface{}, retained bool) error {
	var data []byte
	var err error

	switch v := payload.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload failed: %w", err)
		}
	}

	token := c.client.Publish(topic, c.config.QoS, retained, data)

	if !token.WaitTimeout(30 * time.Second) {
		return fmt.Errorf("publish timeout on topic: %s", topic)
	}

	if token.Error() != nil {
		return fmt.Errorf("publish failed: %w", token.Error())
	}

	log.Printf("Published to %s (retained: %v, size: %d bytes)", topic, retained, len(data))
	return nil
}

func (c *Client) Subscribe(topic string, handler MessageHandler) error {
	token := c.client.Subscribe(topic, c.config.QoS, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("Received message on %s (size: %d bytes)", msg.Topic(), len(msg.Payload()))

		if err := handler(msg.Topic(), msg.Payload()); err != nil {
			log.Printf("Handler error for topic %s: %v", msg.Topic(), err)
		}
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe timeout for topic: %s", topic)
	}

	if token.Error() != nil {
		return fmt.Errorf("subscribe failed: %w", token.Error())
	}

	log.Printf("Subscribed to: %s", topic)
	return nil
}

func (c *Client) Unsubscribe(topics ...string) error {
	token := c.client.Unsubscribe(topics...)

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("unsubscribe timeout")
	}

	if token.Error() != nil {
		return fmt.Errorf("unsubscribe failed: %w", token.Error())
	}

	log.Printf("Unsubscribed from: %v", topics)
	return nil
}

func (c *Client) Disconnect() {
	c.client.Disconnect(250)
	log.Println("MQTT disconnected")
}

func (c *Client) IsConnected() bool {
	return c.client.IsConnected()
}
