package dynamostore

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type FileMetadata struct {
	UserSub      string `json:"userSub"      dynamodbav:"user_sub"`
	FileID       string `json:"fileId"       dynamodbav:"file_id"`
	OriginalName string `json:"originalName" dynamodbav:"original_name"`
	ContentType  string `json:"contentType"  dynamodbav:"content_type"`
	SizeBytes    int64  `json:"sizeBytes"    dynamodbav:"size_bytes"`
	ObjectKey    string `json:"objectKey"    dynamodbav:"object_key"`
	CreatedAt    string `json:"createdAt"    dynamodbav:"created_at"`
}

type FileMetadataStore struct {
	client *dynamodb.Client
	table  string
}

func NewFileMetadataStore(client *dynamodb.Client, table string) *FileMetadataStore {
	return &FileMetadataStore{client: client, table: table}
}

func (s *FileMetadataStore) Put(ctx context.Context, f FileMetadata) error {
	if f.CreatedAt == "" {
		f.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	item, err := attributevalue.MarshalMap(f)
	if err != nil {
		return err
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	return err
}

func (s *FileMetadataStore) Get(ctx context.Context, userSub, fileID string) (FileMetadata, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]dtypes.AttributeValue{
			"user_sub": &dtypes.AttributeValueMemberS{Value: userSub},
			"file_id":  &dtypes.AttributeValueMemberS{Value: fileID},
		},
	})
	if err != nil {
		return FileMetadata{}, false, err
	}
	if out.Item == nil {
		return FileMetadata{}, false, nil
	}
	var f FileMetadata
	if err := attributevalue.UnmarshalMap(out.Item, &f); err != nil {
		return FileMetadata{}, false, err
	}
	return f, true, nil
}

func (s *FileMetadataStore) ListByUser(ctx context.Context, userSub string) ([]FileMetadata, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		KeyConditionExpression: aws.String("user_sub = :u"),
		ExpressionAttributeValues: map[string]dtypes.AttributeValue{
			":u": &dtypes.AttributeValueMemberS{Value: userSub},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}
	files := make([]FileMetadata, 0, len(out.Items))
	for _, item := range out.Items {
		var f FileMetadata
		if err := attributevalue.UnmarshalMap(item, &f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}
