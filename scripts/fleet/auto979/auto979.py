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
    match = re.fullmatch(r"([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})(?:\.([0-9]{1,9}))?(Z|[+-][0-9]{2}:[0-9]{2})", value)
    require(match is not None, "invalid_timestamp")
    date, fraction, zone = match.groups()
    require(zone == "Z" or (int(zone[1:3]) <= 23 and int(zone[4:6]) <= 59), "invalid_timestamp")
    # Python 3.9 cannot parse Home's RFC3339Nano fractions directly.
    normalized = date + ("." + fraction[:6].ljust(6, "0") if fraction else "") + ("+00:00" if zone == "Z" else zone)
    try:
        return dt.datetime.fromisoformat(normalized).astimezone(UTC)
    except (ValueError, OverflowError):
        raise Unsafe("invalid_timestamp") from None


def iso(now):
    return now.astimezone(UTC).isoformat().replace("+00:00", "Z")


def load_config(path):
    cfg = json.loads(Path(path).read_text())
    require(set(cfg) == {"shared_ids", "reserve_id", "fallback_id", "max_state_changes", "holdouts"}, "config_keys")
    ids = cfg["shared_ids"]
    require(isinstance(ids, list) and ids and all(isinstance(i, str) and i for i in ids), "shared_ids")
    require(len(set(ids)) == len(ids) and cfg["reserve_id"] in ids and isinstance(cfg["fallback_id"], str)
            and cfg["fallback_id"] and cfg["fallback_id"] not in ids, "config_identity")
    require(type(cfg["max_state_changes"]) is int and 0 <= cfg["max_state_changes"] <= len(ids), "state_limit")
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


def management_password(path=Path("/mnt/user/appdata/cpa-home/.env")):
    # Parse one setting, never execute shell expansions or retain other values.
    with path.open() as source:
        for line in source:
            key, separator, value = line.strip().removeprefix("export ").partition("=")
            if separator and key.strip() == "MANAGEMENT_PASSWORD":
                value = value.strip()
                if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
                    value = value[1:-1]
                require(bool(value), "management_password_empty")
                return value
    raise Unsafe("management_password_missing")


class Home:
    def __init__(self):
        self.secret = management_password()
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
    require(fallback["priority"] == 1, "fallback_changed")
    require({f["priority"] for f in rows if f is not fallback} == set(range(2, len(expected) + 1)), "priority_slots_changed")
    # Only these fields are retained; never copy credential or configuration payloads.
    return sorted(({k: f[k] for k in ("id", "email", "priority", "disabled", "reauth_required")}
                   for f in rows), key=lambda f: -f["priority"])


def eligible(row):
    return not row["disabled"] and not row["reauth_required"]


def weekly(home, row, now, previous=None):
    previous = previous or {}
    detail = home.call("/quota/credentials/" + urllib.parse.quote(row["id"], safe=""))
    require(abs((now - timestamp(detail.get("generated_at"))).total_seconds()) <= 120, "quota_response_old_or_future")
    credential = detail.get("credential", {})
    require(credential.get("credential_id") == row["id"] and credential.get("provider") == "claude", "quota_identity")
    windows = detail.get("windows")
    require(isinstance(windows, list), "quota_windows_missing")
    candidates = [w for w in windows if w.get("id") == "claude-seven-day"
                  or (w.get("scope") == "account" and w.get("period_unit") == "week" and not w.get("label"))]
    require(len(candidates) <= 1, "weekly_window_ambiguous")
    w = candidates[0] if candidates else {}
    raw_reset = w.get("reset_at") or previous.get("reset")
    if not raw_reset:
        return None  # No reset has ever been seen: caller freezes this priority slot.
    assumptions = []
    if not w.get("reset_at"):
        assumptions.append("last_known_reset_from_saved_state")
    if w:
        require(w.get("id") == "claude-seven-day" and w.get("scope") == "account" and not w.get("label")
                and w.get("period_unit") == "week" and w.get("period_value") == 1
                and w.get("unit") == "percentage" and w.get("limit") == 100, "weekly_window_shape")
    else:
        assumptions.append("weekly_window_missing_using_saved_percentage")
    used = w.get("used") if w else previous.get("used")
    require(type(used) in (int, float) and math.isfinite(used) and 0 <= used <= 100, "weekly_used_invalid")
    observed = timestamp(w.get("observed_at") if w else previous.get("observed"))
    reset = timestamp(raw_reset)
    require((now - observed).total_seconds() >= -120, "weekly_observation_future")
    require(reset <= observed + dt.timedelta(days=7, minutes=2), "weekly_reset_invalid")
    collection = detail.get("collection", {})
    if collection.get("freshness") != "fresh" or (now - observed).total_seconds() > 1800:
        assumptions.append("stale_saved_percentage_used_for_grouping")
    if reset <= now:
        assumptions.append("passed_reset_assume_room_next_reset_unknown")
    return {"used": 0 if reset <= now else used, "saved_used": used,
            "reset": None if reset <= now else iso(reset), "last_reset": iso(reset),
            "observed": iso(observed), "assumptions": assumptions}


