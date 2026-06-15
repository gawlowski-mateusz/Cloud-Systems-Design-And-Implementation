package dynamostore

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const BroadcastUserSub = "*"

type Notification struct {
	UserSub   string `json:"userSub"   dynamodbav:"user_sub"`
	EventTs   string `json:"eventTs"   dynamodbav:"event_ts"`
	EventID   string `json:"eventId"   dynamodbav:"event_id"`
	EventType string `json:"eventType" dynamodbav:"event_type"`
	Payload   string `json:"payload"   dynamodbav:"payload"`
	CreatedAt string `json:"createdAt" dynamodbav:"created_at"`
}

type NotificationStore struct {
	client *dynamodb.Client
	table  string
}

func NewNotificationStore(client *dynamodb.Client, table string) *NotificationStore {
	return &NotificationStore{client: client, table: table}
}

// Put stores a notification idempotently. The (user_sub, event_ts) key is stamped
// by the producer and carried in the SQS message, so a re-delivered message maps
// to the same item. The conditional write reports (false, nil) on a duplicate,
// letting the consumer delete the message without creating a second notification.
func (s *NotificationStore) Put(ctx context.Context, n Notification) (bool, error) {
	if n.EventTs == "" {
		n.EventTs = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if n.CreatedAt == "" {
		n.CreatedAt = n.EventTs
	}
	if n.UserSub == "" {
		n.UserSub = BroadcastUserSub
	}
	item, err := attributevalue.MarshalMap(n)
	if err != nil {
		return false, err
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(user_sub)"),
	})
	if err != nil {
		var conflict *dtypes.ConditionalCheckFailedException
		if errors.As(err, &conflict) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *NotificationStore) ListByUser(ctx context.Context, userSub string, limit int32) ([]Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	notifications := make([]Notification, 0, limit*2)

	for _, sub := range []string{userSub, BroadcastUserSub} {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			KeyConditionExpression: aws.String("user_sub = :u"),
			ExpressionAttributeValues: map[string]dtypes.AttributeValue{
				":u": &dtypes.AttributeValueMemberS{Value: sub},
			},
			Limit:            aws.Int32(limit),
			ScanIndexForward: aws.Bool(false),
		})
		if err != nil {
			return nil, err
		}
		for _, item := range out.Items {
			var n Notification
			if err := attributevalue.UnmarshalMap(item, &n); err != nil {
				return nil, err
			}
			notifications = append(notifications, n)
		}
	}

	return notifications, nil
}
