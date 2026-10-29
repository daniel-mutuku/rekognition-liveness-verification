resource "aws_dynamodb_table" "verifications" {
  name         = "${var.project}-verifications"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  # Best-effort deletion; the server also checks expiresAt on read.
  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  point_in_time_recovery {
    enabled = true
  }

}
