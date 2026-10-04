#!/bin/sh
# Usage: third-party-notices.sh <package>, with the build's GOOS, GOARCH, CGO_ENABLED and GOEXPERIMENT
# so `go list` resolves the packages the binary links. Prints their license, notice and patent files.
set -eu

deps=$(mktemp)
seen=$(mktemp)
trap 'rm -f "$deps" "$seen"' EXIT

# emit <dir> <label> <root>: a text printed once is referenced by its first label, not repeated.
emit() {
	for f in $(find "$1" -maxdepth 1 -type f | grep -iE '/(licen[cs]e|copying|copyright|notice|patents)[^/]*$' | grep -vE '\.go$' | sort); do
		sum=$(cksum < "$f" | cut -d ' ' -f 1,2)
		first=$(awk -v s="$sum " 'index($0, s) == 1 { print substr($0, length(s) + 1); exit }' "$seen")
		if [ -n "$first" ]; then
			echo "${f#"$3"/}: same text as printed for $first"
		else
			echo "$sum $2" >> "$seen"
			echo "--- ${f#"$3"/}"
			cat "$f"
			echo
		fi
	done
}

go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}} {{$.Dir}}{{end}}{{end}}' "$1" | sort > "$deps"

echo "Third-party software linked into $1, with the license, notice and patent files of each component."
echo "Telark's own code is licensed under the Elastic License 2.0. These components stay under their own licenses."
echo
echo "=== Go $(go env GOVERSION) (standard library and runtime)"
emit "$(go env GOROOT)" Go "$(go env GOROOT)"
for mod in $(cut -d ' ' -f 1 "$deps" | uniq); do
	set -- $(awk -v m="$mod" '$1 == m { print; exit }' "$deps")
	echo
	echo "=== $1 $2"
	emit "$3" "$1" "$3"
	# Forked or vendored code below a module root can carry its own license.
	awk -v m="$mod" '$1 == m { print $4 }' "$deps" | while read -r dir; do
		while [ "${#dir}" -gt "${#3}" ]; do echo "$dir"; dir=$(dirname "$dir"); done
	done | sort -u | while read -r sub; do emit "$sub" "$1/${sub#"$3"/}" "$3"; done
done
