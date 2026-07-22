# Cloud Configs

Environment-specific non-secret configuration examples belong here.

Identity bootstrap values are deployment secrets and must be injected by the runtime environment, not committed here. `WT_MEDIA_INITIAL_ADMIN_USERNAME` and `WT_MEDIA_INITIAL_ADMIN_PASSWORD` are required together and are ignored after the first user exists. The legacy `WT_MEDIA_INITIAL_TECHNICIAN_*` names remain compatibility aliases only.
