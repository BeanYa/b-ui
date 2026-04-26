#!/bin/sh

DB_FOLDER="${SUI_DB_FOLDER:-/app/db}"
DB_PATH="${DB_FOLDER}/b-ui.db"
LEGACY_DB_PATH="${DB_FOLDER}/b-ui.db"
if [ -f "$DB_PATH" ] || [ -f "$LEGACY_DB_PATH" ]; then
	./sui migrate
fi

exec ./sui
