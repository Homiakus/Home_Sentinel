#!/bin/sh
set -eu
RULES=${1:?usage: apply-nftables.sh rules.nft}
command -v nft >/dev/null 2>&1 || { echo "nft not installed" >&2; exit 1; }
nft -c -f "$RULES"
# Remove only the table owned by Home Sentinel. Ignore absence on first apply.
nft delete table inet home_sentinel 2>/dev/null || true
nft -f "$RULES"
echo "Home Sentinel nftables policy applied"