def renewal_order(rows, quotas):
    before = {r["id"]: set() for r in rows}
    for i, a in enumerate(rows):
        for b in rows[i + 1:]:
            delta = (timestamp(quotas[a["id"]]["reset"]) - timestamp(quotas[b["id"]]["reset"])).total_seconds()
            first, second = (a, b) if delta <= 120 else (b, a)
            before[second["id"]].add(first["id"])
    # Collapse only actual cycles; their members retain live order, other groups sort normally.
    reach = {key: {other for other in before if key in before[other]} for key in before}
    for pivot in before:
        for key in before:
            if pivot in reach[key]:
                reach[key] |= reach[pivot]
    groups, seen = [], set()
    for row in rows:
        key = row["id"]
        if key not in seen:
            members = [r for r in rows if r["id"] == key or (r["id"] in reach[key] and key in reach[r["id"]])]
            groups.append(members)
            seen.update(r["id"] for r in members)
            if len(members) > 1:
                for member in members:
                    notices = quotas[member["id"]]["assumptions"]
                    if "tie_cycle_kept_live_order" not in notices:
                        notices.append("tie_cycle_kept_live_order")
    result = []
    while groups:
        remaining = {r["id"] for group in groups for r in group}
        group = next(g for g in groups if all(not (before[r["id"]] & (remaining - {m["id"] for m in g})) for r in g))
        result += group
        groups.remove(group)
    return result


def compute(rows, quotas, cfg, now):
    held = {h["id"] for h in cfg["holdouts"] if timestamp(h["until"]) > now}
    active = [r for r in rows if r["id"] != cfg["fallback_id"] and eligible(r)
              and (r["id"] in quotas or r["id"] in held or r["id"] == cfg["reserve_id"])]
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


def state_of(rows, quotas, previous=None):
    previous = previous or {}
    result = {}
    for row in rows:
        key, q, old = row["id"], quotas.get(row["id"]), previous.get(row["id"], {})
        result[key] = {"eligible": eligible(row), "disabled": row["disabled"], "reauth_required": row["reauth_required"],
                       "exhausted": q["used"] == 100 if q else old.get("exhausted"),
                       "reset": q["last_reset"] if q else old.get("reset"),
                       "used": q["saved_used"] if q else old.get("used"),
                       "observed": q["observed"] if q else old.get("observed")}
    return result


def state_guard(previous, current, cfg, now):
    require(set(previous) == set(current), "state_inventory_changed")
    changed = 0
    for key, new in current.items():
        old = previous[key]
        rollover = (old["reset"] is not None and new["reset"] is not None
                    and timestamp(old["reset"]) <= now and timestamp(new["reset"]) >= timestamp(old["reset"]))
        reset_changed = ((old["reset"] is None) != (new["reset"] is None)
                         or (old["reset"] is not None and new["reset"] is not None
                             and abs((timestamp(old["reset"]) - timestamp(new["reset"])).total_seconds()) > 120))
        if (any(old[k] != new[k] for k in ("eligible", "disabled", "reauth_required"))
                or (not rollover and (old["exhausted"] != new["exhausted"] or reset_changed))):
            changed += 1
    require(changed <= cfg["max_state_changes"], "too_many_unexpected_state_changes")


def label(row):
    return {"label": row["email"].split("@")[0][:6], "id6": row["id"][:6]}


