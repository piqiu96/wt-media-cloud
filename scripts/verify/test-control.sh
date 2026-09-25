#!/usr/bin/env bash
# Arms for `bin/control.sh`'s *entry* properties -- not for its start/stop
# behaviour (that needs a build, a database and a port).
#
# Why this file exists: the entry is documented as a direct invocation
# (`bin/control.sh status`), and a mode that is executable on one machine can
# still be 100644 in the git index -- which is what a fresh checkout and CI
# materialise. Only the index can tell us that, and only a *direct* invocation
# can tell us whether the file runs at all: invoking it as `bash bin/control.sh`
# succeeds even when the execute bit is missing, so a check written that way
# cannot see the very defect it is meant to catch. Every arm below therefore
# invokes the entry DIRECTLY, and the four verbs are anchored at the start of a
# line (a bare `grep -F start` is satisfied by the `restart` line alone).
#
# Output discipline: every line is prefixed with `[control]`. `scripts/test.sh`
# runs this script and `scripts/verify_m3_acceptance.py` parses that script's
# log with `^ok\s`, `^FAIL`, `Test Files N passed (N)` and `Tests N passed (N)`
# -- a bare `ok`/`FAIL` prefix here would be counted as a Go package result or a
# vitest summary. The prefix keeps this script's lines out of those counts.
#
# Note on reading failure output: a non-executable entry exits **126** from a
# plain shell but **1** from inside a `set -e` script (this one). Measured
# matrix: no options / `-u` / `-o pipefail` -> 126; `-e` / `-eu` -> 1. So no arm
# here asserts a literal exit code for that case; they assert the property.
#
# usage: scripts/verify/test-control.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTROL="$ROOT_DIR/bin/control.sh"
TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/wt-media-cloud-control-test.XXXXXX")"
trap 'rm -rf "$TEMP_ROOT"' EXIT

PASSED=0
FAILED=0

pass() {
  PASSED=$((PASSED + 1))
  echo "[control] PASS $1"
}

fail() {
  FAILED=$((FAILED + 1))
  echo "[control] FAIL $1"
  echo "[control]      $2"
}

# --- existence and mode ------------------------------------------------------

if [[ -f "$CONTROL" ]]; then
  pass "bin/control.sh exists"
else
  fail "bin/control.sh exists" "not a regular file: $CONTROL"
  echo "[control] control: $PASSED passed, $FAILED failed"
  exit 1
fi

if command -v git >/dev/null 2>&1; then
  # `ls-files -s` prints "<mode> <object> <stage>\t<path>". The mode is 100755
  # for an executable blob and 100644 otherwise; it is what a fresh clone
  # materialises, so a file that is +x on disk but 100644 in the index works
  # here and breaks everywhere else.
  if ! git -C "$ROOT_DIR" ls-files --error-unmatch bin/control.sh >/dev/null 2>&1; then
    fail "bin/control.sh is tracked" "not tracked by git in $ROOT_DIR"
    index_mode=""
  else
    pass "bin/control.sh is tracked"
    index_mode="$(git -C "$ROOT_DIR" ls-files -s -- bin/control.sh | awk '{print $1}')"
    if [[ "$index_mode" == "100755" ]]; then
      pass "git index mode is 100755"
    else
      fail "git index mode is 100755" "got '$index_mode' -- a fresh checkout would not be executable"
    fi
  fi
else
  # Failing loudly beats skipping: the index mode is the half of this property
  # that a working-tree check cannot see, and a silent skip would leave the
  # suite green while checking only half of what it claims to.
  fail "git is available" "git is required to read the index mode of bin/control.sh"
fi

if [[ -x "$CONTROL" ]]; then
  pass "working tree mode is executable"
else
  fail "working tree mode is executable" "$(stat -f '%Sp' "$CONTROL" 2>/dev/null || stat -c '%A' "$CONTROL")"
fi

# --- direct invocation -------------------------------------------------------

status=0
"$CONTROL" help >"$TEMP_ROOT/help.out" 2>"$TEMP_ROOT/help.err" || status=$?
if [[ "$status" -eq 0 ]]; then
  pass "direct invocation: bin/control.sh help exits 0"
else
  # No literal exit code in this message on purpose: an exec-permission-denied
  # command exits 126 from a plain shell but **1** from inside a `set -e` script
  # (measured: none/`-u`/`pipefail` -> 126, `-e`/`-eu` -> 1), and this file is
  # such a script. Asserting the property (exit 0) is what makes this arm read
  # the same on both sides of that asymmetry.
  fail "direct invocation: bin/control.sh help exits 0" \
    "got exit=$status; 'Permission denied' below means the entry is not executable: $(head -2 "$TEMP_ROOT/help.err" | tr '\n' ' ')"
fi

# One arm per verb, anchored at the start of a line and followed by a word
# boundary. A bare `grep -F start` would be satisfied by the `restart` line
# alone, so a `help` that had lost `start` would still look complete.
for verb in start stop restart status; do
  if grep -qE "^  ${verb}\\b" "$TEMP_ROOT/help.out"; then
    pass "help lists '$verb'"
  else
    fail "help lists '$verb'" "no line matching '^  $verb\\b' in: $(tr '\n' '|' <"$TEMP_ROOT/help.out")"
  fi
done

# --- unknown verb ------------------------------------------------------------

status=0
"$CONTROL" no-such-verb >"$TEMP_ROOT/bad.out" 2>"$TEMP_ROOT/bad.err" || status=$?
if [[ "$status" -eq 2 ]]; then
  pass "unknown verb exits 2"
else
  fail "unknown verb exits 2" "got exit=$status"
fi

if [[ -s "$TEMP_ROOT/bad.out" ]]; then
  fail "unknown verb writes nothing to stdout" "stdout: $(head -2 "$TEMP_ROOT/bad.out" | tr '\n' ' ')"
else
  pass "unknown verb writes nothing to stdout"
fi

if grep -qF "Usage:" "$TEMP_ROOT/bad.err"; then
  pass "unknown verb prints the usage on stderr"
else
  fail "unknown verb prints the usage on stderr" "stderr: $(head -2 "$TEMP_ROOT/bad.err" | tr '\n' ' ')"
fi

# --- summary -----------------------------------------------------------------

echo "[control] control: $PASSED passed, $FAILED failed"
[[ "$FAILED" -eq 0 ]]
