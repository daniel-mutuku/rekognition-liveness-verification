# The server assumes this role through a profile (aws_config_snippet output).
# No access keys: they'd sit in plain text in the state file.

data "aws_iam_policy_document" "server_trust" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${local.account_id}:root"]
    }
  }
}

resource "aws_iam_role" "server" {
  name                 = "${var.project}-server"
  description          = "Go API server: create verifications, run Rekognition, read/write the media bucket and table."
  assume_role_policy   = data.aws_iam_policy_document.server_trust.json
  max_session_duration = 3600
}

data "aws_iam_policy_document" "server" {
  statement {
    sid = "Rekognition"
    actions = [
      "rekognition:DetectFaces",
      "rekognition:CreateFaceLivenessSession",
      "rekognition:GetFaceLivenessSessionResults",
      "rekognition:CompareFaces",
    ]
    # No resource-level permissions for these actions.
    resources = ["*"]
  }

  statement {
    sid = "MediaObjects"
    # Presigned URLs and Rekognition's own S3 reads/writes all run as this role.
    actions = ["s3:PutObject", "s3:GetObject"]
    resources = [
      "${aws_s3_bucket.media.arn}/${local.profiles_prefix}*",
      "${aws_s3_bucket.media.arn}/${local.liveness_prefix}*",
    ]
  }

  statement {
    sid = "MediaList"
    # Without it, a missing object is a 403 instead of a 404.
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.media.arn]
  }

  statement {
    sid       = "Verifications"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem"]
    resources = [aws_dynamodb_table.verifications.arn]
  }
}

resource "aws_iam_role_policy" "server" {
  name   = "server"
  role   = aws_iam_role.server.id
  policy = data.aws_iam_policy_document.server.json
}
