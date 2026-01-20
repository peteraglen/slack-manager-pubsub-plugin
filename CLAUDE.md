# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make init             # Download and tidy dependencies (go mod tidy)
make test             # Run security scan, format check, tests with coverage and race detection, and vet
make lint             # Run golangci-lint
make lint-fix         # Run golangci-lint with auto-fix
make bump-common-lib  # Update slack-manager-common to latest version
```

Run a single test:
```bash
go test -v -run TestName ./...
```

**IMPORTANT:** Both `make test` and `make lint` MUST pass with zero errors before committing any changes. This applies regardless of whether the errors were introduced by your changes or existed previously - all issues must be resolved before committing. Always run both commands to verify code quality.

## Architecture

This is a Google Cloud Pub/Sub plugin for the Slack Manager system. It provides message queue integration following a plugin architecture consistent with other Slack Manager plugins (SQS, DynamoDB, PostgreSQL).

### Core Components

- **client.go** - Main `Client` struct implementing publisher/subscriber patterns for Pub/Sub. Uses `common.FifoQueueItem` for FIFO queue semantics with message ordering via `OrderingKey` and deduplication via message attributes. Subscription is optional (supports publisher-only mode).

- **options.go** - Functional options pattern (`WithPublisher*`, `WithSubscriber*`) for configuring publisher thresholds and subscriber receive settings.

- **webhook_handler.go** - `WebhookHandler` for dynamic topic publishing. Caches publishers per topic, validates topic names against GCP naming regex, and converts `common.WebhookCallback` to JSON for publishing.

### Dependencies

- `cloud.google.com/go/pubsub/v2` - Google Cloud Pub/Sub client
- `github.com/peteraglen/slack-manager-common` - Shared interfaces (`Logger`, `FifoQueueItem`, `WebhookCallback`)

### Usage Pattern

1. Create Google Cloud Pub/Sub client: `pubsub.NewClient(ctx, projectID)`
2. Instantiate with `New()` or `NewWebhookHandler()`
3. Configure with functional options
4. Call `Init()` to prepare
5. Use `Send()` for publishing, `Receive()` for subscribing
