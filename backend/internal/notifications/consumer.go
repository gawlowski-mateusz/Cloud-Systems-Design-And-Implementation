package notifications

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"neurosciolar/backend/internal/dynamostore"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// message mirrors the events.Event contract used by the producers.
type message struct {
	EventID   string `json:"eventId"`
	EventType string `json:"eventType"`
	UserSub   string `json:"userSub"`
	EventTs   string `json:"eventTs"`
	Payload   string `json:"payload"`
}

// Notifier delivers a stored notification to the recipient (e.g. by email). It is
// optional: if nil, the service still persists events (pull-based history).
type Notifier interface {
	Notify(ctx context.Context, n dynamostore.Notification) error
}

type Consumer struct {
	client   *sqs.Client
	queueURL string
	store    *dynamostore.NotificationStore
	notifier Notifier
}

func NewConsumer(client *sqs.Client, queueURL string, store *dynamostore.NotificationStore, notifier Notifier) *Consumer {
	return &Consumer{client: client, queueURL: queueURL, store: store, notifier: notifier}
}

// Run long-polls the queue until ctx is cancelled. A message is deleted only
// after it has been stored (or recognised as a duplicate); anything else is left
// on the queue, and after the redrive policy's maxReceiveCount SQS moves it to
// the Dead Letter Queue configured in Terraform.
func (c *Consumer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("sqs receive error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, m := range out.Messages {
			c.process(ctx, m)
		}
	}
}

func (c *Consumer) process(ctx context.Context, m sqstypes.Message) {
	var evt message
	if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &evt); err != nil {
		// A malformed body can never be processed; leave it so it lands in the DLQ.
		log.Printf("failed to parse sqs message %s: %v", aws.ToString(m.MessageId), err)
		return
	}

	notification := dynamostore.Notification{
		UserSub:   evt.UserSub,
		EventTs:   evt.EventTs,
		EventID:   evt.EventID,
		EventType: evt.EventType,
		Payload:   evt.Payload,
	}
	stored, err := c.store.Put(ctx, notification)
	if err != nil {
		log.Printf("failed to store notification for event %s: %v", evt.EventID, err)
		return
	}
	if stored {
		log.Printf("stored notification: type=%s user=%s event=%s", evt.EventType, evt.UserSub, evt.EventID)
		// Notify only on first store so a re-delivered message never double-emails.
		c.notify(ctx, notification)
	} else {
		log.Printf("duplicate notification skipped: event=%s", evt.EventID)
	}

	if _, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); err != nil {
		log.Printf("failed to delete sqs message %s: %v", aws.ToString(m.MessageId), err)
	}
}

// notify emails the recipient about the stored notification. Failures are logged
// but never block the queue: the item is already persisted and will appear on the
// next history fetch.
func (c *Consumer) notify(ctx context.Context, n dynamostore.Notification) {
	if c.notifier == nil {
		return
	}
	if err := c.notifier.Notify(ctx, n); err != nil {
		log.Printf("failed to email notification for event %s: %v", n.EventID, err)
	}
}
