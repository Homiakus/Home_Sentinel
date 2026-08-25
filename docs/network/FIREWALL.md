# Home Sentinel network/firewall policy

## Scope

Home Sentinel generates a dedicated `table inet home_sentinel`. It deliberately **does not** replace the host's global `input` policy. This prevents an installer from accidentally locking out SSH/VPN administration.

## Trust model

- **trusted LAN/VPN** may reach Sentinel `tcp/8080` and WebRTC `tcp+udp/8555`;
- untrusted/WAN sources are rejected from those Home Sentinel-owned published ports;
- MQTT `1883`, go2rtc API `1984`, Ollama `11434` and Frigate internal API `5000` must not be host-published at all;
- the Docker `camera_egress` subnet can optionally be restricted to configured camera CIDRs only;
- camera devices should be denied direct WAN egress at the router/VLAN firewall. The host cannot enforce that rule for traffic that never traverses it.

## Generate and validate

```sh
go run ./cmd/sentinel-firewall -policy deploy/firewall/policy.example.json -out /tmp/home-sentinel.nft -matrix /tmp/home-sentinel-matrix.json
sudo nft -c -f /tmp/home-sentinel.nft
sudo deploy/firewall/apply-nftables.sh /tmp/home-sentinel.nft
```

Always keep an out-of-band/console path when changing a remote host firewall. The apply script changes only the `home_sentinel` table.

## Router/VLAN matrix

At the router enforce:

| Source | Destination | Policy |
|---|---|---|
| Camera VLAN | Home Sentinel/Frigate server | allow required RTSP/ONVIF/NTP only |
| Camera VLAN | Internet | deny |
| Camera VLAN | trusted clients | deny |
| trusted LAN/VPN | Sentinel UI/WebRTC | allow |
| WAN | Sentinel/MQTT/go2rtc/Ollama/Frigate internal ports | deny |

Run an `nmap`/connection-matrix test from trusted, camera and untrusted segments before production acceptance.
