#!/bin/sh
# Upgrade a CPA Home fleet node's cli-proxy-api binary in place, or roll back automatically if
# the new binary never comes ready or its banner doesn't match (runbook: cpa-model-currency.md
# "Upgrading the node binary"). Parameterised (WS-547 T1) from the one-off 7.2.159->7.3.16 script
# that ran on all 5 nodes 2026-09-24 (PR #743) -- same steps, same order, same safety posture:
# pre version print, live-sha check (now also the idempotent no-op check), staged-sha check,
# .bak-exists abort, cp -p backup + verify, chmod 0700 + mv install, RESTART_AT capture,
# `launchctl kickstart -k`, healthz poll, banner grep, launchctl print, manifest update. The only
# behavioural addition is the not-ready/bad-banner -> rollback branch and its own manifest record.
#
# Expects the verified candidate binary already staged at NEW_PATH (default
# $HOME/cli-proxy-api.$TO_VERSION.new) -- this script does not download or verify the release
# tarball itself (that is cpa-release-check's job, WS-547 T2).
#
# Runs on the node via `ssh <box> 'sh -s' < upgrade-node-core.sh` with leading `VAR=value` env
# assignments -- POSIX sh only (macOS /bin/sh), one self-contained file, no bash-isms.
#
# Required env (abort exit 2 if any is missing/empty):
#   FROM_VERSION FROM_SHA256 FROM_SIZE TO_VERSION TO_SHA256 TO_SIZE TO_COMMIT
# Optional env (defaults match the 2026-09-24 rollout):
#   SERVICE_ROOT   default $HOME/.openclaw/services/cpa-home-node
#   LABEL          default ai.openclaw.cpa-home-node
#   HEALTH_URL     default http://127.0.0.1:18319/healthz
#   NEW_PATH       default $HOME/cli-proxy-api.$TO_VERSION.new
#   READY_TIMEOUT_S default 90 (bound for BOTH the forward poll and the rollback poll)
#   POLL_INTERVAL_S default 2  (not part of the plan's required list; exists only so the test
#                    suite can shrink the wait -- default reproduces the original ~2s cadence)
#   SOURCE, REASON  free-text strings recorded in the manifest; sane defaults if unset
#   LAUNCHCTL, CURL default launchctl / curl -- override ONLY so tests can inject fakes; never
#                    override on a real node
#   ROLLBACK_ONLY  default 0 (WS-547 T3). When "1", skip the entire normal upgrade flow below and
#                    go straight to the ALREADY-TESTED do_rollback() function -- for a caller (the
#                    laptop-canary real completion check in cpa-rollout.mjs) that must undo an
#                    upgrade THIS SCRIPT has already reported RESULT=UPGRADED for, because an
#                    external post-hoc check (e.g. a real inference call) failed. Still requires
#                    every FROM_*/TO_* var above -- a live-sha-must-equal-TO_SHA256 guard (reason
#                    token rollback_only_live_not_to_version on mismatch) means this mode can only
#                    ever undo the SPECIFIC upgrade named by that exact FROM/TO pair, never an
#                    arbitrary node state. Reuses do_rollback()'s own RESULT=/exit-code contract
#                    unchanged (ROLLED_BACK/3, ROLLBACK_NOT_READY/5, ROLLBACK_UNSAFE/4) rather than
#                    a second implementation of rollback mechanics.
#   ROLLBACK_TRIGGER free-text reason token recorded in the manifest's attempt reason when
#                    ROLLBACK_ONLY=1; default external_completion_check_failed.
#
# Every exit path prints exactly one line matching `^RESULT=`, always the last line of output:
#   RESULT=ALREADY_CURRENT node_version=<TO_VERSION>                          exit 0
#   RESULT=ABORTED reason=<token>                                            exit 2  (pre-flight)
#   RESULT=UPGRADED node_version=<TO_VERSION> restart_at=<ts>                exit 0
#   RESULT=UPGRADED_MANIFEST_FAILED node_version=<TO_VERSION> restart_at=<ts> exit 6
#   RESULT=ROLLED_BACK node_version=<FROM_VERSION> restart_at=<ts>           exit 3
#   RESULT=ROLLBACK_UNSAFE reason=<token>                                    exit 4  (see below)
#   RESULT=ROLLBACK_NOT_READY node_version=<FROM_VERSION> restart_at=<ts>    exit 5
#   RESULT=INTERNAL_ERROR reason=unexpected_exit exit_code=<n>               exit 1  (trap backstop)
#
# Alongside every RESTART_AT=<ISO> line this script also prints RESTART_AT_LEDGER=<space form>
# (`YYYY-MM-DD HH:MM:SS`, same captured instant) -- the Home membership/ledger SQL compares
# against the space form (runbook: cpa-model-currency.md "Ledger timestamps use a SPACE").
#
# UPGRADED_MANIFEST_FAILED means the binary swap AND the post-restart verification (healthz +
# banner) both succeeded, but the install-manifest.json write itself failed afterwards (e.g. a
# read-only manifest). The node is genuinely running TO_VERSION -- this does NOT roll back (the
# binary is proven healthy; rolling back a working node because bookkeeping failed would be
# worse) -- it just means nobody was told via the manifest. Needs a human to fix write access and
# re-run the manifest update by hand.
#
# ROLLBACK-UNSAFE means the retained `.bak` could not be verified against FROM_SHA256 (or the
# restore copy itself failed to verify) -- this script never installs an unverified artifact, so
# it leaves the just-installed TO_VERSION binary running as-is and says the node needs a human.
#
# INTERNAL_ERROR is a backstop only: an EXIT trap prints it if the script is about to terminate
# without having gone through any of the named RESULT paths above (e.g. an unexpected non-zero
# exit from a command this script doesn't explicitly handle elsewhere). It should never fire in
# practice -- if it does, treat it like ROLLBACK_UNSAFE/UPGRADED_MANIFEST_FAILED: a human needs
# to look at the node directly, its on-disk state is not fully characterised by this script.
#
# A pre-existing `.bak` for FROM_VERSION no longer unconditionally aborts: if its sha256 already
# matches FROM_SHA256 (the common shape of a retry after a prior rollback of the SAME upgrade
# attempt) it is reused as-is; only a genuinely mismatched `.bak` aborts (reason=backup_exists_mismatch).
#
# The service is NEVER unloaded/stopped/bootout -- only `launchctl kickstart -k`, in place, both
# on the way up and (if needed) on the way back down.
set -eu

