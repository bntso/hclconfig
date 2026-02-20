database {
    host = "localhost"
    port = 5432

    credentials {
        username = "admin"
        password = "secret"
    }
}

app {
    conn_string = "postgres://${database.credentials.username}:${database.credentials.password}@${database.host}:${database.port}/mydb"
}
