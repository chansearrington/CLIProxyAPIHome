#!/usr/bin/env python3
"""MacBook hourly runner: SSH collection, durable deduplication, hc delivery."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

ROOT = Path.home() / "Library/Application Support/cpa-model-alert"
HC = str(Path.home() / ".local/bin/hc")
SSH = ["/usr/bin/ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10",
       "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none", "-i",
       str(Path.home() / ".ssh/cronos_ark"), "root@100.110.133.6",
       "/mnt/cache/appdata/cpa-model-alert/run-check.sh"]
TARGETS = {"lead": ("CLIProxyAPIHome / 1", str(Path.home() / "GitHub/personal/CLIProxyAPIHome")),
           "orchestrator": ("Hype | Orchestrator / 1", str(Path.home() / "GitHub/hype-sports/hype"))}
DAY = 24 * 3600


def atomic_json(path, data):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as stream:
        tmp = Path(stream.name)
        try:
            json.dump(data, stream, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
            os.replace(tmp, path)
        finally:
            tmp.unlink(missing_ok=True)


def validate(result):
    if type(result["ok"]) is not bool or not isinstance(result["missing"], dict):
        raise ValueError("invalid_result")
    for catalog in (result["missing"], result.get("hidden_missing", {})):
        for model, tiers in catalog.items():
            if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}", model):
                raise ValueError("invalid_model")
            if not isinstance(tiers, list) or not tiers:
                raise ValueError("invalid_tiers")
            for tier in tiers:
                if not isinstance(tier, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9 +_-]{0,31}", tier):
                    raise ValueError("invalid_tier")
    return result


def process(state, result, now, deliver, save=lambda state: None):
    """Only accepted deliveries advance each recipient's alert timestamp."""
    validate(result)
    models = state.setdefault("models", {})
    if result["ok"]:
        for model in list(models):
            if model not in result["missing"]:
                del models[model]
        state["failures"] = 0
        state["failure_sent"] = []
    else:
        state["failures"] = state.get("failures", 0) + 1
    save(state)
    pending = []
    for model, tiers in sorted(result["missing"].items()):
        sent = models.setdefault(model, {})
        message = "Codex model missing from Home: " + model + " [" + ", ".join(sorted(set(tiers))) + "]."
        for target in TARGETS:
            if target in sent and now - sent[target] < DAY:
                continue
            if deliver(target, message):
                sent[target] = now
                save(state)
            else:
                pending.append(target)
    if state["failures"] >= 3:
        notified = state.setdefault("failure_sent", [])
        for target in TARGETS:
            if target in notified:
                continue
            if deliver(target, "Codex model alert: check failed 3 consecutive runs."):
                notified.append(target)
                save(state)
            else:
                pending.append(target)
    return sorted(set(pending))


def send(target, message):
    try:
        listing = subprocess.run([HC, "ls", "--json"], capture_output=True, timeout=20, check=True)
        where, cwd = TARGETS[target]
        matches = [t for t in json.loads(listing.stdout)["terminals"]
                   if t.get("state") == "live" and t.get("agent") == "claude-code"
                   and t.get("where") == where and t.get("cwd") == cwd]
        if len(matches) != 1:
            return False
        # Never force: hc respects unfinished drafts and refuses blocked dialogs.
        delivered = subprocess.run([HC, "send", "--json", matches[0]["address"], message],
                                   capture_output=True, timeout=150, check=True)
        return isinstance(json.loads(delivered.stdout).get("turn"), int)
    except Exception:
        return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--test-alert", action="store_true", help="Collect normally, then force a TEST through alert delivery without changing dedup history")
    args = parser.parse_args()
    ROOT.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(ROOT, 0o700)
    with (ROOT / "run.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return 0
        try:
            response = subprocess.run(SSH, capture_output=True, timeout=600)
            result = validate(json.loads(response.stdout))
            if response.returncode and result["ok"]:
                raise ValueError("remote_exit_mismatch")
        except Exception:
            result = {"ok": False, "missing": {}}
        if args.test_alert:
            if not result["ok"]:
                print('{"test_delivery": {}, "check_ok": false}')
                return 1
            results = {}
            def test_send(target, message):
                results[target] = send(target, "TEST ONLY: " + message)
                return results[target]
            process({}, {"ok": True, "missing": {"gpt-model-alert-test": ["test"]}}, time.time(), test_send)
            print(json.dumps({"test_delivery": results, "check_ok": True, "real_missing": result["missing"]}, sort_keys=True))
            return 0 if all(results.values()) else 1
        state_path = ROOT / "state.json"
        # Corrupt/unreadable state must not silently reset already-sent alerts.
        state = json.loads(state_path.read_text()) if state_path.exists() else {}
        pending = process(state, result, time.time(), send, lambda s: atomic_json(state_path, s))
        summary = {"ok": result["ok"], "missing": result["missing"],
                   "hidden_missing": result.get("hidden_missing", {}),
                   "failures": state["failures"], "delivery_pending": pending,
                   "checked_accounts": len(result.get("checked", []))}
        atomic_json(ROOT / "last-run.json", summary)
        print(json.dumps(summary, sort_keys=True))
        return 0 if result["ok"] and not pending else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception:
        # Do not print raw subprocess/HTTP/state exceptions.
        print('{"ok": false, "error": "local_state_or_io_failure"}')
        raise SystemExit(1)
