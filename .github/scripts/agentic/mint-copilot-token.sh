#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
#
# Mint a short-lived Copilot session token from the durable token and write the
# pi auth file. We do the exchange here, deterministically, rather than letting
# pi do it: pi's own cold token exchange intermittently fails its first model
# call (the exchange succeeds and the token is persisted, but the immediate call
# connection-errors). Handing pi a freshly minted, valid session token avoids
# that path entirely and is reliable.
#
# The durable token is the only stored secret and is non-expiring. The session
# token is minted fresh every run and lives ~30 minutes (refresh_in ~25m), which
# comfortably covers a single review.
#
# Inputs (env):
#   COPILOT_DURABLE_TOKEN  the durable GitHub Copilot OAuth token (the secret)
#   PI_CODING_AGENT_DIR    directory to write auth.json into (pi reads it here)
set -euo pipefail

: "${COPILOT_DURABLE_TOKEN:?COPILOT_DURABLE_TOKEN is required}"
: "${PI_CODING_AGENT_DIR:?PI_CODING_AGENT_DIR is required}"

# Exchange the durable token for a session token. --retry covers transient
# network blips on the exchange endpoint itself.
resp="$(curl -sS --retry 3 --retry-connrefused \
  -H "Authorization: token ${COPILOT_DURABLE_TOKEN}" \
  -H "Accept: application/json" \
  https://api.github.com/copilot_internal/v2/token)"

session="$(printf '%s' "$resp" | jq -r '.token // empty')"
if [ -z "$session" ]; then
  echo "::error::Copilot token exchange returned no token (durable token revoked or policy block?)"
  exit 1
fi
# expires_at is unix seconds; pi's auth.json wants milliseconds.
expms="$(printf '%s' "$resp" | jq -r '.expires_at * 1000')"

# Keep the session token out of any log that echoes it.
echo "::add-mask::$session"

mkdir -p "$PI_CODING_AGENT_DIR"
jq -n \
  --arg r "$COPILOT_DURABLE_TOKEN" \
  --arg s "$session" \
  --argjson e "$expms" \
  '{"github-copilot":{type:"oauth",refresh:$r,access:$s,expires:$e,enterpriseUrl:"github.com"}}' \
  > "$PI_CODING_AGENT_DIR/auth.json"
chmod 600 "$PI_CODING_AGENT_DIR/auth.json"

echo "Minted Copilot session token (valid for ~25m); pi auth written to $PI_CODING_AGENT_DIR/auth.json"
