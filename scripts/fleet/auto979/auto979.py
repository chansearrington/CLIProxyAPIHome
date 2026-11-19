#!/usr/bin/env python3
"""Decision 979. No provider calls, credential downloads, or write retries."""
import argparse
import datetime as dt
import fcntl
import json
import math
import os
from pathlib import Path
import re
import stat
import tempfile
import urllib.error
import urllib.parse
import urllib.request

UTC = dt.timezone.utc
HOME = "http://100.110.133.6:8327/v8/management"
JOB = Path("/mnt/cache/appdata/cpa-home/fleet-jobs")
BACKUPS = Path("/mnt/cache/appdata/cpa-home/data/backups")


class Unsafe(Exception):
    """Only fixed, non-sensitive reason codes belong in this exception."""


def require(ok, reason):
    if not ok:
        raise Unsafe(reason)


def timestamp(value):
    require(isinstance(value, str), "missing_timestamp")
    match = re.fullmatch(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})", value)
    require(match is not None, "invalid_timestamp")
    date, fraction, zone = match.groups()
    # Python 3.9 cannot parse Home's RFC3339Nano fractions directly.
    normalized = date + ("." + fraction[:6].ljust(6, "0") if fraction else "") + ("+00:00" if zone == "Z" else zone)
    try:
        return dt.datetime.fromisoformat(normalized).astimezone(UTC)
    except ValueError:
        raise Unsafe("invalid_timestamp") from None


def iso(now):
    return now.astimezone(UTC).isoformat().replace("+00:00", "Z")


def load_config(path):
    cfg = json.loads(Path(path).read_text())
    require(set(cfg) == {"shared_ids", "reserve_id", "fallback_id", "max_state_changes", "holdouts"}, "config_keys")
    ids = cfg["shared_ids"]
    require(isinstance(ids, list) and len(ids) == 12 and all(isinstance(i, str) and i for i in ids), "shared_ids")
    require(len(set(ids)) == 12 and cfg["reserve_id"] in ids and isinstance(cfg["fallback_id"], str)
            and cfg["fallback_id"] and cfg["fallback_id"] not in ids, "config_identity")
    require(type(cfg["max_state_changes"]) is int and 0 <= cfg["max_state_changes"] <= 12, "state_limit")
    require(isinstance(cfg["holdouts"], list), "holdouts_type")
    seen = set()
    for hold in cfg["holdouts"]:
        require(isinstance(hold, dict) and set(hold) == {"id", "until"}, "holdout_keys")
        require(hold["id"] in ids and hold["id"] != cfg["reserve_id"] and hold["id"] not in seen, "holdout_identity")
        seen.add(hold["id"])
        timestamp(hold["until"])
    return cfg


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class Home:
    def __init__(self):
        self.secret = os.environ.get("MANAGEMENT_PASSWORD")
        require(bool(self.secret), "management_password_not_in_environment")
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def call(self, path, body=None):
        require(path.startswith("/") and not path.startswith("//"), "invalid_api_path")
        request = urllib.request.Request(HOME + path,
                                        data=None if body is None else json.dumps(body).encode(),
                                        method="GET" if body is None else "PATCH",
                                        headers={"Authorization": "Bearer " + self.secret,
                                                 "Content-Type": "application/json", "Cache-Control": "no-cache"})
        try:
            with self.opener.open(request, timeout=10) as response:
                require(response.status == 200, "http_status")
                result = json.load(response)
        except urllib.error.HTTPError as error:
            raise Unsafe("http_" + str(error.code)) from None
        except (OSError, ValueError):
            raise Unsafe("api_unreadable") from None
        require(isinstance(result, dict), "api_object_required")
        return result


