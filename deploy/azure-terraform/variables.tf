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