# ---------------------------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------------------------

RESULT_PRINTED=0

emit_result() {
  # $1 = full RESULT= line content, without the leading "RESULT=". Every RESULT-line exit path
  # in this script goes through here so the EXIT trap below can tell "printed" from "didn't".
  echo "RESULT=$1"
  RESULT_PRINTED=1
}

# shellcheck disable=SC2329,SC2317  # invoked indirectly via 'trap on_exit EXIT' below (SC2329 local shellcheck version, SC2317 CI's -- suppress both per instruments-lie-catalog)
on_exit() {
  # EXIT trap backstop (MUST-2): if the script is about to terminate and no RESULT= line has
  # been printed yet, something failed in a way this script didn't explicitly handle -- print a
  # RESULT line anyway rather than leaving the caller with nothing.
  ec=$?
  if [ "$RESULT_PRINTED" != "1" ]; then
    echo "RESULT=INTERNAL_ERROR reason=unexpected_exit exit_code=$ec"
    exit 1
  fi
}
trap on_exit EXIT

abort() {
  # $1 = reason token (no spaces -- stays a single grep-able field)
  echo "ABORT: $1" >&2
  emit_result "ABORTED reason=$1"
  exit 2
}

require() {
  # $1 = var name (for the message), $2 = its current value
  if [ -z "$2" ]; then
    echo "ABORT: missing required $1" >&2
    emit_result "ABORTED reason=missing_param:$1"
    exit 2
  fi
}

sha256_of() {
  # $1 = path; prints the sha256 hex digest to stdout
  shasum -a 256 "$1" | cut -d' ' -f1
}

poll_ready() {
  # Polls HEALTH_URL every POLL_INTERVAL_S up to READY_TIMEOUT_S total. Returns 0 if ready was
  # seen, 1 on timeout. Sets POLL_ELAPSED_S either way.
  i=0
  iters=$((READY_TIMEOUT_S / POLL_INTERVAL_S))
  [ "$iters" -ge 1 ] || iters=1
  while [ "$i" -lt "$iters" ]; do
    sleep "$POLL_INTERVAL_S"
    i=$((i + 1))
    if "$CURL" -s -m 2 "$HEALTH_URL" 2>/dev/null | grep -q '"ready":true'; then
      POLL_ELAPSED_S=$((i * POLL_INTERVAL_S))
      return 0
    fi
  done
  POLL_ELAPSED_S=$((i * POLL_INTERVAL_S))
  return 1
}

