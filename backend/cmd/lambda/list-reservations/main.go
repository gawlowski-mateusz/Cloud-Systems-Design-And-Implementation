// list-reservations is the AWS Lambda backing GET /reservations. It returns the
// caller's reservations, sorted by date and start time.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/lambdahttp"
	"neurosciolar/backend/internal/reservationstore"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type app struct {
	store *reservationstore.Store
}

func (a *app) handle(ctx context.Context, req awsevents.APIGatewayV2HTTPRequest) (awsevents.APIGatewayV2HTTPResponse, error) {
	userSub, ok := lambdahttp.UserSub(req)
	if !ok {
		return lambdahttp.Error(http.StatusUnauthorized, "unauthorized"), nil
	}

	items, err := a.store.ListByUser(ctx, userSub)
	if err != nil {
		log.Printf("failed to load reservations: %v", err)
		return lambdahttp.Error(http.StatusInternalServerError, "failed to load reservations"), nil
	}

	out := make([]reservationstore.APIReservation, 0, len(items))
	for _, r := range items {
		out = append(out, r.ToAPI())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].StartTime < out[j].StartTime
	})

	return lambdahttp.JSON(http.StatusOK, map[string]any{"reservations": out}), nil
}

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	a := &app{store: reservationstore.New(dynamodb.NewFromConfig(cfg), requireEnv("RESERVATIONS_TABLE"))}
	lambda.Start(a.handle)
}

func requireEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}
