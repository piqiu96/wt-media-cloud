name = "douyin"
timeout = "30s"

[endpoint]
scheme = {{WT_DOUYIN_API_SCHEME}}
host = {{WT_DOUYIN_API_HOST}}
port = {{WT_DOUYIN_API_PORT}}

[connection]
dial_timeout = "5s"
read_timeout = "30s"
write_timeout = "30s"
max_conns_per_host = 100
max_idle_conn_duration = "90s"
max_conn_duration = "5m"
max_conn_wait_timeout = "3s"
keep_alive = true

[retry]
attempts = 2
delay = "300ms"
max_delay = "2s"
policy = "fixed"