write_manifest() {
  # $1 = MODE (upgrade|attempt). Reads FROM_*/TO_*/SOURCE/REASON (exported globally) plus
  # MANIFEST_* (set by the caller just before this call) from the environment.
  MANIFEST_MODE="$1"
  MANIFEST_PATH="$R/install-manifest.json"
  MANIFEST_BACKUP_PATH="$R/cli-proxy-api.$FROM_VERSION.bak"
  export MANIFEST_MODE MANIFEST_PATH MANIFEST_BACKUP_PATH MANIFEST_TS MANIFEST_OUTCOME MANIFEST_ATTEMPT_REASON
  python3 <<'PYEOF'
import json, os, sys

mode = os.environ["MANIFEST_MODE"]
path = os.environ["MANIFEST_PATH"]
m = json.load(open(path))

common = {
    "from_version": os.environ["FROM_VERSION"],
    "from_sha256": os.environ["FROM_SHA256"],
    "from_size": int(os.environ["FROM_SIZE"]),
    "to_version": os.environ["TO_VERSION"],
    "to_sha256": os.environ["TO_SHA256"],
    "to_commit": os.environ["TO_COMMIT"],
    "source": os.environ["SOURCE"],
    "reason": os.environ["REASON"],
}

if mode == "upgrade":
    entry = dict(common)
    entry["upgraded_at"] = os.environ["MANIFEST_TS"]
    entry["backup_path"] = os.environ["MANIFEST_BACKUP_PATH"]
    m["core_upgrade_history"] = m.get("core_upgrade_history", []) + (
        [m["core_upgrade"]] if "core_upgrade" in m else []
    )
    m["core_upgrade"] = entry
    m["core_version"] = os.environ["TO_VERSION"]
    m["core_sha256"] = os.environ["TO_SHA256"]
    m["core_size"] = int(os.environ["TO_SIZE"])
elif mode == "attempt":
    entry = dict(common)
    entry["attempted_at"] = os.environ["MANIFEST_TS"]
    entry["outcome"] = os.environ["MANIFEST_OUTCOME"]
    entry["attempt_reason"] = os.environ["MANIFEST_ATTEMPT_REASON"]
    m["core_upgrade_attempts"] = m.get("core_upgrade_attempts", []) + [entry]
else:
    sys.exit("write_manifest: unknown MODE " + mode)

json.dump(m, open(path, "w"), indent=2)
print("manifest updated (%s)" % mode)
PYEOF
}

rollback_unsafe() {
  # $1 = reason token. Never restores an unverified .bak; leaves the new binary running.
  echo "ROLLBACK-UNSAFE: $1 -- leaving the newly-installed $TO_VERSION binary running as-is;" >&2
  echo "this node needs a human (verify/replace the .bak, then re-run by hand)." >&2
  MANIFEST_TS="$RESTART_AT"
  MANIFEST_OUTCOME="rollback_unsafe"
  MANIFEST_ATTEMPT_REASON="$1"
  write_manifest attempt || true
  emit_result "ROLLBACK_UNSAFE reason=$1"
  exit 4
}

