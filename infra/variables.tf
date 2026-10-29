variable "profile" {
  description = "AWS CLI profile Terraform uses to deploy. null = SDK default credential chain."
  type        = string
  default     = null
}

variable "region" {
  description = "AWS region. Face Liveness only runs in a few regions, so this is validated."
  type        = string
  default     = "eu-west-1"

  validation {
    # Face Liveness regions. The bucket must be in the same region.
    condition     = contains(["us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1", "ap-south-1"], var.region)
    error_message = "Face Liveness is only available in us-east-1, us-west-2, eu-west-1, ap-northeast-1 and ap-south-1."
  }
}

variable "project" {
  description = "Name prefix for every resource. Lowercase letters, digits and hyphens."
  type        = string
  default     = "face-verify"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,20}$", var.project))
    error_message = "Use 3-21 lowercase letters, digits or hyphens, starting with a letter."
  }
}

variable "bucket_suffix" {
  description = "Makes the bucket name globally unique, e.g. your initials plus a few digits."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]{3,20}$", var.bucket_suffix))
    error_message = "Use 3-20 lowercase letters, digits or hyphens."
  }
}

variable "allowed_origins" {
  description = "Origins allowed to PUT the profile photo straight to S3 (CORS). Must match where the page is served."
  type        = list(string)
  default     = ["http://localhost:5173", "http://localhost:8080"]
}

variable "retention_days" {
  description = "Photos, liveness images and verification records are deleted after this many days. Biometric data should not live forever."
  type        = number
  default     = 7

  validation {
    condition     = var.retention_days >= 1 && var.retention_days <= 365
    error_message = "retention_days must be between 1 and 365."
  }
}
