#!/bin/bash

# Export browser history to domains.csv for use with dns-bench

BROWSER=$1
OUTPUT="domains.csv"
LIMIT=1000

if [ -z "$BROWSER" ]; then
    echo "Usage: $0 <chrome|brave|safari|firefox> [output_file]"
    exit 1
fi

if [ ! -z "$2" ]; then
    OUTPUT=$2
fi

echo "Exporting $BROWSER history to $OUTPUT..."

case $BROWSER in
    chrome)
        DB_PATH="$HOME/Library/Application Support/Google/Chrome/Default/History"
        QUERY="SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT $LIMIT;"
        ;;
    brave)
        DB_PATH="$HOME/Library/Application Support/BraveSoftware/Brave-Browser/Default/History"
        QUERY="SELECT url FROM urls ORDER BY last_visit_time DESC LIMIT $LIMIT;"
        ;;
    safari)
        DB_PATH="$HOME/Library/Safari/History.db"
        QUERY="SELECT url FROM history_items ORDER BY visit_count DESC LIMIT $LIMIT;"
        ;;
    firefox)
        # Find the first default-release profile
        PROFILE=$(ls -d "$HOME/Library/Application Support/Firefox/Profiles/"*.default-release 2>/dev/null | head -n 1)
        if [ -z "$PROFILE" ]; then
             PROFILE=$(ls -d "$HOME/Library/Application Support/Firefox/Profiles/"*.default 2>/dev/null | head -n 1)
        fi
        
        if [ -z "$PROFILE" ]; then
            echo "Error: Could not find Firefox profile"
            exit 1
        fi
        DB_PATH="$PROFILE/places.sqlite"
        QUERY="SELECT url FROM moz_places ORDER BY last_visit_date DESC LIMIT $LIMIT;"
        ;;
    *)
        echo "Unknown browser: $BROWSER"
        exit 1
        ;;
esac

if [ ! -f "$DB_PATH" ]; then
    echo "Error: History file not found at $DB_PATH"
    exit 1
fi

# Copy to temp file to avoid locks
TEMP_DB=$(mktemp)
cp "$DB_PATH" "$TEMP_DB"

if [ $? -ne 0 ]; then
    echo "Error: Failed to copy history file. Check permissions."
    rm "$TEMP_DB"
    exit 1
fi

# Extract URLs, parse hostnames (simple regex), and save unique list
sqlite3 "$TEMP_DB" "$QUERY" | \
sed -E 's|https?://||' | \
sed -E 's|/.*||' | \
sort | uniq | head -n $LIMIT > "$OUTPUT"

rm "$TEMP_DB"

echo "Done! Saved $(wc -l < "$OUTPUT") domains to $OUTPUT"
