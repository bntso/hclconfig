database {
    host = "localhost"
    port = 5432
}

app {
    db_url = "postgres://${database.host}:${database.port}/mydb"
    env    = env("APP_ENV")
}