do_rollback() {
  # $1 = trigger reason token (not_ready_timeout | banner_mismatch | kickstart_failed). Always
  # exits (3 or 5), or diverts into rollback_unsafe (4) if the backup can't be trusted.
  trigger="$1"
  bak="cli-proxy-api.$FROM_VERSION.bak"

  [ -f "$bak" ] || rollback_unsafe "backup_missing"
  bak_sha=$(sha256_of "$bak")
  [ "$bak_sha" = "$FROM_SHA256" ] || rollback_unsafe "backup_sha_mismatch"

  echo "== rollback: restoring $FROM_VERSION from backup (trigger: $trigger)"
  cp -p "$bak" cli-proxy-api
  chmod 0700 cli-proxy-api
  restore_sha=$(sha256_of cli-proxy-api)
  [ "$restore_sha" = "$FROM_SHA256" ] || rollback_unsafe "restore_copy_verify_failed"

  RESTART_EPOCH=$(date -u +%s)
  RESTART_AT=$(date -u -r "$RESTART_EPOCH" '+%Y-%m-%dT%H:%M:%SZ')
  RESTART_AT_LEDGER=$(date -u -r "$RESTART_EPOCH" '+%Y-%m-%d %H:%M:%S')
  echo "RESTART_AT=$RESTART_AT (rollback restart)"
  echo "RESTART_AT_LEDGER=$RESTART_AT_LEDGER"
  if ! "$LAUNCHCTL" kickstart -k "gui/$(id -u)/$LABEL"; then
    # MUST-2: a kickstart failure here previously died silently under set -e (no RESULT, no
    # further handling). It has the same practical effect as a not-ready poll (the FROM binary
    # is on disk but not confirmed running) -- report it via the same contract.
    echo "== rollback: kickstart failed" >&2
    MANIFEST_TS="$RESTART_AT"
    MANIFEST_OUTCOME="rollback_not_ready"
    MANIFEST_ATTEMPT_REASON="${trigger}+rollback_kickstart_failed"
    write_manifest attempt || true
    emit_result "ROLLBACK_NOT_READY node_version=$FROM_VERSION restart_at=$RESTART_AT"
    exit 5
  fi

  if poll_ready; then
    echo "== rollback verify (after ${POLL_ELAPSED_S}s): ready"
    MANIFEST_TS="$RESTART_AT"
    MANIFEST_OUTCOME="rolled_back"
    MANIFEST_ATTEMPT_REASON="$trigger"
    write_manifest attempt || true
    emit_result "ROLLED_BACK node_version=$FROM_VERSION restart_at=$RESTART_AT"
    exit 3
  else
    echo "== rollback verify (after ${POLL_ELAPSED_S}s): NOT ready"
    MANIFEST_TS="$RESTART_AT"
    MANIFEST_OUTCOME="rollback_not_ready"
    MANIFEST_ATTEMPT_REASON="$trigger"
    write_manifest attempt || true
    emit_result "ROLLBACK_NOT_READY node_version=$FROM_VERSION restart_at=$RESTART_AT"
    exit 5
  fi
}

# ---------------------------------------------------------------------------------------------
# validate + default
# ---------------------------------------------------------------------------------------------

require FROM_VERSION "${FROM_VERSION:-}"
require FROM_SHA256 "${FROM_SHA256:-}"
require FROM_SIZE "${FROM_SIZE:-}"
require TO_VERSION "${TO_VERSION:-}"
require TO_SHA256 "${TO_SHA256:-}"
require TO_SIZE "${TO_SIZE:-}"
require TO_COMMIT "${TO_COMMIT:-}"

: "${SERVICE_ROOT:=$HOME/.openclaw/services/cpa-home-node}"
: "${LABEL:=ai.openclaw.cpa-home-node}"
: "${HEALTH_URL:=http://127.0.0.1:18319/healthz}"
: "${NEW_PATH:=$HOME/cli-proxy-api.$TO_VERSION.new}"
: "${READY_TIMEOUT_S:=90}"
: "${POLL_INTERVAL_S:=2}"
: "${SOURCE:=github.com/router-for-me/CLIProxyAPI release v$TO_VERSION, verified against upstream checksums.txt}"
: "${REASON:=core upgrade from $FROM_VERSION to $TO_VERSION}"
: "${LAUNCHCTL:=launchctl}"
: "${CURL:=curl}"
: "${ROLLBACK_ONLY:=0}"
: "${ROLLBACK_TRIGGER:=external_completion_check_failed}"

export FROM_VERSION FROM_SHA256 FROM_SIZE TO_VERSION TO_SHA256 TO_SIZE TO_COMMIT SOURCE REASON
MANIFEST_TS=""
MANIFEST_OUTCOME=""
MANIFEST_ATTEMPT_REASON=""

R="$SERVICE_ROOT"
[ -d "$R" ] || abort service_root_missing
cd "$R"

