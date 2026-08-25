#!/bin/sh
set -eu
read_secret() {
  value_var="$1"; file_var="$2"
  eval "value=\${$value_var:-}"
  eval "file=\${$file_var:-}"
  if [ -n "$file" ]; then
    [ -f "$file" ] || { echo "$file_var points to missing file" >&2; exit 1; }
    value="$(cat "$file")"
  fi
  [ -n "$value" ] || { echo "$value_var or $file_var required" >&2; exit 1; }
  printf '%s' "$value"
}
sentinel_pw="$(read_secret SENTINEL_MQTT_PASSWORD SENTINEL_MQTT_PASSWORD_FILE)"
frigate_pw="$(read_secret FRIGATE_MQTT_PASSWORD FRIGATE_MQTT_PASSWORD_FILE)"
ha_pw="$(read_secret HOMEASSISTANT_MQTT_PASSWORD HOMEASSISTANT_MQTT_PASSWORD_FILE)"
out="${1:-/mosquitto/runtime/passwords}"
tmp="${out}.tmp"
mkdir -p "$(dirname "$out")"
rm -f "$tmp"
mosquitto_passwd -b -c "$tmp" sentinel "$sentinel_pw"
mosquitto_passwd -b "$tmp" frigate "$frigate_pw"
mosquitto_passwd -b "$tmp" homeassistant "$ha_pw"
unset sentinel_pw frigate_pw ha_pw
chmod 0600 "$tmp"
mv "$tmp" "$out"
