#!/usr/bin/env bash
# Local development defaults shared by Cloud scripts.
#
# Local M/CHG acceptance uses one stable database. Do not create a new
# database for every task; apply schema changes to this database through
# migrations or explicit acceptance data updates.

export WT_MEDIA_MYSQL_DSN="${WT_MEDIA_MYSQL_DSN:-root:root123@tcp(127.0.0.1:3306)/wt_media_cloud?parseTime=true&multiStatements=true}"
export WT_MEDIA_SESSION_COOKIE_SECURE="${WT_MEDIA_SESSION_COOKIE_SECURE:-false}"
export WT_MEDIA_CLOUD_HTTP_ADDR="${WT_MEDIA_CLOUD_HTTP_ADDR:-127.0.0.1:18080}"