# ---------------------------------------------------------------------------------------------
# ROLLBACK_ONLY mode (WS-547 T3) -- undo a SPECIFIC already-completed upgrade whose EXTERNAL
# completion check (something outside this script) failed after this script already returned
# RESULT=UPGRADED. Deliberately skips every normal-flow step (staged-sha check, .bak reuse/
# create, cp+chmod+mv install) and calls the existing, already-tested do_rollback() directly --
# one implementation of rollback mechanics, not two. The live-sha-must-equal-TO_SHA256 guard is
# load-bearing: it is what stops this mode being pointed at the wrong FROM/TO pair and undoing a
# DIFFERENT upgrade than the caller means to undo.
# ---------------------------------------------------------------------------------------------
if [ "$ROLLBACK_ONLY" = "1" ]; then
  [ -f cli-proxy-api ] || abort rollback_only_live_missing
  live_sha=$(sha256_of cli-proxy-api)
  [ "$live_sha" = "$TO_SHA256" ] || abort rollback_only_live_not_to_version
  # do_rollback()'s rollback_unsafe() path (backup_missing/backup_sha_mismatch, both possible
  # before do_rollback() computes its own RESTART_AT) reads the global $RESTART_AT. In the normal
  # upgrade flow that is always already set by "step 4 restart in place" (this invocation's own
  # forward restart) before do_rollback() is ever reached. ROLLBACK_ONLY skips that whole flow --
  # there is no forward restart in THIS process, and the original upgrade's restart time isn't
  # threaded through -- so without this line `set -u` kills the script with "unbound variable" the
  # instant an unsafe .bak is hit (found live, WS-547 T3 case (r)). "Now" is the best available
  # value for "when this rollback attempt happened".
  RESTART_AT=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
  do_rollback "$ROLLBACK_TRIGGER"
  # do_rollback() always exits (3, 4, or 5) -- unreachable in practice; kept so a future edit that
  # breaks that invariant fails loudly via the EXIT trap's INTERNAL_ERROR backstop instead of
  # silently falling through into the normal upgrade flow below.
  # shellcheck disable=SC2317  # deliberately unreachable -- see comment above (instruments-lie-catalog: shellcheck-code-differs-by-version)
  exit 1
fi

# ---------------------------------------------------------------------------------------------
# step: pre version print
# ---------------------------------------------------------------------------------------------

echo "== pre"
[ -f cli-proxy-api ] && ./cli-proxy-api --version 2>/dev/null | head -1 || true

# ---------------------------------------------------------------------------------------------
# step: live-sha check (also the idempotent no-op check)
# ---------------------------------------------------------------------------------------------

[ -f cli-proxy-api ] || abort live_binary_missing
live_sha=$(sha256_of cli-proxy-api)
if [ "$live_sha" = "$TO_SHA256" ]; then
  # SHOULD-2 (coordinator review round 2): NEW_PATH can still be sitting here -- an orchestrator
  # that placed the staged binary unconditionally before this run (or one still running an older,
  # pre-round-2 build) leaves it behind, since this branch exits before the staged-sha check below
  # ever runs. Clean it up now that we've verified it's exactly the binary we'd have installed --
  # never remove anything else, and never remove it on a sha mismatch (an unverified/wrong-version
  # file here is worth investigating, not silently discarding).
  if [ -f "$NEW_PATH" ] && [ "$(sha256_of "$NEW_PATH")" = "$TO_SHA256" ]; then
    echo "== already current: removing verified, unused staged binary at $NEW_PATH"
    rm -f "$NEW_PATH"
  fi
  emit_result "ALREADY_CURRENT node_version=$TO_VERSION"
  exit 0
fi
[ "$live_sha" = "$FROM_SHA256" ] || abort live_sha_mismatch

# ---------------------------------------------------------------------------------------------
# step: staged-sha check
# ---------------------------------------------------------------------------------------------

[ -f "$NEW_PATH" ] || abort staged_missing
[ "$(sha256_of "$NEW_PATH")" = "$TO_SHA256" ] || abort staged_sha_mismatch

# ---------------------------------------------------------------------------------------------
# step: .bak reuse-or-create + verify (SHOULD-1: a pre-existing .bak matching FROM_SHA256 -- the
# normal shape of a retry after a prior rollback of this SAME attempt -- is reused rather than
# treated as a conflict; only a genuine mismatch aborts)
# ---------------------------------------------------------------------------------------------

BAK="cli-proxy-api.$FROM_VERSION.bak"
if [ -e "$BAK" ]; then
  existing_bak_sha=$(sha256_of "$BAK")
  [ "$existing_bak_sha" = "$FROM_SHA256" ] || abort backup_exists_mismatch
  echo "== step 2 backup: $BAK already present and verified -- reusing, skipping cp"
else
  echo "== step 2 backup"
  cp -p cli-proxy-api "$BAK"
  [ "$(sha256_of "$BAK")" = "$FROM_SHA256" ] || abort backup_verify_failed
  echo "backup ok"
fi

# ---------------------------------------------------------------------------------------------
# step: chmod 0700 + mv install
# ---------------------------------------------------------------------------------------------

