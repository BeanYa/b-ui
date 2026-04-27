#!/bin/sh

DB_FOLDER="${BUI_DB_FOLDER:-/app/db}"
DB_PATH="${DB_FOLDER}/b-ui.db"
LEGACY_DB_PATH="${DB_FOLDER}/s-ui.db"
if [ -f "$DB_PATH" ] || [ -f "$LEGACY_DB_PATH" ]; then
	./b-ui migrate
fi

exec ./b-ui
