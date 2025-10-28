terraform {
  required_providers {
    # Provider WITH version constraint - will show if outdated
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }

    # Provider WITHOUT version constraint - will NOT show (assumed latest)
    google = {
      source = "hashicorp/google"
    }
  }
}
