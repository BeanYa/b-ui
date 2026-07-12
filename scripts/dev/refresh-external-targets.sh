#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
go run ./src/backend/cmd/b-ui external-targets "$@"
