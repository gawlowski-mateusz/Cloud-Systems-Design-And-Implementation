// Package events publishes application events to Amazon SQS. It replaces the
// previous direct/SNS coupling: every producer (reservation Lambdas, files
// service) drops a message on the shared queue, and the notifications Fargate
// service consumes it asynchronously.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// BroadcastUserSub marks events not tied to a single recipient.
const BroadcastUserSub = "*"

// Event is the message contract shared between producers and the consumer.
// EventID + EventTs are stamped by the producer so the consumer can deduplicate
// re-delivered messages (SQS guarantees at-least-once delivery).
type Event struct {
	EventID   string `json:"eventId"`
	EventType string `json:"eventType"`
	UserSub   string `json:"userSub"`
	EventTs   string `json:"eventTs"`
	Payload   string `json:"payload"`
}

type Publisher struct {
	client   *sqs.Client
	queueURL string
}

func NewPublisher(client *sqs.Client, queueURL string) *Publisher {
	return &Publisher{client: client, queueURL: strings.TrimSpace(queueURL)}
}

func (p *Publisher) Publish(ctx context.Context, e Event) error {
	if p == nil || p.client == nil || p.queueURL == "" {
		return errors.New("sqs publisher not configured")
	}
	if e.EventID == "" {
		e.EventID = newEventID()
	}
	if e.EventTs == "" {
		e.EventTs = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(e.UserSub) == "" {
		e.UserSub = BroadcastUserSub
	}
	if e.Payload == "" {
		e.Payload = "{}"
	}

	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

func newEventID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