def credentials(home, cfg):
    files = home.call("/credentials").get("files")
    require(isinstance(files, list), "credential_list_missing")
    rows = [f for f in files if f.get("provider", "").lower() == "claude"]
    expected = set(cfg["shared_ids"]) | {cfg["fallback_id"]}
    require(len(rows) == len(expected) and {f.get("id") for f in rows} == expected, "claude_inventory_changed")
    for f in rows:
        require(type(f.get("priority")) is int and type(f.get("disabled")) is bool
                and type(f.get("reauth_required")) is bool, "credential_fields_missing")
        require(isinstance(f.get("email"), str) and "@" in f["email"], "credential_identity_missing")
    require(len({f["email"].lower() for f in rows}) == len(rows), "duplicate_identity")
    fallback = next(f for f in rows if f["id"] == cfg["fallback_id"])
    require(fallback["priority"] == 1 and fallback["disabled"], "fallback_changed")
    require({f["priority"] for f in rows if f is not fallback} == set(range(2, 14)), "priority_slots_changed")
    # Only these fields are retained; never copy credential or configuration payloads.
    return sorted(({k: f[k] for k in ("id", "email", "priority", "disabled", "reauth_required")}
                   for f in rows), key=lambda f: -f["priority"])


def eligible(row):
    return not row["disabled"] and not row["reauth_required"]


def weekly(home, row, now):
    detail = home.call("/quota/credentials/" + urllib.parse.quote(row["id"], safe=""))
    require(abs((now - timestamp(detail.get("generated_at"))).total_seconds()) <= 120, "quota_response_old_or_future")
    credential = detail.get("credential", {})
    require(credential.get("credential_id") == row["id"] and credential.get("provider") == "claude", "quota_identity")
    collection = detail.get("collection", {})
    require(collection.get("freshness") in ("fresh", "stale"), "quota_never_observed")
    require((now - timestamp(collection.get("observed_at"))).total_seconds() >= -120, "quota_observation_future")
    windows = detail.get("windows")
    require(isinstance(windows, list), "quota_windows_missing")
    candidates = [w for w in windows if w.get("id") == "claude-seven-day"
                  or (w.get("scope") == "account" and w.get("period_unit") == "week" and not w.get("label"))]
    require(len(candidates) == 1, "weekly_window_ambiguous_or_missing")
    w = candidates[0]
    require(w.get("id") == "claude-seven-day" and w.get("scope") == "account" and not w.get("label")
            and w.get("period_unit") == "week" and w.get("period_value") == 1
            and w.get("unit") == "percentage" and w.get("limit") == 100, "weekly_window_shape")
    used = w.get("used")
    require(type(used) in (int, float) and math.isfinite(used) and 0 <= used <= 100, "weekly_used_invalid")
    observed = timestamp(w.get("observed_at"))
    reset = timestamp(w.get("reset_at"))
    require((now - observed).total_seconds() >= -120, "weekly_observation_future")
    require(reset <= observed + dt.timedelta(days=7, minutes=2), "weekly_reset_invalid")
    assumptions = []
    if collection["freshness"] == "stale" or (now - observed).total_seconds() > 1800:
        assumptions.append("stale_saved_percentage_used_for_grouping")
    if reset <= now:
        assumptions.append("passed_reset_assume_room_next_reset_unknown")
    return {"used": 0 if reset <= now else used, "saved_used": used,
            "reset": None if reset <= now else iso(reset), "last_reset": iso(reset),
            "observed": iso(observed), "assumptions": assumptions}


def renewal_order(rows, quotas):
    # Pairwise 2-minute ties are not transitive. Contradictory chains fail closed.
    before = {r["id"]: set() for r in rows}
    for i, a in enumerate(rows):
        for b in rows[i + 1:]:
            delta = (timestamp(quotas[a["id"]]["reset"]) - timestamp(quotas[b["id"]]["reset"])).total_seconds()
            first, second = (a, b) if delta <= 120 else (b, a)
            before[second["id"]].add(first["id"])
    result = []
    while before:
        choices = [r for r in rows if r["id"] in before and not before[r["id"]]]
        require(bool(choices), "ambiguous_two_minute_tie_chain")
        row = choices[0]
        result.append(row)
        del before[row["id"]]
        for predecessors in before.values():
            predecessors.discard(row["id"])
    return result


