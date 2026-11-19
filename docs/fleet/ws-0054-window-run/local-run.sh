#!/bin/bash
# ARCHIVE of the 2026-11-19 window run. driver.sh still holds the two defective checks fixed in b5fe18d
# (footprint digest; broad-mount writer refusal). Regenerate from the current manual; never run this copy.
echo 'ARCHIVE: regenerate from the current ws-0054 manual; do not run' >&2; exit 1
# WS-0054 laptop side: runs under the four fleet-lock holds. Ark phases run detached (nohup) so an SSH drop cannot kill them.
set +e
W=/mnt/cache/appdata/cpa-home-build/ws0054-window-today
ark(){ ssh -o IdentitiesOnly=yes -o IdentityAgent=none -o ConnectTimeout=15 -i /Users/chansearrington/.ssh/cronos_ark root@100.110.133.6 "$@"; }
log(){ echo "[local $(date -u +%FT%TZ)] $*"; }
run_phase(){
  ark "cd $W && rm -f state.env.done-$1 && (nohup bash driver.sh $1 >> run-$1.log 2>&1 < /dev/null &) && echo started-$1" || return 1
  for i in $(seq 1 540); do sleep 10; ark "test -e $W/state.env.done-$1" && return 0; done
  return 1
}
log "locks held; forward phase"
run_phase forward || { log "forward phase did not finish in 90 min"; exit 1; }
for round in 1 2; do
  eval "$(ark "cat $W/state.env")"; log "NEXT=$NEXT TARGET=$ACCEPT_TARGET LABEL=$ACCEPT_LABEL"
  test "$NEXT" = NATIVE || break
  ARK_BOUNDARY="$WS/ready-$ACCEPT_LABEL.json"
  log "Annex N native acceptance ($ACCEPT_LABEL)"
# ---- Annex N laptop invocation (verbatim except the placeholder export line)
# SECOND LOCAL laptop terminal; no fleet-lock owner is replaced or exited.
set +e
set +x
export ARK_BOUNDARY ACCEPT_LABEL
if bash -eu <<'SH'
umask 077
case "$ACCEPT_LABEL" in forward|rollback) ;; *) exit 1 ;; esac
case "$ARK_BOUNDARY" in /mnt/cache/appdata/cpa-home-build/ws0054-20261118T2055Z/window-*/ready-"$ACCEPT_LABEL".json) ;; *) exit 1 ;; esac
python3 -I - <<'PY'
import hashlib,pathlib
root=pathlib.Path('/Users/chansearrington/GitHub/personal/CLIProxyAPIHome/.claude/worktrees/agent-a8df6a8a370c6fdd1/tools/fleet-worker')
for name,want in [('ws0054-acceptance.py','960967e132e2f32f977893d0ae4a244746c07753338d10ab40597212a1e6fe77'),('ws0054_acceptance_test.py','cacbaee6862e82abed3f800820fb4f9c6684e3b6caceef3913c171f55d923da4')]:
    assert hashlib.sha256((root/name).read_bytes()).hexdigest()==want,'STOP native source/test drift'
PY
LOCAL=$(mktemp -d /tmp/ws0054-native-result.XXXXXX)
if python3 -I /Users/chansearrington/GitHub/personal/CLIProxyAPIHome/.claude/worktrees/agent-a8df6a8a370c6fdd1/tools/fleet-worker/ws0054-acceptance.py "$ARK_BOUNDARY" > "$LOCAL/native-$ACCEPT_LABEL.json" 2> "$LOCAL/stderr"; then RC=0; else RC=$?; fi
printf '%s\n' "$RC" > "$LOCAL/native-$ACCEPT_LABEL.exit"
# Always deliver the real exit code and JSON, including abort; never manufacture success.
scp -o IdentitiesOnly=yes -o IdentityAgent=none -i /Users/chansearrington/.ssh/cronos_ark "$LOCAL/native-$ACCEPT_LABEL.json" "$LOCAL/native-$ACCEPT_LABEL.exit" "root@100.110.133.6:${ARK_BOUNDARY%/*}/"
printf '%s\n' "native helper exit=$RC; result delivered. Return to original Ark shell section6b; its gate controls NEXT. Local sanitized proof kept at $LOCAL."
test "$RC" = 0
SH
then printf '%s\n' 'Helper finished; Ark must still verify native, preservation, full panel and backup.'
else printf '%s\n' 'STOP local helper/transport: original Ark NEXT is not advanced. Run section6b to latch RECOVERY; retain ownership.'; fi
  log "finish phase"
  run_phase finish || { log "finish phase did not finish in 90 min"; exit 1; }
done
eval "$(ark "cat $W/state.env")"; log "FINAL NEXT=$NEXT TARGET=$ACCEPT_TARGET LABEL=$ACCEPT_LABEL"
test "$NEXT" = CLOSEOUT
