package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"balance-service/internal/config"
	"balance-service/internal/processor"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	reconnectDelay = 5 * time.Second
	maxReconnectAttempts = 10
	consumerTimeout = 30 * time.Second
)

type Consumer struct {
	cfg     config.RabbitConfig
	log     *logrus.Logger
	updates chan<- processor.IncomingUpdate

	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg config.RabbitConfig, log *logrus.Logger, updates chan<- processor.IncomingUpdate) (*Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		cfg:     cfg,
		log:     log,
		updates: updates,
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := c.connect(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return c, nil
}

func (c *Consumer) connect() error {
	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		c.cfg.User, c.cfg.Password, c.cfg.Host, c.cfg.Port, c.cfg.VHost)

	conn, err := amqp.Dial(dsn)
	if err != nil {
		return fmt.Errorf("failed to dial RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if _, err := ch.QueueDeclare(
		c.cfg.Queue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.channel = ch
	c.mu.Unlock()

	c.log.WithFields(logrus.Fields{
		"host":  c.cfg.Host,
		"queue": c.cfg.Queue,
	}).Info("connected to RabbitMQ")

	// Monitor connection for errors
	go c.monitorConnection()

	return nil
}

func (c *Consumer) monitorConnection() {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	notifyClose := conn.NotifyClose(make(chan *amqp.Error))

	select {
	case err := <-notifyClose:
		if err != nil {
			c.log.WithError(err).Error("RabbitMQ connection closed unexpectedly")
			c.reconnect()
		}
	case <-c.ctx.Done():
		return
	}
}

func (c *Consumer) reconnect() {
	c.mu.Lock()
	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		c.log.WithField("attempt", attempt).Info("attempting to reconnect to RabbitMQ")

		if err := c.connect(); err == nil {
			c.log.Info("successfully reconnected to RabbitMQ")
			// Restart consuming in a new goroutine
			go func() {
				if err := c.Start(c.ctx); err != nil && c.ctx.Err() == nil {
					c.log.WithError(err).Error("failed to restart consumer after reconnect")
				}
			}()
			return
		}

		delay := reconnectDelay * time.Duration(attempt)
		c.log.WithFields(logrus.Fields{
			"attempt": attempt,
			"delay":   delay,
		}).Warn("reconnection failed, retrying")

		select {
		case <-time.After(delay):
		case <-c.ctx.Done():
			return
		}
	}

	c.log.Error("max reconnection attempts reached, giving up")
}

func (c *Consumer) Start(ctx context.Context) error {
	c.mu.RLock()
	channel := c.channel
	c.mu.RUnlock()

	if channel == nil {
		return fmt.Errorf("channel is not initialized")
	}

	msgs, err := channel.Consume(
		c.cfg.Queue,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.log.WithField("workers", c.cfg.Workers).Info("starting consumer workers")

	for i := 0; i < c.cfg.Workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, msgs, i)
	}

	<-ctx.Done()
	c.log.Info("stopping consumer workers")
	c.wg.Wait()

	return nil
}

func (c *Consumer) worker(ctx context.Context, msgs <-chan amqp.Delivery, workerID int) {
	defer c.wg.Done()

	c.log.WithField("worker_id", workerID).Debug("worker started")

	for {
		select {
		case <-ctx.Done():
			c.log.WithField("worker_id", workerID).Debug("worker stopped")
			return

		case msg, ok := <-msgs:
			if !ok {
				c.log.WithField("worker_id", workerID).Warn("message channel closed")
				return
			}

			c.processMessage(ctx, msg, workerID)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg amqp.Delivery, workerID int) {
	ctx, cancel := context.WithTimeout(ctx, consumerTimeout)
	defer cancel()

	var payload processor.BalanceMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.log.WithFields(logrus.Fields{
			"worker_id": workerID,
			"error":     err,
			"body":      string(msg.Body),
		}).Error("failed to unmarshal message")

		// Reject and don't requeue malformed messages
		_ = msg.Nack(false, false)
		return
	}

	// Validate payload
	if payload.UserID == 0 {
		c.log.WithFields(logrus.Fields{
			"worker_id": workerID,
			"payload":   payload,
		}).Error("invalid user_id in message")
		_ = msg.Nack(false, false)
		return
	}

	// Validate amount (should be non-negative)
	amount := payload.GetAmount()
	if amount < 0 {
		c.log.WithFields(logrus.Fields{
			"worker_id": workerID,
			"user_id":   payload.UserID,
			"amount":     amount,
		}).Warn("negative amount in message, will process anyway")
	}

	select {
	case c.updates <- processor.IncomingUpdate{
		Payload:  payload,
		Delivery: msg,
	}:
		// Message sent to processor successfully
		c.log.WithFields(logrus.Fields{
			"worker_id": workerID,
			"user_id":   payload.UserID,
			"version":   payload.Version,
		}).Debug("message sent to processor")
	case <-ctx.Done():
		c.log.WithField("worker_id", workerID).Warn("context cancelled while sending message")
		_ = msg.Nack(false, true) // Requeue
		return
	}
}

func (c *Consumer) Close() {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.log.Info("consumer closed")
}
