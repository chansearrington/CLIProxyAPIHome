#!/usr/bin/env python3
"""Synthetic-data checks only; no network, sleeps, or production paths."""
import contextlib
import copy
import datetime as dt
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import sys
import tempfile

spec = importlib.util.spec_from_file_location("auto979", Path(__file__).with_name("auto979.py"))
a = importlib.util.module_from_spec(spec)
sys.modules["auto979"] = a
spec.loader.exec_module(a)
NOW = a.timestamp("2030-01-01T12:00:00Z")


def fixture():
    rows = [{"id": "fake-%02d" % n, "email": "sample%02d@example.test" % n, "priority": 14 - n,
             "provider": "claude", "disabled": False, "reauth_required": False} for n in range(1, 14)]
    rows[-1]["disabled"] = True
    cfg = {"shared_ids": [r["id"] for r in rows[:-1]], "reserve_id": rows[-2]["id"],
           "fallback_id": rows[-1]["id"], "holdouts": [], "max_state_changes": 3}
    details = {}
    for n, row in enumerate(rows[:-1], 1):
        window = {"id": "claude-seven-day", "scope": "account", "period_unit": "week", "period_value": 1,
                  "unit": "percentage", "limit": 100, "used": 1, "reset_at": a.iso(NOW + dt.timedelta(hours=n)),
                  "observed_at": a.iso(NOW)}
        details[row["id"]] = {"credential": {"credential_id": row["id"], "provider": "claude"},
                               "generated_at": a.iso(NOW), "windows": [window],
                               "collection": {"status": "success", "freshness": "fresh", "observed_at": a.iso(NOW)}}
    return rows, cfg, details


class FakeHome:
    def __init__(self, rows, details):
        self.rows, self.details = copy.deepcopy(rows), copy.deepcopy(details)
        self.calls, self.writes = [], []
        self.fail_write = None
        self.fail_readback = False
        self.readback_mismatch = False
        self.concurrent_edit = False

    def call(self, path, body=None):
        self.calls.append(path)
        if path == "/credentials":
            if self.writes and self.fail_readback:
                raise a.Unsafe("fake_readback_failure")
            result = copy.deepcopy(self.rows)
            if self.readback_mismatch and self.writes:
                result[0]["priority"] = 99
            if self.concurrent_edit and self.calls.count(path) == 3:
                result[0]["disabled"] = True
            return {"files": result}
        if path == "/credentials/fields":
            assert set(body) == {"id", "priority"}
            self.writes.append(body)
            if len(self.writes) == self.fail_write:
                raise a.Unsafe("fake_patch_failure")
            next(r for r in self.rows if r["id"] == body["id"])["priority"] = body["priority"]
            return {"status": "ok"}
        assert path.startswith("/quota/credentials/") and body is None
        return copy.deepcopy(self.details[path.rsplit("/", 1)[-1]])


def read_quotas(home, rows):
    return {r["id"]: a.weekly(home, r, NOW) for r in rows if a.eligible(r)}


def ordered(rows, wanted):
    return [r["id"] for r in sorted(rows, key=lambda r: -wanted[r["id"]])]


def rejects(call, code):
    try:
        call()
    except a.Unsafe as error:
        assert str(error) == code, (str(error), code)
    else:
        raise AssertionError("did not reject " + code)


def timestamps():
    for value, micros in (("2030-01-01T12:00:00Z", 0), ("2030-01-01T12:00:00.123Z", 123000),
                          ("2030-01-01T12:00:00.123456Z", 123456), ("2030-01-01T12:00:00.123456789Z", 123456),
                          ("2030-01-01T13:00:00.123456789+01:00", 123456)):
        assert a.timestamp(value) == NOW.replace(microsecond=micros)
    for invalid in ("2030-01-01", "2030-01-01T12:00:00", "2030-13-01T12:00:00Z", "2030-01-01T12:00:00.1234567890Z"):
        rejects(lambda: a.timestamp(invalid), "invalid_timestamp")
    print("PASS RFC3339Nano, millisecond, microsecond, no-fraction, UTC offset, invalid timestamps")


