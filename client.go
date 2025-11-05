package sqs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub/v2"
	common "github.com/peteraglen/slack-manager-common"
)

type Client struct {
	client       *pubsub.Client
	publisher    *pubsub.Publisher
	subscriber   *pubsub.Subscriber
	topic        string
	subscription string
	opts         *Options
	logger       common.Logger
	initialized  bool
}

func New(c *pubsub.Client, topic string, subscription string, logger common.Logger, opts ...Option) *Client {
	options := newOptions()

	for _, o := range opts {
		o(options)
	}

	logger = logger.WithField("plugin", "pubsub").WithField("topic", topic).WithField("subscription", subscription)

	return &Client{
		client:       c,
		topic:        topic,
		subscription: subscription,
		opts:         options,
		logger:       logger,
	}
}

func (c *Client) Init(_ context.Context) (*Client, error) {
	if c.initialized {
		return c, nil
	}

	if c.topic == "" {
		return nil, errors.New("pub/sub topic cannot be empty")
	}

	if c.subscription == "" {
		return nil, errors.New("pub/sub subscription cannot be empty")
	}

	if err := c.opts.validate(); err != nil {
		return nil, fmt.Errorf("invalid pub/sub client options: %w", err)
	}

	c.publisher = c.client.Publisher(c.topic)

	c.publisher.EnableMessageOrdering = true
	c.publisher.PublishSettings.DelayThreshold = c.opts.publisherDelayThreshold
	c.publisher.PublishSettings.CountThreshold = c.opts.publisherCountThreshold
	c.publisher.PublishSettings.ByteThreshold = c.opts.publisherByteThreshold

	c.subscriber = c.client.Subscriber(c.subscription)

	c.subscriber.ReceiveSettings.MaxExtension = c.opts.subscriberMaxExtension
	c.subscriber.ReceiveSettings.MaxDurationPerAckExtension = c.opts.subscriberMaxDurationPerAckExtension
	c.subscriber.ReceiveSettings.MinDurationPerAckExtension = c.opts.subscriberMinDurationPerAckExtension
	c.subscriber.ReceiveSettings.MaxOutstandingMessages = c.opts.subscriberMaxOutstandingMessages
	c.subscriber.ReceiveSettings.MaxOutstandingBytes = c.opts.subscriberMaxOutstandingBytes

	c.subscriber.ReceiveSettings.ShutdownOptions = &pubsub.ShutdownOptions{
		Behavior: pubsub.ShutdownBehaviorNackImmediately,
		Timeout:  c.opts.subscriberShutdownTimeout,
	}

	c.initialized = true

	return c, nil
}

func (c *Client) Send(ctx context.Context, groupID, dedupID, body string) error {
	if !c.initialized {
		return errors.New("pub/sub client not initialized")
	}

	if groupID == "" {
		return errors.New("groupID cannot be empty")
	}

	if dedupID == "" {
		return errors.New("dedupID cannot be empty")
	}

	if body == "" {
		return errors.New("body cannot be empty")
	}

	msg := &pubsub.Message{
		Data:        []byte(body),
		OrderingKey: groupID,
		Attributes:  map[string]string{"dedup_id": dedupID},
	}

	if _, err := c.publisher.Publish(ctx, msg).Get(ctx); err != nil {
		return fmt.Errorf("failed to publish message to pub/sub topic %s with ordering key %s: %w", c.topic, groupID, err)
	}

	return nil
}

func (c *Client) Name() string {
	return c.topic
}

func (c *Client) Receive(ctx context.Context, sinkCh chan<- *common.FifoQueueItem) error {
	if !c.initialized {
		return errors.New("pub/sub client not initialized")
	}

	defer close(sinkCh)

	c.logger.Debug("Reading pub/sub topic")

	err := c.subscriber.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		ack := func(_ context.Context) {
			msg.Ack()
		}

		nack := func(_ context.Context) {
			msg.Nack()
		}

		item := &common.FifoQueueItem{
			MessageID:        msg.ID,
			SlackChannelID:   msg.OrderingKey,
			ReceiveTimestamp: time.Now(),
			Body:             string(msg.Data),
			Ack:              ack,
			Nack:             nack,
		}

		if err := trySend(ctx, item, sinkCh); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				c.logger.Errorf("Failed to send pub/sub message %s to sink channel: %v", msg.ID, err)
			}
		}

		c.logger.WithField("message_id", msg.ID).Debug("Pub/Sub message received")
	})

	return err
}

func trySend(ctx context.Context, msg *common.FifoQueueItem, sinkCh chan<- *common.FifoQueueItem) error {
	select {
	case sinkCh <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
