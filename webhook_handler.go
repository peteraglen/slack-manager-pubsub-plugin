package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/pubsub/v2"
	common "github.com/peteraglen/slack-manager-common"
)

// topicNameRegex validates Pub/Sub topic resource names.
// Project IDs may contain colons for domain-prefixed projects (e.g., google.com:my-project).
// Topic names must start with a letter, followed by 2-254 word characters, dots, underscores, or hyphens.
var topicNameRegex = regexp.MustCompile(`^projects\/([a-z][a-z0-9-:.]{5,29})\/topics\/([a-zA-Z][\w._-]{2,254})$`)

// WebhookHandler handles webhook callbacks by publishing them to Pub/Sub topics.
type WebhookHandler struct {
	client *pubsub.Client

	// The publisher cache is unbounded and assumes a small, finite set of topics.
	publishers     map[string]*pubsub.Publisher
	publishersLock sync.RWMutex

	isOrdered   bool
	opts        *Options
	initialized atomic.Bool
}

func NewWebhookHandler(c *pubsub.Client, isOrdered bool, opts ...Option) *WebhookHandler {
	options := newOptions()

	for _, o := range opts {
		o(options)
	}

	return &WebhookHandler{
		client:     c,
		publishers: make(map[string]*pubsub.Publisher),
		isOrdered:  isOrdered,
		opts:       options,
	}
}

func (c *WebhookHandler) Init(_ context.Context) (*WebhookHandler, error) {
	if c.initialized.Load() {
		return c, nil
	}

	if err := c.opts.validatePublisher(); err != nil {
		return nil, fmt.Errorf("invalid pub/sub publisher options: %w", err)
	}

	c.initialized.Store(true)

	return c, nil
}

// Close stops all cached publishers, flushing any pending messages.
func (c *WebhookHandler) Close() {
	c.publishersLock.Lock()
	defer c.publishersLock.Unlock()

	for _, publisher := range c.publishers {
		publisher.Stop()
	}
}

func (c *WebhookHandler) ShouldHandleWebhook(_ context.Context, target string) bool {
	return topicNameRegex.MatchString(target)
}

// HandleWebhook publishes the webhook callback data to the specified Pub/Sub topic.
// Callers should use ShouldHandleWebhook to validate the topic name before calling this method.
func (c *WebhookHandler) HandleWebhook(ctx context.Context, topic string, data *common.WebhookCallback, logger common.Logger) error {
	if !c.initialized.Load() {
		return errors.New("pub/sub webhook handler not initialized")
	}

	if data == nil {
		return errors.New("webhook callback data cannot be nil")
	}

	if logger == nil {
		return errors.New("logger cannot be nil")
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

	if c.isOrdered {
		logger.Debugf("Webhook body sent to pub/sub topic %s with ordering key %s", topic, data.ChannelID)
	} else {
		logger.Debugf("Webhook body sent to pub/sub topic %s", topic)
	}

	return nil
}

func (c *WebhookHandler) getPublisher(topic string) *pubsub.Publisher {
	// Fast path: read lock
	c.publishersLock.RLock()
	publisher, exists := c.publishers[topic]
	c.publishersLock.RUnlock()

	if exists {
		return publisher
	}

	// Slow path: write lock with double-check
	c.publishersLock.Lock()
	defer c.publishersLock.Unlock()

	// Double-check after acquiring write lock
	if publisher, exists = c.publishers[topic]; exists {
		return publisher
	}

	publisher = c.client.Publisher(topic)
	publisher.EnableMessageOrdering = c.isOrdered
	publisher.PublishSettings.DelayThreshold = c.opts.publisherDelayThreshold
	publisher.PublishSettings.CountThreshold = c.opts.publisherCountThreshold
	publisher.PublishSettings.ByteThreshold = c.opts.publisherByteThreshold

	c.publishers[topic] = publisher

	return publisher
}
