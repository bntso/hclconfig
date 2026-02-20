service "api" {
    host = "api.example.com"
    port = 8080
}

service "web" {
    host = "web.example.com"
    port = 3000
}

app {
    api_url = "http://${service.api.host}:${service.api.port}"
    web_url = "http://${service.web.host}:${service.web.port}"
}
