// Package reservationstore is the DynamoDB persistence layer for conference-hall
// reservations. It replaces the former RDS PostgreSQL store so the reservation
// Lambdas stay fully serverless (no VPC attachment, no connection pooling).
package reservationstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// hallDateIndex is the GSI used for conflict detection: it groups reservations
// by hall and day so an overlap check only scans the relevant slots.
const hallDateIndex = "hall_date_index"

type Reservation struct {
	UserSub       string `dynamodbav:"user_sub"`
	ReservationID string `dynamodbav:"reservation_id"`
	HallID        string `dynamodbav:"hall_id"`
	Date          string `dynamodbav:"reservation_date"`
	StartTime     string `dynamodbav:"start_time"`
	EndTime       string `dynamodbav:"end_time"`
	Attendees     int    `dynamodbav:"attendees"`
	Purpose       string `dynamodbav:"purpose"`
	CreatedAt     string `dynamodbav:"created_at"`
}

// APIReservation is the JSON shape returned to the frontend (kept identical to
// the previous reservations microservice so the client needs no changes).
type APIReservation struct {
	ID        string `json:"id"`
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

func (r Reservation) ToAPI() APIReservation {
	return APIReservation{
		ID:        r.ReservationID,
		HallID:    r.HallID,
		Date:      r.Date,
		StartTime: r.StartTime,
		EndTime:   r.EndTime,
		Attendees: r.Attendees,
		Purpose:   r.Purpose,
	}
}

type Store struct {
	client *dynamodb.Client
	table  string
}

func New(client *dynamodb.Client, table string) *Store {
	return &Store{client: client, table: table}
}

// HasConflict reports whether the given hall is already booked for an
// overlapping slot on the same day. Two slots overlap when the existing start is
// before the new end AND the existing end is after the new start — the same
// predicate the old SQL conflict query used.
func (s *Store) HasConflict(ctx context.Context, hallID, date, start, end string) (bool, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		IndexName:              aws.String(hallDateIndex),
		KeyConditionExpression: aws.String("hall_id = :h AND reservation_date = :d"),
		ExpressionAttributeValues: map[string]dtypes.AttributeValue{
			":h": &dtypes.AttributeValueMemberS{Value: hallID},
			":d": &dtypes.AttributeValueMemberS{Value: date},
		},
	})
	if err != nil {
		return false, err
	}
	for _, item := range out.Items {
		var r Reservation
		if err := attributevalue.UnmarshalMap(item, &r); err != nil {
			return false, err
		}
		if r.StartTime < end && r.EndTime > start {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) Create(ctx context.Context, r Reservation) (Reservation, error) {
	r.ReservationID = newReservationID()
	r.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	item, err := attributevalue.MarshalMap(r)
	if err != nil {
		return Reservation{}, err
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	if err != nil {
		return Reservation{}, err
	}
	return r, nil
}

func (s *Store) ListByUser(ctx context.Context, userSub string) ([]Reservation, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.table),
		KeyConditionExpression: aws.String("user_sub = :u"),
		ExpressionAttributeValues: map[string]dtypes.AttributeValue{
			":u": &dtypes.AttributeValueMemberS{Value: userSub},
		},
	})
	if err != nil {
		return nil, err
	}
	items := make([]Reservation, 0, len(out.Items))
	for _, raw := range out.Items {
		var r Reservation
		if err := attributevalue.UnmarshalMap(raw, &r); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, nil
}

func (s *Store) GetOne(ctx context.Context, userSub, reservationID string) (Reservation, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]dtypes.AttributeValue{
			"user_sub":       &dtypes.AttributeValueMemberS{Value: userSub},
			"reservation_id": &dtypes.AttributeValueMemberS{Value: reservationID},
		},
	})
	if err != nil {
		return Reservation{}, false, err
	}
	if out.Item == nil {
		return Reservation{}, false, nil
	}
	var r Reservation
	if err := attributevalue.UnmarshalMap(out.Item, &r); err != nil {
		return Reservation{}, false, err
	}
	return r, true, nil
}

func newReservationID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(buf))
}