def compute(rows, quotas, cfg, now):
    active = [r for r in rows if r["id"] != cfg["fallback_id"] and eligible(r)]
    held = {h["id"] for h in cfg["holdouts"] if timestamp(h["until"]) > now}
    normal = [r for r in active if r["id"] not in held and r["id"] != cfg["reserve_id"]]
    ordered = []
    for exhausted in (False, True):
        group = [r for r in normal if (quotas[r["id"]]["used"] == 100) == exhausted]
        ordered += renewal_order([r for r in group if quotas[r["id"]]["reset"] is not None], quotas)
        ordered += [r for r in group if quotas[r["id"]]["reset"] is None]
    ordered += [r for r in active if r["id"] in held]
    ordered += [r for r in active if r["id"] == cfg["reserve_id"]]
    # Reuse only eligible slots: disabled/reauth accounts keep their exact numbers.
    wanted = {r["id"]: r["priority"] for r in rows}
    wanted.update({r["id"]: slot["priority"] for r, slot in zip(ordered, active)})
    return wanted


def state_of(rows, quotas):
    return {r["id"]: {"eligible": eligible(r), "disabled": r["disabled"], "reauth_required": r["reauth_required"],
                      "exhausted": quotas[r["id"]]["used"] == 100 if r["id"] in quotas else None,
                      "reset": quotas[r["id"]]["last_reset"] if r["id"] in quotas else None} for r in rows}


def state_guard(previous, current, cfg, now):
    require(set(previous) == set(current), "state_inventory_changed")
    changed = 0
    for key, new in current.items():
        old = previous[key]
        rollover = (old["reset"] is not None and new["reset"] is not None
                    and timestamp(old["reset"]) <= now and timestamp(new["reset"]) >= timestamp(old["reset"]))
        if any(old[k] != new[k] for k in ("eligible", "disabled", "reauth_required")) or (not rollover and old != new):
            changed += 1
    require(changed <= cfg["max_state_changes"], "too_many_unexpected_state_changes")


def order_view(rows, priorities, emails=False):
    return [{**{"label": r["email"].split("@")[0][:6], "id6": r["id"][:6], "priority": priorities[r["id"]]},
             **({"email": r["email"]} if emails else {})}
            for r in sorted(rows, key=lambda r: -priorities[r["id"]])]