def sorting():
    rows, cfg, details = fixture()
    home = FakeHome(rows, details)
    q = read_quotas(home, rows)
    wanted = a.compute(rows, q, cfg, NOW)
    assert ordered(rows, wanted) == [r["id"] for r in rows]
    # 1% is room, not exhausted. An exhausted earliest-renewal account goes behind all room.
    q["fake-01"]["used"] = 100
    wanted = a.compute(rows, q, cfg, NOW)
    assert ordered(rows, wanted) == [r["id"] for r in rows[1:11]] + ["fake-01", "fake-12", "fake-13"]
    q["fake-02"]["used"] = 100
    assert ordered(rows, a.compute(rows, q, cfg, NOW))[-4:] == ["fake-01", "fake-02", "fake-12", "fake-13"]
    # Reserve remains last even if earliest reset or exhausted.
    q["fake-12"]["reset"] = a.iso(NOW + dt.timedelta(minutes=1))
    q["fake-12"]["used"] = 100
    assert ordered(rows, a.compute(rows, q, cfg, NOW))[-2:] == ["fake-12", "fake-13"]
    # Exact 120-second tolerance preserves previous order, not raw reset order.
    q["fake-03"]["reset"] = a.iso(NOW + dt.timedelta(hours=4, seconds=120))
    order = ordered(rows, a.compute(rows, q, cfg, NOW))
    assert order.index("fake-03") < order.index("fake-04")
    q["fake-03"]["reset"] = a.iso(NOW + dt.timedelta(hours=4, seconds=121))
    order = ordered(rows, a.compute(rows, q, cfg, NOW))
    assert order.index("fake-04") < order.index("fake-03")
    cfg["holdouts"] = [{"id": "fake-05", "until": a.iso(NOW + dt.timedelta(minutes=1))}]
    assert ordered(rows, a.compute(rows, q, cfg, NOW))[-3:] == ["fake-05", "fake-12", "fake-13"]
    cfg["holdouts"][0]["until"] = a.iso(NOW)
    assert ordered(rows, a.compute(rows, q, cfg, NOW))[-3] != "fake-05"
    # Frozen slots cannot be assigned to any other account.
    rows[0]["disabled"], rows[1]["reauth_required"] = True, True
    wanted = a.compute(rows, q, cfg, NOW)
    assert wanted["fake-01"] == 13 and wanted["fake-02"] == 12 and wanted["fake-13"] == 1
    assert sorted(wanted.values()) == list(range(1, 14))
    # Conflicting pairwise tie chain is ambiguous rather than silently unstable.
    rows, cfg, details = fixture()
    q = read_quotas(FakeHome(rows, details), rows)
    for n, minutes in ((1, 4), (2, 2), (3, 0)):
        q["fake-%02d" % n]["reset"] = a.iso(NOW + dt.timedelta(hours=1, minutes=minutes))
    rejects(lambda: a.compute(rows, q, cfg, NOW), "ambiguous_two_minute_tie_chain")
    print("PASS sorting (a)-(e), exact ties, reserve, holdout, ambiguous tie chain")


def validation():
    rows, cfg, details = fixture()
    home = FakeHome(rows, details)
    row = rows[0]
    home.details[row["id"]]["collection"]["freshness"] = "stale"
    q = a.weekly(home, row, NOW)
    assert q["reset"] is not None and q["assumptions"] == ["stale_saved_percentage_used_for_grouping"]
    window = home.details[row["id"]]["windows"][0]
    window["reset_at"] = a.iso(NOW - dt.timedelta(seconds=1))
    window["used"] = 100
    q = a.weekly(home, row, NOW)
    assert q["reset"] is None and q["used"] == 0 and q["saved_used"] == 100
    quotas = read_quotas(home, rows)
    assert ordered(rows, a.compute(rows, quotas, cfg, NOW))[-3:] == ["fake-01", "fake-12", "fake-13"]
    window["reset_at"] = None
    rejects(lambda: a.weekly(home, row, NOW), "missing_timestamp")
    window["reset_at"] = a.iso(NOW + dt.timedelta(hours=1))
    window["used"] = float("nan")
    rejects(lambda: a.weekly(home, row, NOW), "weekly_used_invalid")
    window["used"] = 1
    home.details[row["id"]]["windows"].append(copy.deepcopy(window))
    rejects(lambda: a.weekly(home, row, NOW), "weekly_window_ambiguous_or_missing")
    home = FakeHome(rows, details)
    home.rows.pop()
    rejects(lambda: a.credentials(home, cfg), "claude_inventory_changed")
    home = FakeHome(rows, details)
    home.rows[-1]["disabled"] = False
    rejects(lambda: a.credentials(home, cfg), "fallback_changed")
    home = FakeHome(rows, details)
    del home.rows[0]["reauth_required"]
    rejects(lambda: a.credentials(home, cfg), "credential_fields_missing")
    old = a.state_of(rows, read_quotas(FakeHome(rows, details), rows))
    new = copy.deepcopy(old)
    for key in list(new)[:4]:
        new[key]["exhausted"] = True
    rejects(lambda: a.state_guard(old, new, cfg, NOW), "too_many_unexpected_state_changes")
    for key in list(new)[:4]:
        old[key]["reset"] = a.iso(NOW - dt.timedelta(seconds=1))
    a.state_guard(old, new, cfg, NOW)  # Expected weekly renewals do not trip the cap.
    print("PASS stale future reset, passed reset assumption, missing/ambiguous data, inventory, state cap")


