#!/bin/bash
# Локальный рендеринг PlantUML диаграмм в SVG.
# Требует Docker. Запускать из корня репозитория.
#
# Использование:
#   ./scripts/render-diagrams.sh
#   ./scripts/render-diagrams.sh topology      # только одна диаграмма

set -e

SRC_DIR="docs/diagrams/src"
OUTPUT_DIR="docs/diagrams/rendered"

mkdir -p "$OUTPUT_DIR"

if [ -n "$1" ]; then
  PATTERN="$SRC_DIR/$1.puml"
else
  PATTERN="$SRC_DIR/*.puml"
fi

echo "Рендеринг PlantUML диаграмм: $PATTERN -> $OUTPUT_DIR"

docker run --rm \
  -v "$(pwd):/workspace" \
  -w "/workspace" \
  plantuml/plantuml:latest \
  -svg \
  -o "/workspace/$OUTPUT_DIR" \
  $PATTERN

echo "Готово:"
ls -lh "$OUTPUT_DIR"/*.svg 2>/dev/null || echo "SVG файлы не найдены"
