#!/bin/bash

if [[ -z "$API_KEY" ]]; then
  echo "API key not set"
  exit 1
fi

bearer="Authorization: Bearer $API_KEY"

cmd="$1"

function get {
  curl "https://api.replicant.space$1" \
    -H "$bearer" \
    | jq
}

case "$cmd" in
  me)
    get /v1/accounts/me
    ;;
  *)
    echo "Unknown command"
    exit 1
    ;;
esac
