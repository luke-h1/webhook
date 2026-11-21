provider "aws" {
  region = "eu-west-2"
}

terraform {
  backend "s3" {
    bucket       = "webhook-production-tfstate"
    key          = "terraform.tfstate"
    region       = "eu-west-2"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
