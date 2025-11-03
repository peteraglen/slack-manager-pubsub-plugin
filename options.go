package sqs

import (
	"errors"
	"time"
)

type Option func(*Options)

type Options struct {
	publisherDelayThreshold              time.Duration
	publisherCountThreshold              int
	publiserByteThreshold                int
	subscriberMaxExtension               time.Duration
	subscriberMaxDurationPerAckExtension time.Duration
	subscriberMinDurationPerAckExtension time.Duration
	subscriberMaxOutstandingMessages     int
	subscriberMaxOutstandingBytes        int
	subscriberShutdownTimeout            time.Duration
}

func newOptions() *Options {
	return &Options{
		publisherDelayThreshold:              10 * time.Millisecond,
		publisherCountThreshold:              100,
		publiserByteThreshold:                1e6, // 1 MB
		subscriberMaxExtension:               10 * time.Minute,
		subscriberMaxDurationPerAckExtension: time.Minute,
		subscriberMinDurationPerAckExtension: 10 * time.Second,
		subscriberMaxOutstandingMessages:     100,
		subscriberMaxOutstandingBytes:        1e6, // 1 MB
		subscriberShutdownTimeout:            time.Second,
	}
}

func (o *Options) validate() error {
	if o.publisherDelayThreshold < 0 {
		return errors.New("delay threshold must be non-negative")
	}

	if o.publisherCountThreshold <= 0 {
		return errors.New("count threshold must be greater than zero")
	}

	if o.publiserByteThreshold <= 0 {
		return errors.New("byte threshold must be greater than zero")
	}

	return nil
}

func WithPublisherDelayThreshold(d time.Duration) Option {
	return func(o *Options) {
		o.publisherDelayThreshold = d
	}
}

func WithPublisherCountThreshold(n int) Option {
	return func(o *Options) {
		o.publisherCountThreshold = n
	}
}

func WithPublisherByteThreshold(n int) Option {
	return func(o *Options) {
		o.publiserByteThreshold = n
	}
}

func WithSubscriberMaxExtension(d time.Duration) Option {
	return func(o *Options) {
		o.subscriberMaxExtension = d
	}
}

func WithSubscriberMaxDurationPerAckExtension(d time.Duration) Option {
	return func(o *Options) {
		o.subscriberMaxDurationPerAckExtension = d
	}
}

func WithSubscriberMinDurationPerAckExtension(d time.Duration) Option {
	return func(o *Options) {
		o.subscriberMinDurationPerAckExtension = d
	}
}

func WithSubscriberMaxOutstandingMessages(n int) Option {
	return func(o *Options) {
		o.subscriberMaxOutstandingMessages = n
	}
}

func WithSubscriberMaxOutstandingBytes(n int) Option {
	return func(o *Options) {
		o.subscriberMaxOutstandingBytes = n
	}
}

func WithSubscriberShutdownTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.subscriberShutdownTimeout = d
	}
}