def write_json(path, value):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as output:
        name = output.name
        try:
            json.dump(value, output, sort_keys=True, indent=1)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        except BaseException:
            os.unlink(name)
            raise
    os.replace(name, path)
    descriptor = os.open(str(path.parent), os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def private_dir(path):
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    require(not path.is_symlink() and path.is_dir() and path.stat().st_uid == os.geteuid(), "directory_not_private_owner")
    path.chmod(0o700)


def append_log(path, event):
    descriptor = os.open(str(path), os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, "a") as output:
        info = os.fstat(output.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and info.st_nlink == 1, "log_not_private_owner")
        os.fchmod(output.fileno(), 0o600)
        output.write(json.dumps(event, sort_keys=True) + "\n")
        output.flush()
        os.fsync(output.fileno())


def backup(rows, directory, now):
    require(directory.is_dir() and not directory.is_symlink(), "backup_directory_missing")
    path = directory / ("home-config-" + now.strftime("%Y%m%dT%H%M%S%fZ") + ".json")
    # Same accounts/attributes format as targeted-backup-979; priority-only rollback.
    value = {"taken_utc": iso(now), "purpose": "decision 979 Claude priority reorder (pre-change)",
             "scope": "priorities-only", "accounts": [
                 {"id": r["id"], "provider": "claude", "label": r["email"], "disabled": r["disabled"],
                  "attributes": {"priority": str(r["priority"])}} for r in rows]}
    write_json(path, value)
    require(json.loads(path.read_text()) == value and stat.S_IMODE(path.stat().st_mode) == 0o600, "backup_verification_failed")
    older = sorted((p for p in directory.glob("home-config-*.json") if p != path),
                   key=lambda p: (p.stat().st_mtime_ns, p.name), reverse=True)
    for obsolete in older[1:]:
        obsolete.unlink()
    return path.name


def execute(home, cfg, mode, job=JOB, backups=BACKUPS, clock=lambda: dt.datetime.now(UTC)):
    now = clock()
    event = {"time_utc": iso(now), "mode": mode, "reason": "", "backup": None, "readback_ok": None}
    pending = job / "979-STOP.json"
    log = job / "979-apply.jsonl"
    state_path = job / "979-state.json"
    try:
        require(not pending.exists(), "STOP_operator_reconciliation_required")
        rows = credentials(home, cfg)
        quotas = {r["id"]: weekly(home, r, now) for r in rows
                  if r["id"] != cfg["fallback_id"] and eligible(r)}
        wanted = compute(rows, quotas, cfg, now)
        current = state_of(rows, quotas)
        previous = json.loads(state_path.read_text()) if state_path.exists() else None
        if previous is not None:
            state_guard(previous, current, cfg, now)
        else:
            require(mode != "apply", "shadow_baseline_required")
        live = {r["id"]: r["priority"] for r in rows}
        public = {"live": order_view(rows, live), "computed": order_view(rows, wanted),
                  "weekly": [{"id6": key[:6], **q} for key, q in quotas.items()]}
        print(json.dumps(public, sort_keys=True))
        event["order"] = order_view(rows, wanted, emails=True)
        event["weekly"] = public["weekly"]
        event["reason"] = "order_unchanged" if wanted == live else "weekly_renewal_order_differs"
        if mode == "apply" and wanted != live:
            require(credentials(home, cfg) == rows, "credentials_changed_during_plan")
            wall = clock()
            require(0 <= (wall - now).total_seconds() <= 120, "plan_expired")
            for quota in quotas.values():
                require(quota["reset"] is None or wall < timestamp(quota["reset"]), "reset_crossed_during_plan")
            require(all(not (now < timestamp(h["until"]) <= wall) for h in cfg["holdouts"]), "holdout_expired_during_plan")
            event["backup"] = backup(rows, backups, now)
            # Persist before the first PATCH: even SIGKILL/power loss forbids another write pass.
            write_json(pending, {**event, "reason": "APPLY_PENDING_operator_reconciliation_required"})
            for row in rows:
                if wanted[row["id"]] != row["priority"]:
                    result = home.call("/credentials/fields", {"id": row["id"], "priority": wanted[row["id"]]})
                    require(result.get("status") == "ok", "patch_not_acknowledged")
            expected = sorted(({**r, "priority": wanted[r["id"]]} for r in rows), key=lambda r: -r["priority"])
            require(credentials(home, cfg) == expected, "READBACK_FAILED_STOP_no_more_writes")
            event["readback_ok"] = True
            event["reason"] = "applied_weekly_renewal_order"
        if mode != "dry-run":
            write_json(state_path, current)
            append_log(log, event)
            if event["readback_ok"]:
                pending.unlink()
        return 0
    except Exception as error:
        # Never include API bodies, exception text, or unknown fields in logs/stdout.
        event["reason"] = str(error) if isinstance(error, Unsafe) else "invalid_data_or_io_" + type(error).__name__
        if pending.exists() and event["backup"]:
            event["readback_ok"] = False
            event["reason"] = "CRITICAL_PARTIAL_OR_UNCERTAIN_APPLY_STOP_" + event["reason"]
        print(json.dumps({k: event[k] for k in ("time_utc", "mode", "reason", "readback_ok")}))
        if mode != "dry-run":
            append_log(log, event)
        return 2


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--dry-run", action="store_true", help="GET only; stdout labels; no files")
    modes.add_argument("--shadow", action="store_true", help="GET only; persist private log and state")
    modes.add_argument("--apply", action="store_true", help="operator-only; never used by shadow schedule")
    args = parser.parse_args()
    mode = "apply" if args.apply else "shadow" if args.shadow else "dry-run"
    os.umask(0o077)
    try:
        cfg = load_config(args.config)
        home = Home()
        if mode == "dry-run":
            return execute(home, cfg, mode)
        require(os.geteuid() == 0, "root_required")
        private_dir(JOB)
        with (JOB / "979.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                append_log(JOB / "979-apply.jsonl", {"time_utc": iso(dt.datetime.now(UTC)), "reason": "another_run_active", "mode": mode})
                return 2
            return execute(home, cfg, mode)
    except Exception as error:
        reason = str(error) if isinstance(error, Unsafe) else "setup_" + type(error).__name__
        event = {"time_utc": iso(dt.datetime.now(UTC)), "mode": mode, "reason": reason}
        print(json.dumps(event))
        if mode != "dry-run" and JOB.is_dir():
            append_log(JOB / "979-apply.jsonl", event)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