echo "== step 3 install"
chmod 0700 "$NEW_PATH"
mv -f "$NEW_PATH" cli-proxy-api
shasum -a 256 cli-proxy-api | cut -c1-16
stat -f "%Sp %z" cli-proxy-api || true

# ---------------------------------------------------------------------------------------------
# step: RESTART_AT capture + launchctl kickstart -k
# ---------------------------------------------------------------------------------------------

echo "== step 4 restart in place"
RESTART_EPOCH=$(date -u +%s)
RESTART_AT=$(date -u -r "$RESTART_EPOCH" '+%Y-%m-%dT%H:%M:%SZ')
RESTART_AT_LEDGER=$(date -u -r "$RESTART_EPOCH" '+%Y-%m-%d %H:%M:%S')
echo "RESTART_AT=$RESTART_AT"
echo "RESTART_AT_LEDGER=$RESTART_AT_LEDGER"
# MUST-1: record stdout.log's size BEFORE this restart so the banner check below only ever
# searches bytes THIS restart wrote -- a leftover banner from an earlier attempt must not count.
PRE_KICKSTART_LOG_SIZE=$(stat -f "%z" stdout.log 2>/dev/null || echo 0)
if ! "$LAUNCHCTL" kickstart -k "gui/$(id -u)/$LABEL"; then
  # MUST-2: a kickstart failure here previously died silently under set -e (no RESULT, no
  # rollback). Treat it exactly like a not-ready poll -- attempt the rollback.
  echo "kickstart failed" >&2
  do_rollback "kickstart_failed"
fi

# ---------------------------------------------------------------------------------------------
# step: healthz poll -> not-ready is now a ROLLBACK trigger, not a shrug
# ---------------------------------------------------------------------------------------------

if poll_ready; then
  echo "== step 5 verify (after ${POLL_ELAPSED_S}s): ready"
else
  echo "== step 5 verify (after ${POLL_ELAPSED_S}s): NOT ready"
  do_rollback "not_ready_timeout"
fi
"$CURL" -s -m 3 "$HEALTH_URL" 2>/dev/null || true; echo

# ---------------------------------------------------------------------------------------------
# step: banner grep -> a missing/mismatched banner is ALSO a ROLLBACK trigger
# ---------------------------------------------------------------------------------------------

# MUST-1: scoped to only the bytes stdout.log gained since THIS restart's kickstart -- a banner
# left over from an earlier attempt at TO_VERSION must not satisfy this check.
banner_ok=0
if [ -f stdout.log ]; then
  if tail -c "+$((PRE_KICKSTART_LOG_SIZE + 1))" stdout.log 2>/dev/null | grep -q "Version: $TO_VERSION"; then
    banner_ok=1
  fi
  tail -c "+$((PRE_KICKSTART_LOG_SIZE + 1))" stdout.log 2>/dev/null | grep -c "Version: $TO_VERSION" | sed 's/^/banner lines (since restart): /' || true
fi
if [ "$banner_ok" != "1" ]; then
  do_rollback "banner_mismatch"
fi

# ---------------------------------------------------------------------------------------------
# step: launchctl print (informational)
# ---------------------------------------------------------------------------------------------

"$LAUNCHCTL" print "gui/$(id -u)/$LABEL" 2>/dev/null | grep -E "state = |pid = " | head -2 || true

# ---------------------------------------------------------------------------------------------
# step: manifest update (core_upgrade + core_upgrade_history)
# ---------------------------------------------------------------------------------------------

echo "== manifest"
MANIFEST_TS="$RESTART_AT"
if write_manifest upgrade; then
  emit_result "UPGRADED node_version=$TO_VERSION restart_at=$RESTART_AT"
  exit 0
else
  # MUST-2: the binary swap AND readiness/banner verification already succeeded -- this is NOT a
  # rollback trigger. Say so plainly; a human fixes write access and re-runs the manifest step.
  echo "MANIFEST WRITE FAILED after a verified-healthy upgrade to $TO_VERSION -- the node IS" >&2
  echo "running $TO_VERSION but install-manifest.json was not updated. Fix write access to" >&2
  echo "install-manifest.json and re-run 'write_manifest upgrade' by hand (or edit it directly)." >&2
  emit_result "UPGRADED_MANIFEST_FAILED node_version=$TO_VERSION restart_at=$RESTART_AT"
  exit 6
fi
