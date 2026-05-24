// Default config for local development. Terraform overwrites this file in the
// frontend S3 bucket with the deployed ALB DNS name and Cognito IDs.
window.APP_CONFIG = window.APP_CONFIG || {
  API_BASE_URL: "http://localhost:8080",
  COGNITO_USER_POOL_ID: "",
  COGNITO_CLIENT_ID: "",
  AWS_REGION: "us-east-1"
};
