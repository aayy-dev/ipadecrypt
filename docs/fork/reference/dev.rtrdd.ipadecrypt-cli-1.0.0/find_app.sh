#!/bin/bash
# Find installed app by bundle identifier

BUNDLE_ID="$1"

if [ -z "$BUNDLE_ID" ]; then
    echo "Usage: $0 <bundle_id>"
    exit 1
fi

SEARCH_DIRS=(
    '/private/var/containers/Bundle/Application'
)

for dir in "${SEARCH_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        while IFS= read -r app_path; do
            plist="$app_path/Info.plist"
            if [ -f "$plist" ]; then
                # Different plutil builds use different CLI grammars (Apple's
                # "-extract key raw -o -" vs. third-party "-key key" clones
                # found on some jailbreaks), so try each and use the first
                # that actually produces a value.
                bundle_id=$(plutil -extract CFBundleIdentifier raw -o - "$plist" 2>/dev/null)
                if [ -z "$bundle_id" ]; then
                    bundle_id=$(plutil -key CFBundleIdentifier "$plist" 2>/dev/null)
                fi
                if [ -z "$bundle_id" ]; then
                    bundle_id=$(defaults read "$plist" CFBundleIdentifier 2>/dev/null)
                fi
                if [ -z "$bundle_id" ]; then
                    # Last resort: only works if the plist happens to be XML, not binary
                    bundle_id=$(grep -a -A1 'CFBundleIdentifier' "$plist" 2>/dev/null | grep -o '<string>.*</string>' | sed 's/<string>\(.*\)<\/string>/\1/' | head -n1)
                fi
                if [ "$bundle_id" = "$BUNDLE_ID" ]; then
                    echo "$app_path"
                    exit 0
                fi
            fi
        done < <(find "$dir" -name '*.app' -type d 2>/dev/null)
    fi
done