def execution():
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        job, backups = root / "job", root / "backups"
        a.private_dir(job)
        backups.mkdir()
        rows, cfg, details = fixture()
        details["fake-01"]["windows"][0]["used"] = 100
        home = FakeHome(rows, details)

        def run(mode, client=home):
            with contextlib.redirect_stdout(io.StringIO()) as output:
                result = a.execute(client, cfg, mode, job, backups, clock=lambda: NOW)
            assert "@" not in output.getvalue(), "stdout exposed full identity"
            return result

        assert run("dry-run") == 0 and not home.writes and not list(job.iterdir()) and not list(backups.iterdir())
        assert run("apply") == 2 and not home.writes  # baseline absent
        assert run("shadow") == 0 and not home.writes and not list(backups.iterdir())
        assert run("apply") == 0 and len(home.writes) == 11
        assert not (job / "979-STOP.json").exists()
        saved = json.loads(next(backups.glob("*.json")).read_text())
        assert saved["scope"] == "priorities-only" and len(saved["accounts"]) == 13
        assert "secret" not in json.dumps(saved)
        for path in [job / "979-apply.jsonl", job / "979-state.json", *backups.glob("*.json")]:
            assert stat.S_IMODE(path.stat().st_mode) == 0o600
        event = json.loads((job / "979-apply.jsonl").read_text().splitlines()[-1])
        assert event["readback_ok"] is True and event["backup"] and len(event["order"]) == 13
        before = len(home.writes)
        assert run("apply") == 0 and len(home.writes) == before
        assert len(list(backups.glob("*.json"))) == 1
        for n in range(4):
            a.backup(rows, backups, NOW + dt.timedelta(seconds=n + 1))
        assert len(list(backups.glob("home-config-*.json"))) == 2
        for fail in ("patch", "readback", "mismatch", "concurrent"):
            home = FakeHome(rows, details)
            if fail == "patch":
                home.fail_write = 2
            elif fail == "readback":
                home.fail_readback = True
            elif fail == "mismatch":
                home.readback_mismatch = True
            else:
                home.concurrent_edit = True
            assert run("shadow", home) == 0
            assert run("apply", home) == 2
            writes = len(home.writes)
            if fail == "concurrent":
                assert writes == 0 and not (job / "979-STOP.json").exists()
            else:
                assert (job / "979-STOP.json").exists()
                assert run("apply", home) == 2 and len(home.writes) == writes
                event = json.loads((job / "979-apply.jsonl").read_text().splitlines()[-2])
                assert event["readback_ok"] is False and event["reason"].startswith("CRITICAL_")
                (job / "979-STOP.json").unlink()  # Test-only operator reconciliation.
        print("PASS GET-only dry-run/shadow, backup retention/modes, apply/readback, no-op, pending STOP, no retries")


def preparation():
    spec = importlib.util.spec_from_file_location("prepare979", Path(__file__).with_name("prepare.py"))
    prep = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(prep)
    rows, cfg, details = fixture()
    target = [r["email"] for r in rows[:-1]]
    source = "TARGET = " + repr(target) + '\nroute = "/v0/management/auth-files"\npatch = "/v8/management/credentials/fields"\n'
    migrated, extracted = prep.migrate(source)
    assert extracted == target and '/v0/management/' not in migrated
    assert '/v8/management/credentials"' in migrated
    assert prep.make_config(FakeHome(rows, details), extracted) == cfg
    rejects(lambda: prep.migrate(source + '\nother = "/v0/management/quota/credentials"\n'), "other_legacy_routes_remain")
    print("PASS manual script v8 migration preserves targets/PATCH and produces exact UUID config")


def replay(directory):
    """Exercise sanitized API samples supplied by the lead, never live data."""
    rows = json.loads((directory / "credentials.json").read_text())["files"]
    quota = json.loads((directory / "quota-credentials.json").read_text())
    claude = sorted([r for r in rows if r.get("provider") == "claude"], key=lambda r: -r["priority"])
    cfg = {"shared_ids": [r["id"] for r in claude[:-1]], "reserve_id": claude[-2]["id"],
           "fallback_id": claude[-1]["id"], "holdouts": [], "max_state_changes": 3}
    now = a.timestamp(quota["generated_at"])
    details = {r["credential_id"]: {"credential": r, "generated_at": quota["generated_at"],
                                    "windows": r["primary_windows"], "collection": {
                                        "freshness": r["freshness"], "observed_at": r["observed_at"]}}
               for r in quota["items"] if r["provider"] == "claude"}
    home = FakeHome(rows, details)
    with tempfile.TemporaryDirectory() as directory:
        with contextlib.redirect_stdout(io.StringIO()):
            result = a.execute(home, cfg, "dry-run", Path(directory), clock=lambda: now)
    assert result == 0 and not home.writes
    print("PASS redacted API sample replay (list primary windows used as detail fixture)")


if __name__ == "__main__":
    os.umask(0o077)
    timestamps()
    sorting()
    validation()
    execution()
    preparation()
    if len(sys.argv) == 2:
        replay(Path(sys.argv[1]))
    print("ALL CHECKS PASSED")
