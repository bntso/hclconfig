group = "mygroup"

instance "mytest" {
  norun    = true
  image    = "images:ubuntu/24.04"
  networks = ["web"]
  build = [
    <<-SETUPEOF
    ${myvar}
    SETUPEOF
  ]
}

myvar = <<-EOF
  export DEBIAN_FRONTEND=noninteractive
  sudo apt install -y postgresql-common
  ${mysubvar}
  EOF

mysubvar = <<-EOF
  sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
  EOF
