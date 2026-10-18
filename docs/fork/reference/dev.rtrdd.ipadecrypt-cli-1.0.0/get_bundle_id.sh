#!/bin/bash
# Extract bundle identifier from IPA Info.plist

IPA_PATH="$1"
TEMP_DIR="$2"

if [ -z "$IPA_PATH" ] || [ -z "$TEMP_DIR" ]; then
    echo "Usage: $0 <ipa_path> <temp_dir>" >&2
    exit 1
fi

# Create temp directory for extraction
EXTRACT_DIR=$(mktemp -d /tmp/ipa_extract.XXXXXX)

# Extract Info.plist from IPA
unzip -q "$IPA_PATH" 'Payload/*.app/Info.plist' -d "$EXTRACT_DIR" >/dev/null 2>&1 || true
plist=$(find "$EXTRACT_DIR/Payload" -maxdepth 2 -name Info.plist | head -n1)

if [ -z "$plist" ]; then
    echo "[ERROR] Could not find Info.plist in IPA" >&2
    rm -rf "$EXTRACT_DIR"
    exit 1
fi

# Extract CFBundleIdentifier. Different plutil builds use different CLI
# grammars (Apple's "-extract key raw -o -" vs. third-party "-key key"
# clones found on some jailbreaks), so try each and use the first that
# actually produces a value.
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
echo "$bundle_id"

# Cleanup
rm -rf "$EXTRACT_DIR"
