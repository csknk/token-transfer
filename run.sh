#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

go run . \
    --receiver 7dC3RCm5V5wHrskmznQHbsEtaxqdX8qqY5mEGJC7sfBV \
    --amount 2 \
    --network devnet
