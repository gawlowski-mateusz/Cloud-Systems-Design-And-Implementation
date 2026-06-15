// get-reservation is the AWS Lambda backing GET /reservations/{id}. It returns a
// single reservation owned by the caller, or 404 if it does not exist.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
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

	id := strings.TrimSpace(req.PathParameters["id"])
	if id == "" {
		return lambdahttp.Error(http.StatusBadRequest, "invalid reservation id"), nil
	}

	reservation, found, err := a.store.GetOne(ctx, userSub, id)
	if err != nil {
		log.Printf("failed to load reservation: %v", err)
		return lambdahttp.Error(http.StatusInternalServerError, "failed to load reservation"), nil
	}
	if !found {
		return lambdahttp.Error(http.StatusNotFound, "reservation not found"), nil
	}

	return lambdahttp.JSON(http.StatusOK, map[string]any{"reservation": reservation.ToAPI()}), nil
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
