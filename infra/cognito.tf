# The browser streams video straight to Rekognition and needs short-lived
# credentials for that. Guest access is fine: the role can only stream into a
# session the server created, and the server decides the outcome.

resource "aws_cognito_identity_pool" "liveness" {
  identity_pool_name               = "${replace(var.project, "-", "_")}_liveness"
  allow_unauthenticated_identities = true
  allow_classic_flow               = false
}

data "aws_iam_policy_document" "liveness_guest_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = ["cognito-identity.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "cognito-identity.amazonaws.com:aud"
      values   = [aws_cognito_identity_pool.liveness.id]
    }

    condition {
      test     = "ForAnyValue:StringLike"
      variable = "cognito-identity.amazonaws.com:amr"
      values   = ["unauthenticated"]
    }
  }
}

resource "aws_iam_role" "liveness_guest" {
  name                 = "${var.project}-liveness-guest"
  description          = "Browser role: stream a selfie video to an existing Face Liveness session, nothing else."
  assume_role_policy   = data.aws_iam_policy_document.liveness_guest_trust.json
  max_session_duration = 3600
}

data "aws_iam_policy_document" "liveness_guest" {
  statement {
    sid     = "StreamLivenessVideo"
    actions = ["rekognition:StartFaceLivenessSession"]
    # No resource-level permissions for this action.
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "liveness_guest" {
  name   = "start-face-liveness-session"
  role   = aws_iam_role.liveness_guest.id
  policy = data.aws_iam_policy_document.liveness_guest.json
}

resource "aws_cognito_identity_pool_roles_attachment" "liveness" {
  identity_pool_id = aws_cognito_identity_pool.liveness.id

  roles = {
    unauthenticated = aws_iam_role.liveness_guest.arn
  }
}
