#!/bin/bash

echo "Checking JSON files..."
failed=false
while read -r -d '' f; do 
	if ! OUT=$(jq type "$f"); then
		echo "$OUT"
		failed=true
	fi
done < <(find . -iname '*.json' -print0)
if $failed; then
	exit 1
fi

