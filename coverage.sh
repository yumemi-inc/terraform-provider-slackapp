#!/usr/bin/env bash
#
# coverage.sh runs the tests and prints what they cover, per package.
#
# The acceptance tests in internal/provider serve the provider from the test
# process itself, so one go test profile counts everything they reach. With
# -coverpkg=./..., every test binary lists every package's blocks, so a block
# can appear once per binary; it counts as covered when any run reached it.
#
# It writes coverage.txt, which CI uploads to Codecov. GOTESTFLAGS adds flags
# to go test, as CI does with -race. The exit status is go test's.

set -euo pipefail

cd "$(dirname "$0")"

# -count=1 turns off go test's result cache, so that every package runs and
# reports its counters.
#
# shellcheck disable=SC2086 # GOTESTFLAGS is a list of flags
go test -count=1 ${GOTESTFLAGS:-} \
    -covermode=atomic -coverpkg=./... -coverprofile=coverage.txt ./...

echo ""
echo "Statement coverage:"
awk '
    NR == 1 { next }
    {
        key = $1 " " $2
        statements[key] = $2
        if ($3 > 0) hit[key] = 1
    }
    END {
        for (key in statements) {
            split(key, loc, ":")
            pkg = loc[1]
            sub(/\/[^\/]*$/, "", pkg)
            sub(/^github\.com\/ymm-oss\/terraform-provider-slackapp\/?/, "", pkg)
            if (pkg == "") pkg = "(root)"
            all[pkg] += statements[key]
            total += statements[key]
            if (key in hit) { covered[pkg] += statements[key]; coveredTotal += statements[key] }
        }
        for (p in all) printf "  %-40s %6.1f%%  (%d/%d)\n", p, 100 * covered[p] / all[p], covered[p], all[p] | "sort"
        close("sort")
        printf "  %-40s %6.1f%%  (%d/%d)\n", "total", 100 * coveredTotal / total, coveredTotal, total
    }
' coverage.txt
