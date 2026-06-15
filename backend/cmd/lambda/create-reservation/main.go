// create-reservation is the AWS Lambda backing POST /reservations. It validates
// the request, rejects overlapping bookings, persists to DynamoDB and emits a
// ReservationCreated event to SQS for the notifications service to pick up.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/events"
	"neurosciolar/backend/internal/lambdahttp"
	"neurosciolar/backend/internal/reservationstore"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type createRequest struct {
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

type app struct {
	store     *reservationstore.Store
	publisher *events.Publisher
}

func (a *app) handle(ctx context.Context, req awsevents.APIGatewayV2HTTPRequest) (awsevents.APIGatewayV2HTTPResponse, error) {
	userSub, ok := lambdahttp.UserSub(req)
	if !ok {
		return lambdahttp.Error(http.StatusUnauthorized, "unauthorized"), nil
	}

	raw, err := lambdahttp.Body(req)
	if err != nil {
		return lambdahttp.Error(http.StatusBadRequest, "invalid request body"), nil
	}
	var in createRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		return lambdahttp.Error(http.StatusBadRequest, "invalid request body"), nil
	}
	in.HallID = strings.TrimSpace(in.HallID)
	in.Purpose = strings.TrimSpace(in.Purpose)

	reservationDate, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return lambdahttp.Error(http.StatusBadRequest, "invalid date format"), nil
	}
	if _, err := time.Parse("15:04", in.StartTime); err != nil {
		return lambdahttp.Error(http.StatusBadRequest, "invalid start time format"), nil
	}
	if _, err := time.Parse("15:04", in.EndTime); err != nil {
		return lambdahttp.Error(http.StatusBadRequest, "invalid end time format"), nil
	}
	if in.HallID == "" || in.Purpose == "" || in.Attendees < 1 {
		return lambdahttp.Error(http.StatusBadRequest, "hallId, purpose and attendees are required"), nil
	}
	if in.StartTime >= in.EndTime {
		return lambdahttp.Error(http.StatusBadRequest, "end time must be after start time"), nil
	}

	date := reservationDate.Format("2006-01-02")
	conflict, err := a.store.HasConflict(ctx, in.HallID, date, in.StartTime, in.EndTime)
	if err != nil {
		log.Printf("conflict check failed: %v", err)
		return lambdahttp.Error(http.StatusInternalServerError, "failed to validate reservation"), nil
	}
	if conflict {
		return lambdahttp.Error(http.StatusConflict, "selected hall is already reserved for this time slot"), nil
	}

	created, err := a.store.Create(ctx, reservationstore.Reservation{
		UserSub:   userSub,
		HallID:    in.HallID,
		Date:      date,
		StartTime: in.StartTime,
		EndTime:   in.EndTime,
		Attendees: in.Attendees,
		Purpose:   in.Purpose,
	})
	if err != nil {
		log.Printf("failed to create reservation: %v", err)
		return lambdahttp.Error(http.StatusInternalServerError, "failed to create reservation"), nil
	}

	a.publishEvent(ctx, userSub, created.ToAPI())

	return lambdahttp.JSON(http.StatusCreated, map[string]any{
		"message":     "reservation created successfully",
		"reservation": created.ToAPI(),
	}), nil
}

// publishEvent is best-effort: a failed notification must not fail the booking
// that already succeeded, so the error is logged and swallowed.
func (a *app) publishEvent(ctx context.Context, userSub string, payload reservationstore.APIReservation) {
	body, _ := json.Marshal(payload)
	if err := a.publisher.Publish(ctx, events.Event{
		EventType: "ReservationCreated",
		UserSub:   userSub,
		Payload:   string(body),
	}); err != nil {
		log.Printf("failed to publish ReservationCreated event: %v", err)
	}
}

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	a := &app{
		store:     reservationstore.New(dynamodb.NewFromConfig(cfg), requireEnv("RESERVATIONS_TABLE")),
		publisher: events.NewPublisher(sqs.NewFromConfig(cfg), requireEnv("SQS_QUEUE_URL")),
	}
	lambda.Start(a.handle)
}

func requireEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}
