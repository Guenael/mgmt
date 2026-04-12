#!/usr/bin/env bash
# check for any mcl files that aren't properly formatted
# TODO: this is hardly exhaustive

echo running "$0"
set -o errexit
#set -o nounset
set -o pipefail

#ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && cd .. && pwd )"	# dir!
ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${ROOT}"
. test/util.sh

#F="misc/TODO.mcl"	# TODO: you can add single files like this...
find_files() {
	# TODO: improve this match if we use txtar for non-mcl things eventually
	git ls-files | grep -e '\.mcl$' -e '\.txtar$' | grep -v 'misc/TODO.mcl'
}

bad_files=$(
	#if grep -q '^  ' "$F"; then
	#	echo "$F"
	#fi
	for i in $(find_files); do
		# search for at least one leading space, to ensure we use tabs
		# if following the leading spaces we have a caret, then skip...
		# (the caret scenario is to exclude txtar tests with a textarea)
		if grep -q -E '^ +[^ ^]' "$i"; then
			echo "$i"
		fi
	done
)

if [[ -n "${bad_files}" ]]; then
	fail_test "The following mcl files are not properly formatted: ${bad_files}"
fi

# mgmt fmt -l
find_mcl_files() {
	git ls-files | grep '\.mcl$' | grep -v 'misc/TODO.mcl'
}

if [ -x "${MGMT}" ]; then
	bad_fmt_files=$(
		for i in $(find_mcl_files); do
			# use || true to prevent errexit from killing the loop
			${MGMT} fmt -l "$i" 2>/dev/null || true
		done
	)
	if [[ -n "${bad_fmt_files}" ]]; then
		fail_test "The following mcl files are not properly formatted (mgmt fmt -l): ${bad_fmt_files}"
	fi
else
	echo "WARNING: mgmt binary not found at ${MGMT}, skipping mgmt fmt check"
fi
echo 'PASS'
