variable "ipzilon_token" {
  description = "IPzilon API token (ipam_...)."
  type        = string
  sensitive   = true
}

variable "ipzilon_api_url" {
  description = "IPzilon API base URL."
  type        = string
  default     = "http://localhost:8000"
}

variable "site_id" {
  description = "ID of an existing IPzilon site."
  type        = number
}
