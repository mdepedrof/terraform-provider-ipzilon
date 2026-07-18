terraform {
  required_providers {
    ipzilon = {
      source  = "registry.terraform.io/mdepedrof/ipzilon"
      version = "~> 1.0"
    }
  }
}

provider "ipzilon" {
  api_url = "https://ipzilon.example.com"  # or env: IPZILON_API_URL
  token   = var.ipzilon_token           # or env: IPZILON_TOKEN
}
