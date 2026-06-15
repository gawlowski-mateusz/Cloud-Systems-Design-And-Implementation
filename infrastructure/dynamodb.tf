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

# Reservations moved off RDS to DynamoDB so the Lambdas stay VPC-free. The
# hall_date_index GSI groups bookings by hall and day, which is what the
# create-reservation Lambda queries to detect overlapping time slots.
resource "aws_dynamodb_table" "reservations" {
  name         = "${var.project_name}-reservations"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_sub"
  range_key    = "reservation_id"

  attribute {
    name = "user_sub"
    type = "S"
  }

  attribute {
    name = "reservation_id"
    type = "S"
  }

  attribute {
    name = "hall_id"
    type = "S"
  }

  attribute {
    name = "reservation_date"
    type = "S"
  }

  global_secondary_index {
    name            = "hall_date_index"
    hash_key        = "hall_id"
    range_key       = "reservation_date"
    projection_type = "ALL"
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
