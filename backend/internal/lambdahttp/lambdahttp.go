// Package lambdahttp holds small helpers shared by the reservation Lambdas:
// JSON responses with CORS headers and identity extraction from the API Gateway
// JWT authorizer context.
package lambdahttp

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// corsHeaders are returned on every response. The HTTP API also has its own CORS
// configuration; echoing the header keeps non-preflight error responses usable
// from the browser too.
var corsHeaders = map[string]string{
	"Content-Type":                "application/json",
	"Access-Control-Allow-Origin": "*",
}

func JSON(status int, body any) events.APIGatewayV2HTTPResponse {
	encoded, err := json.Marshal(body)
	if err != nil {
		encoded = []byte(`{"error":"failed to encode response"}`)
		status = 500
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    corsHeaders,
		Body:       string(encoded),
	}
}

func Error(status int, message string) events.APIGatewayV2HTTPResponse {
	return JSON(status, map[string]string{"error": message})
}

// UserSub reads the Cognito subject from the JWT authorizer claims. The HTTP API
// rejects unauthenticated calls before invoking the Lambda, so a missing claim
// indicates a misconfigured route rather than an anonymous caller.
func UserSub(req events.APIGatewayV2HTTPRequest) (string, bool) {
	authz := req.RequestContext.Authorizer
	if authz == nil || authz.JWT == nil {
		return "", false
	}
	sub := strings.TrimSpace(authz.JWT.Claims["sub"])
	if sub == "" {
		return "", false
	}
	return sub, true
}

// Body returns the request body, transparently decoding the base64 wrapping that
// API Gateway applies to some payloads.
func Body(req events.APIGatewayV2HTTPRequest) ([]byte, error) {
	if req.IsBase64Encoded {
		return base64.StdEncoding.DecodeString(req.Body)
	}
	return []byte(req.Body), nil
}
