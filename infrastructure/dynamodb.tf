resource "aws_dynamodb_table" "auth_profiles" {
  name         = "${var.project_name}-auth-profiles"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_sub"

  attribute {
    name = "user_sub"
    type = "S"
  }
}

resource "aws_dynamodb_table" "file_metadata" {
  name         = "${var.project_name}-file-metadata"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_sub"
  range_key    = "file_id"

  attribute {
    name = "user_sub"
    type = "S"
  }

  attribute {
    name = "file_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "notification_history" {
  name         = "${var.project_name}-notification-history"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_sub"
  range_key    = "event_ts"

  attribute {
    name = "user_sub"
    type = "S"
  }

  attribute {
    name = "event_ts"
    type = "S"
  }
}
