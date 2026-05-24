package dynamostore

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Profile struct {
	UserSub   string `json:"userSub"  dynamodbav:"user_sub"`
	Email     string `json:"email"    dynamodbav:"email"`
	FullName  string `json:"fullName" dynamodbav:"full_name"`
	CreatedAt string `json:"createdAt" dynamodbav:"created_at"`
}

type ProfileStore struct {
	client *dynamodb.Client
	table  string
}

func NewProfileStore(client *dynamodb.Client, table string) *ProfileStore {
	return &ProfileStore{client: client, table: table}
}

func (s *ProfileStore) Put(ctx context.Context, p Profile) error {
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return err
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *ProfileStore) Get(ctx context.Context, userSub string) (Profile, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]dtypes.AttributeValue{
			"user_sub": &dtypes.AttributeValueMemberS{Value: userSub},
		},
	})
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if out.Item == nil {
		return p, nil
	}
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return Profile{}, err
	}
	return p, nil
}
