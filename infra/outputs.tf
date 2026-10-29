output "region" {
  value = var.region
}

output "media_bucket" {
  description = "Bucket for profile photos and liveness images."
  value       = aws_s3_bucket.media.bucket
}

output "verifications_table" {
  value = aws_dynamodb_table.verifications.name
}

output "identity_pool_id" {
  description = "Goes into the React app's Amplify config (Auth.Cognito.identityPoolId)."
  value       = aws_cognito_identity_pool.liveness.id
}

output "server_role_arn" {
  value = aws_iam_role.server.arn
}

output "aws_config_snippet" {
  description = "Append to ~/.aws/config so the Go server runs as the least-privilege role."
  value       = <<-EOT
    [profile ${var.project}-server]
    role_arn       = ${aws_iam_role.server.arn}
    source_profile = ${coalesce(var.profile, "default")}
    region         = ${var.region}
  EOT
}

output "env_file" {
  description = "Starting point for the repo's .env (Go server) and web/.env (React app)."
  value       = <<-EOT
    # .env (Go server)
    AWS_PROFILE=${var.project}-server
    AWS_REGION=${var.region}
    MEDIA_BUCKET=${aws_s3_bucket.media.bucket}
    VERIFICATIONS_TABLE=${aws_dynamodb_table.verifications.name}
    RETENTION_DAYS=${var.retention_days}

    # web/.env
    VITE_AWS_REGION=${var.region}
    VITE_IDENTITY_POOL_ID=${aws_cognito_identity_pool.liveness.id}
  EOT
}
