#!/bin/sh
# check-changelog.sh checks the changelog entry files in changelog.d and CHANGELOG.md, as described in
# changelog.d/README.md. When the latest version in CHANGELOG.md has no tag yet, a release is being
# prepared, and changelog.d must hold no entry files. With a tag vX.Y.Z as argument, as in the release
# workflow, X.Y.Z must also be the latest version in CHANGELOG.md.
set -eu
cd "$(dirname "$0")/.."

status=0
fail() {
	echo "check-changelog: $*" >&2
	status=1
}

for f in changelog.d/*; do
	case $f in
	changelog.d/README.md) continue ;;
	*.md) ;;
	*)
		if [ -e "$f" ]; then
			fail "$f: entry file names end in .md"
		fi
		continue
		;;
	esac
	awk -v f="$f" '
		/^### (Added|Changed|Deprecated|Removed|Fixed|Security)$/ { section = 1; next }
		/^$/ { next }
		/^- / && section { entries++; next }
		/^  [^ ]/ && entries { next }
		{
			printf "check-changelog: %s:%d: expected a ### section heading, a \"- \" entry or a continuation line indented by two spaces\n", f, NR
			bad = 1
		}
		END {
			if (!entries) {
				printf "check-changelog: %s: no entries\n", f
				bad = 1
			}
			exit bad
		}' "$f" >&2 || status=1
done

awk '
	/^## \[Unreleased\]/ { unreleased = 1; next }
	/^## \[/ { unreleased = 0 }
	unreleased && /^(### |- )/ {
		printf "check-changelog: CHANGELOG.md:%d: entries for the next release belong in changelog.d\n", NR
		bad = 1
	}
	END { exit bad }' CHANGELOG.md >&2 || status=1

latest=$(awk -F '[][]' '/^## \[[0-9]+\.[0-9]+\.[0-9]+\] - / { print $2; exit }' CHANGELOG.md)
if [ -z "$latest" ]; then
	fail "CHANGELOG.md has no version section"
	exit 1
fi
release=
if [ $# -gt 0 ]; then
	[ "$1" = "v$latest" ] || fail "tag $1 is not v$latest, the latest version in CHANGELOG.md"
	release=$1
elif [ -z "$(git tag -l "v$latest")" ]; then
	release=v$latest
fi
if [ -n "$release" ]; then
	for f in changelog.d/*; do
		if [ -e "$f" ] && [ "$f" != changelog.d/README.md ]; then
			fail "$f: $release has no tag here, so a release is being prepared: move the entries into" \
				"CHANGELOG.md and remove the file (or run git fetch --tags if $release is tagged)"
		fi
	done
	grep -q "^\[$latest\]: " CHANGELOG.md || fail "CHANGELOG.md has no [$latest] link"
fi
exit $status
