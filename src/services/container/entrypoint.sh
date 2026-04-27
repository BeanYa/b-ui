#!/bin/sh

DB_FOLDER="${BUI_DB_FOLDER:-${SUI_DB_FOLDER:-/app/db}}"
DB_PATH="${DB_FOLDER}/b-ui.db"
LEGACY_DB_PATH="${DB_FOLDER}/s-ui.db"
BINARY_PATH="${BINARY_PATH:-./b-ui}"

if [ ! -x "$BINARY_PATH" ] && [ -x "./sui" ]; then
	BINARY_PATH="./sui"
fi

if [ -f "$DB_PATH" ] || [ -f "$LEGACY_DB_PATH" ]; then
	"$BINARY_PATH" migrate
fi

exec "$BINARY_PATH"
