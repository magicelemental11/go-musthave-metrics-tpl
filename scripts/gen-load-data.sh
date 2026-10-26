#!/bin/bash
set -e
out=${1:-load}
mkdir -p "$out"

{
	printf '['
	for i in $(seq 0 29); do
		[ "$i" -gt 0 ] && printf ','
		if [ $((i % 5)) -eq 0 ]; then
			printf '{"id":"counter%d","type":"counter","delta":%d}' "$i" "$i"
		else
			printf '{"id":"gauge%d","type":"gauge","value":%d.5}' "$i" "$i"
		fi
	done
	printf ']'
} >"$out/batch.json"

gzip -c "$out/batch.json" >"$out/batch.json.gz"
printf '{"id":"gauge1","type":"gauge"}' >"$out/value.json"
