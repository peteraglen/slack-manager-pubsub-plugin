package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"cloud.google.com/go/pubsub/v2"
	common "github.com/peteraglen/slack-manager-common"
)

var topicNameRegex = regexp.MustCompile(`^projects\/([a-z][a-z0-9-]{5,29})\/topics\/([a-zA-Z0-9._-]{3,255})$`)

type WebhookHandler struct {
	client         *pubsub.Client
	publishers     map[string]*pubsub.Publisher
	publishersLock *sync.Mutex
	isOrdered      bool
	opts           *Options
	initialized    bool
}

func NewWebhookHandler(c *pubsub.Client, isOrdered bool, opts ...Option) *WebhookHandler {
	options := newOptions()

	for _, o := range opts {
		o(options)
	}

	return &WebhookHandler{
		client:         c,
		publishers:     make(map[string]*pubsub.Publisher),
		publishersLock: &sync.Mutex{},
		isOrdered:      isOrdered,
		opts:           options,
	}
}

func (c *WebhookHandler) Init(_ context.Context) (*WebhookHandler, error) {
	if c.initialized {
		return c, nil
	}

	c.initialized = true

	return c, nil
}

func (c *WebhookHandler) ShouldHandleWebhook(_ context.Context, target string) bool {
	return topicNameRegex.MatchString(target)
}

func (c *WebhookHandler) HandleWebhook(ctx context.Context, topic string, data *common.WebhookCallback, logger common.Logger) error {
	if !c.initialized {
		return errors.New("pub/sub webhook handler not initialized")
	}

	publisher := c.getPublisher(topic)

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook callback data: %w", err)
	}

	msg := &pubsub.Message{
		Data: body,
	}

	if c.isOrdered {
		msg.OrderingKey = data.ChannelID
	}

	result := publisher.Publish(ctx, msg)

	if _, err = result.Get(ctx); err != nil {
		return fmt.Errorf("failed to publish message to pub/sub topic %s: %w", topic, err)
	}

	logger.Debugf("Webhook body sent to pub/sub topic %s with ordering key %s", topic, data.ChannelID)

	return nil
}

func (c *WebhookHandler) getPublisher(topic string) *pubsub.Publisher {
	c.publishersLock.Lock()
	defer c.publishersLock.Unlock()

	publisher, exists := c.publishers[topic]
	if !exists {
		publisher = c.client.Publisher(topic)
		publisher.EnableMessageOrdering = c.isOrdered
		publisher.PublishSettings.DelayThreshold = c.opts.publisherDelayThreshold
		publisher.PublishSettings.CountThreshold = c.opts.publisherCountThreshold
		publisher.PublishSettings.ByteThreshold = c.opts.publiserByteThreshold

		c.publishers[topic] = publisher
	}

	return publisher
}
