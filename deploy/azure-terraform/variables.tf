variable "name" {
  type        = string
  default     = "blacklight"
  description = "Base name for the deployment. Seeds the Container App, resource group, environment and identity names. Lowercase alphanumeric, hyphens allowed in the middle."

  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?$", var.name))
    error_message = "name must be lowercase alphanumeric (hyphens allowed only in the middle)."
  }
}

variable "location" {
  type        = string
  default     = "eastus"
  description = "Azure region for every resource."
}

variable "image" {
  type        = string
  default     = "ghcr.io/bryanster/blacklight"
  description = "Container image repository."
}

variable "image_tag" {
  type        = string
  default     = "v1.0.0"
  description = "Container image tag. Pin to a release for reproducible deployments."
}

variable "container_cpu" {
  type        = number
  default     = 1
  description = "vCPU for the app container. Must pair with container_memory in a valid Consumption-plan combination."
}

variable "container_memory" {
  type        = string
  default     = "2Gi"
  description = "Memory for the app container."
}

variable "storage_share_quota_gb" {
  type        = number
  default     = 100
  description = "Quota for the Azure Files share holding the database and evidence, in GiB."
}

# ─── secrets ─────────────────────────────────────────────────────────────────
#
# The two secret variables are `ephemeral`: Terraform accepts them from tfvars,
# -var or TF_VAR_*, uses them during the run, and never records them in state or
# in a plan file. Leave both unset and Terraform generates the values instead.

variable "session_secret" {
  type        = string
  default     = null
  sensitive   = true
  ephemeral   = true
  description = "BLACKLIGHT_SESSION_SECRET to store in Key Vault. Leave unset to have Terraform generate one. Supply it to reuse an existing secret — e.g. when adopting a deployment created before this config wrote secrets write-only."

  validation {
    condition     = var.session_secret == null || length(coalesce(var.session_secret, "")) >= 32
    error_message = "session_secret must carry at least 32 bytes of secret material."
  }
}

variable "encryption_key" {
  type        = string
  default     = null
  sensitive   = true
  ephemeral   = true
  description = "BLACKLIGHT_ENCRYPTION_KEY to store in Key Vault. Leave unset to have Terraform generate one. Must differ from session_secret; changing it makes every enrolled MFA authenticator, recovery code and service token unreadable."

  validation {
    condition     = var.encryption_key == null || length(coalesce(var.encryption_key, "")) >= 32
    error_message = "encryption_key must carry at least 32 bytes of secret material."
  }
}

variable "session_secret_version" {
  type        = number
  default     = 1
  description = "Rotation counter for the session secret. The stored value is write-only, so it is only rewritten when this number changes. Bumping it signs every user out."
}

variable "encryption_key_version" {
  type        = number
  default     = 1
  description = "Rotation counter for the encryption key. The stored value is write-only, so it is only rewritten when this number changes. Bumping it without re-encrypting first locks every user out of MFA — see README."
}
