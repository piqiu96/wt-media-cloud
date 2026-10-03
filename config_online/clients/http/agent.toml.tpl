name = "agent"
timeout = "7s"

[endpoint]
scheme = {{WT_AGENT_API_SCHEME}}
host = {{WT_AGENT_API_HOST}}
port = {{WT_AGENT_API_PORT}}

[connection]
dial_timeout = "2s"
read_timeout = "7s"
write_timeout = "7s"
max_conns_per_host = 50
max_idle_conn_duration = "90s"
max_conn_duration = "5m"
max_conn_wait_timeout = "3s"
keep_alive = true

[retry]
attempts = 2
delay = "300ms"
max_delay = "2s"
policy = "fixed"
