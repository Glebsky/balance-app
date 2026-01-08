package consumer

import (
	"balance-consumer/internal/config"
	"balance-consumer/internal/database"
	"balance-consumer/internal/logger"
	"balance-consumer/internal/sync"
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type Consumer struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	config    config.RabbitMQConfig
	db        *database.DB
	cacheSync *sync.CacheSynchronizer
	log       *logrus.Logger
}

type BalanceUpdateMessage struct {
	UserID    int     `json:"user_id"`
	OldAmount float64 `json:"old_amount"`
	NewAmount float64 `json:"new_amount"`
	Version   int     `json:"version"`
	Timestamp string  `json:"timestamp"`
	EventID   string  `json:"event_id"`
}

func New(cfg config.RabbitMQConfig, db *database.DB, cacheSync *sync.CacheSynchronizer, log *logrus.Logger) (*Consumer, error) {
	consumer := &Consumer{
		config:    cfg,
		db:        db,
		cacheSync: cacheSync,
		log:       log,
	}

	if err := consumer.connect(); err != nil {
		return nil, err
	}

	return consumer, nil
}

func (c *Consumer) connect() error {
	dsn := fmt.Sprintf(
		"amqp://%s:%s@%s:%d%s",
		c.config.User, c.config.Password, c.config.Host, c.config.Port, c.config.VHost,
	)

	var err error
	c.conn, err = amqp.Dial(dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = c.channel.ExchangeDeclare(
		c.config.Exchange,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue with dead-letter queue
	_, err = c.channel.QueueDeclare(
		c.config.Queue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    c.config.Exchange + "_dlx",
			"x-dead-letter-routing-key":  c.config.Queue + "_dlq",
			"x-message-ttl":              60000, // 60 seconds
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = c.channel.QueueBind(
		c.config.Queue,
		c.config.Queue,
		c.config.Exchange,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	c.log.Info("RabbitMQ connection established")
	return nil
}

func (c *Consumer) Start() error {
	// Set QoS to process one message at a time
	err := c.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := c.channel.Consume(
		c.config.Queue,
		"",    // consumer
		false, // auto-ack (manual ack for reliability)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.log.Info("Started consuming messages from RabbitMQ")

	for msg := range msgs {
		c.handleMessage(msg)
	}

	return nil
}

func (c *Consumer) handleMessage(msg amqp.Delivery) {
	var balanceMsg BalanceUpdateMessage
	if err := json.Unmarshal(msg.Body, &balanceMsg); err != nil {
		c.log.WithError(err).Error("Failed to unmarshal message")
		msg.Nack(false, false) // Reject and don't requeue
		return
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, balanceMsg.Timestamp)
	if err != nil {
		c.log.WithError(err).Error("Failed to parse timestamp")
		msg.Nack(false, false)
		return
	}

	// Check idempotency - ignore old messages
	if time.Since(timestamp) > 5*time.Minute {
		c.log.WithFields(logrus.Fields{
			"event_id": balanceMsg.EventID,
			"age":      time.Since(timestamp),
		}).Warn("Ignoring old message")
		msg.Ack(false)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Save to database
	if err := c.db.SaveBalanceUpdate(
		ctx,
		balanceMsg.UserID,
		balanceMsg.OldAmount,
		balanceMsg.NewAmount,
		balanceMsg.Version,
		balanceMsg.EventID,
		timestamp,
	); err != nil {
		c.log.WithError(err).WithFields(logrus.Fields{
			"user_id": balanceMsg.UserID,
			"event_id": balanceMsg.EventID,
		}).Error("Failed to save balance update")
		msg.Nack(false, true) // Reject and requeue
		return
	}

	// Update cache
	c.cacheSync.UpdateCache(balanceMsg.UserID, balanceMsg.NewAmount, balanceMsg.Version, timestamp)

	c.log.WithFields(logrus.Fields{
		"user_id": balanceMsg.UserID,
		"version": balanceMsg.Version,
		"event_id": balanceMsg.EventID,
	}).Debug("Balance update processed")

	msg.Ack(false)
}

func (c *Consumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