def order_view(rows, priorities):
    return [{**label(r), "priority": priorities[r["id"]]}
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


def log_event(job, event, error=False):
    path = job / "979-log-state.json"
    previous = json.loads(path.read_text()) if path.exists() else None
    now = timestamp(event["time_utc"])
    hourly = previous is None or (now - timestamp(previous["logged_at"])).total_seconds() >= 3600
    stopped = (job / "979-STOP.json").exists()
    signature = {k: event.get(k) for k in ("order", "exclusions", "notices")}
    changed = previous is None or previous["signature"] != signature or previous["stopped"] != stopped
    latched = event["reason"] == "STOP_operator_reconciliation_required"
    needed = (hourly or previous is None or not previous["stopped"]) if latched else (
        error or changed or hourly or event.get("readback_ok") is True
        or event.get("baseline_reason") == "rebaseline_after_gap")
    if needed:
        append_log(job / "979-apply.jsonl", {**event, "heartbeat": hourly and not error and not changed})
        write_json(path, {"logged_at": iso(now), "signature": signature, "stopped": stopped})


def backup(rows, directory, now):
    require(directory.is_dir() and not directory.is_symlink(), "backup_directory_missing")
    path = directory / ("auto979-config-" + now.strftime("%Y%m%dT%H%M%S%fZ") + ".json")
    # Same accounts/attributes format as targeted-backup-979; priority-only rollback.
    value = {"taken_utc": iso(now), "purpose": "decision 979 Claude priority reorder (pre-change)",
             "scope": "priorities-only", "accounts": [
                 {"id": r["id"], "provider": "claude", **label(r), "disabled": r["disabled"],
                  "attributes": {"priority": str(r["priority"])}} for r in rows]}
    write_json(path, value)
    require(json.loads(path.read_text()) == value and stat.S_IMODE(path.stat().st_mode) == 0o600, "backup_verification_failed")
    older = sorted((p for p in directory.glob("auto979-config-*.json") if p != path),
                   key=lambda p: (p.stat().st_mtime_ns, p.name), reverse=True)
    for obsolete in older[1:]:
        obsolete.unlink()
    return path.name


def execute(home, cfg, mode, job=JOB, backups=BACKUPS, clock=lambda: dt.datetime.now(UTC)):
    now = clock()
    event = {"time_utc": iso(now), "mode": mode, "reason": "", "backup": None, "readback_ok": None}
    pending = job / "979-STOP.json"
    state_path = job / "979-state.json"
    try:
        require(not pending.exists(), "STOP_operator_reconciliation_required")
        baseline = json.loads(state_path.read_text()) if state_path.exists() else None
        previous = baseline["accounts"] if baseline else {}
        rows = credentials(home, cfg)
        held = {h["id"] for h in cfg["holdouts"] if timestamp(h["until"]) > now}
        quotas, exclusions = {}, []
        for row in rows:
            if not eligible(row) or row["id"] in held | {cfg["reserve_id"], cfg["fallback_id"]}:
                continue
            quota = weekly(home, row, now, previous.get(row["id"]))
            if quota is None:
                exclusions.append({**label(row), "reason": "weekly_reset_never_known_keep_priority"})
            else:
                quotas[row["id"]] = quota
        event["exclusions"] = exclusions
        require(len(exclusions) <= 2, "too_many_accounts_without_weekly_reset")
        wanted = compute(rows, quotas, cfg, now)
        current = state_of(rows, quotas, previous)
        if baseline:
            age = (now - timestamp(baseline["observed_at"])).total_seconds()
            require(age >= -120, "baseline_timestamp_future")
            require(set(previous) == set(current), "state_inventory_changed")
            if age > 7200:
                event["baseline_reason"] = "rebaseline_after_gap"
            else:
                state_guard(previous, current, cfg, now)
        else:
            require(mode != "apply", "shadow_baseline_required")
        live = {r["id"]: r["priority"] for r in rows}
        public = {"live": order_view(rows, live), "computed": order_view(rows, wanted),
                  "weekly": [{"id6": key[:6], **q} for key, q in quotas.items()], "exclusions": exclusions}
        print(json.dumps(public, sort_keys=True))
        event["order"] = order_view(rows, wanted)
        event["weekly"] = public["weekly"]
        event["notices"] = sorted({notice for q in quotas.values() for notice in q["assumptions"]})
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
            write_json(state_path, {"observed_at": iso(now), "accounts": current})
            log_event(job, event)
            if event["readback_ok"]:
                pending.unlink()
                log_event(job, {**event, "reason": "STOP_cleared_after_verified_apply"})
        return 0
    except Exception as error:
        # Never include API bodies, exception text, or unknown fields in logs/stdout.
        event["reason"] = str(error) if isinstance(error, Unsafe) else "invalid_data_or_io_" + type(error).__name__
        if pending.exists() and event["backup"]:
            event["readback_ok"] = False
            event["reason"] = "CRITICAL_PARTIAL_OR_UNCERTAIN_APPLY_STOP_" + event["reason"]
        print(json.dumps({k: event[k] for k in ("time_utc", "mode", "reason", "readback_ok")}))
        if mode != "dry-run":
            log_event(job, event, error=True)
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
        if mode != "dry-run":
            require(os.geteuid() == 0, "root_required")
            private_dir(JOB)
            if (JOB / "979-STOP.json").exists():
                return execute(None, None, mode, job=JOB)
        cfg = load_config(args.config)
        home = Home()
        if mode == "dry-run":
            return execute(home, cfg, mode)
        with (JOB / "979.lock").open("a") as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                log_event(JOB, {"time_utc": iso(dt.datetime.now(UTC)), "reason": "another_run_active", "mode": mode}, error=True)
                return 2
            return execute(home, cfg, mode)
    except Exception as error:
        reason = str(error) if isinstance(error, Unsafe) else "setup_" + type(error).__name__
        event = {"time_utc": iso(dt.datetime.now(UTC)), "mode": mode, "reason": reason}
        print(json.dumps(event))
        if mode != "dry-run" and JOB.is_dir():
            log_event(JOB, event, error=True)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
