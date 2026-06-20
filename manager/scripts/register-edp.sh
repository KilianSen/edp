#!/bin/sh
# Self-wires the demo stack: waits for edp + edp-manager, registers edp in the
# manager (idempotent), and seeds an auto-deploying demo env. Run by the
# `register` service in docker-compose.stack.yml.
set -eu

EDP="http://edp:8080"
MGR="http://edp-manager:9090"
PW="${ADMIN_PASSWORD:-devpass123}"

token_of() { sed -E 's/.*"token":"([^"]+)".*/\1/'; }

echo "waiting for edp + manager to come up..."
until curl -sf "$EDP/api/auth" >/dev/null 2>&1; do sleep 2; done
until curl -sf "$MGR/api/auth" >/dev/null 2>&1; do sleep 2; done

EDP_TOKEN=$(curl -s -X POST "$EDP/api/login" -H 'Content-Type: application/json' -d "{\"password\":\"$PW\"}" | token_of)
MGR_TOKEN=$(curl -s -X POST "$MGR/api/login" -H 'Content-Type: application/json' -d "{\"password\":\"$PW\"}" | token_of)
if [ -z "$EDP_TOKEN" ] || [ -z "$MGR_TOKEN" ]; then
  echo "failed to obtain tokens (check ADMIN_PASSWORD)"; exit 1
fi

# Register edp in the manager (skip if the label already exists).
if curl -s "$MGR/api/instances" -H "Authorization: Bearer $MGR_TOKEN" | grep -q '"label":"local-edp"'; then
  echo "instance 'local-edp' already registered"
else
  curl -s -X POST "$MGR/api/instances" -H "Authorization: Bearer $MGR_TOKEN" -H 'Content-Type: application/json' \
    -d '{"label":"local-edp","base_url":"http://edp:8080","api_token":"'"$EDP_TOKEN"'"}' >/dev/null
  echo "registered 'local-edp'"
fi

# Seed an auto-deploying demo env so the dashboard isn't empty and Redeploy
# actually pulls + runs a container (edp has the Docker socket).
if curl -s "$EDP/api/environments" -H "Authorization: Bearer $EDP_TOKEN" | grep -q '"id":'; then
  echo "edp already has environments; not seeding"
else
  curl -s -X POST "$EDP/api/environments" -H "Authorization: Bearer $EDP_TOKEN" -H 'Content-Type: application/json' \
    -d '{"name":"demo-nginx","source_type":"registry","deploy_type":"container","registry_image":"nginx:alpine","proxy_port":"80","auto_deploy":true}' >/dev/null
  echo "seeded auto-deploying env 'demo-nginx'"
fi

echo "stack wired: open http://localhost:9090"
