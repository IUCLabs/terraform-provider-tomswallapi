terraform {
  required_providers {
    tomswallapi = {
      source = "IUCLabs/tomswallapi"
    }
  }
}

# The token defaults to the TOMSWALLAPI_WRITE_TOKEN environment variable.
provider "tomswallapi" {
  endpoint = "https://tomswallapi.example.com"
}
