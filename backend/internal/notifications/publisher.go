package notifications

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type Event struct {
	EventType string
	UserSub   string
	Payload   string
}

type Publisher struct {
	client   *sns.Client
	topicARN string
}

func NewPublisher(client *sns.Client, topicARN string) *Publisher {
	return &Publisher{client: client, topicARN: strings.TrimSpace(topicARN)}
}

func (p *Publisher) Publish(ctx context.Context, e Event) error {
	if p == nil || p.client == nil || p.topicARN == "" {
		return errors.New("publisher not configured")
	}
	if e.Payload == "" {
		e.Payload = "{}"
	}
	_, err := p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Message:  aws.String(e.Payload),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"event_type": {
				DataType:    aws.String("String"),
				StringValue: aws.String(e.EventType),
			},
			"user_sub": {
				DataType:    aws.String("String"),
				StringValue: aws.String(safeAttr(e.UserSub)),
			},
		},
	})
	return err
}

func safeAttr(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "broadcast"
	}
	return value
}
