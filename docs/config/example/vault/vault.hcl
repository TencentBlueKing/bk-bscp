ui = false
api_addr = "http://0.0.0.0:8200"
disable_mlock = true

storage "mysql"{
  address  = "[mysql url]"
  username = "[user]"
  password = "[password]"
  database = "vault"
  ha_enabled = true
  plaintext_connection_allowed = true
}

listener "tcp"{
  address = "0.0.0.0:8200"

  tls_disable = true
}