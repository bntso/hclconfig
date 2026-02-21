var "api_host" {
  default = "api.example.com"
}

var "api_port" {
  default = 8080
}

service {
  url = "http://${var.api_host}:${var.api_port}/api"
}
