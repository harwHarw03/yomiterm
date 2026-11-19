#!/usr/bin/env bash
set -euo pipefail

DEST_DIR="${1:-dicts}"
mkdir -p "$DEST_DIR"

echo "Downloading Yomitan-compatible JMdict and KANJIDIC archives into $DEST_DIR/..."

curl -fL --progress-bar \
  -o "$DEST_DIR/JMdict_english_with_examples.zip" \
  "https://github.com/yomidevs/jmdict-yomitan/releases/latest/download/JMdict_english_with_examples.zip"

curl -fL --progress-bar \
  -o "$DEST_DIR/KANJIDIC_english.zip" \
  "https://github.com/yomidevs/jmdict-yomitan/releases/latest/download/KANJIDIC_english.zip"

echo ""
echo "Done! Import them with:"
echo "  yomiterm import $DEST_DIR/JMdict_english_with_examples.zip"
echo "  yomiterm import $DEST_DIR/KANJIDIC_english.zip"
