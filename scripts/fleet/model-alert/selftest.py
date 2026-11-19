#!/usr/bin/env python3
"""Small deterministic assert checks; no Home/SSH/hc/launchd calls."""
import contextlib
import copy
import io
import json
import plistlib
from pathlib import Path
import runpy
import sys
import tempfile
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
C = runpy.run_path(str(HERE / "check.py"))
A = runpy.run_path(str(HERE / "alert.py"))


def rejects(call):
    try:
        call()
    except (ValueError, KeyError, TypeError):
        return
    raise AssertionError("invalid input accepted")


def fixture_request(path, payload=None):
    if path == "/credentials":
        return {"files": [
            {"id": tier, "auth_index": tier, "provider": "codex", "email": tier + "@example.test",
             "disabled": False, "status": "error", "unavailable": True}
            for tier in ("Team", "Plus", "Pro 20x")] + [
            {"provider": "codex", "disabled": True}, {"provider": "claude"}]}
    if path == "/quota/credentials?provider=codex&limit=200&offset=0":
        return {"items": [{"credential_id": t, "plan": {"name": t}} for t in ("Team", "Plus", "Pro 20x")], "total": 3}
    if path == "/models?scope=available":
        return {"models": [{"id": "gpt-existing"}, {"id": "gpt-6.2-astra"}, {"id": "gpt-6.2-sol"}]}
    assert path == "/requests/api-call"
    assert set(payload) == {"auth_index", "method", "url", "header"}
    assert payload["method"] == "GET" and payload["url"] == C["MODELS_URL"]
    assert payload["header"] == {"Authorization": "Bearer $TOKEN$", "Accept": "application/json"}
    offered = ["gpt-existing", "gpt-6.2-astra", "gpt-6.2-sol", "gpt-new"]
    if payload["auth_index"] == "Pro 20x":
        offered += ["gpt-pro-only"]
    return {"status_code": 200, "body": json.dumps({"models": [
        {"slug": m, "visibility": "list"} for m in offered] + [
        {"slug": "gpt-reserve", "visibility": "hide", "supported_in_api": True}]})}


result = C["collect"](fixture_request)
assert result["ok"] and len(result["checked"]) == 3
assert result["missing"] == {"gpt-new": ["Plus", "Pro 20x", "Team"], "gpt-pro-only": ["Pro 20x"]}
assert all(len(a["label"]) <= 6 for a in result["checked"])
assert "@" not in json.dumps(result)
assert "gpt-6.2-sol" not in result["missing"]
assert result["hidden_missing"] == {"gpt-reserve": ["Plus", "Pro 20x", "Team"]}
assert "gpt-reserve" not in result["missing"]
assert not A["process"]({}, {"ok": True, "missing": {}, "hidden_missing": result["hidden_missing"]},
                          100, lambda t, m: (_ for _ in ()).throw(AssertionError("hidden model alerted")))
# A malformed or failed account cannot make the whole run look successful.
for failure in ({"status_code": 401, "body": "secret fixture content"},
                {"status_code": 200, "body": '{"models":[]}'},
                {"status_code": 200, "body": '{"models":[{"slug":"bad@id"}]}'},
                {"status_code": 200, "body": '{"models":[{"slug":"gpt-new","visibility":"future"}]}'},
                {"status_code": 200, "body": '{"models":[{"slug":"gpt-new"}]}'}):
    def failed_request(path, payload=None):
        if payload is not None and payload["auth_index"] == "Team":
            return failure
        return fixture_request(path, payload)
    partial = C["collect"](failed_request)
    assert not partial["ok"] and len(partial["checked"]) == 2 and partial["failed"] == ["Teamex"]
    assert "secret" not in json.dumps(partial)
rejects(lambda: C["model_ids"]({"models": []}, "slug"))
rejects(lambda: A["validate"]({"ok": True, "missing": {"gpt-inject\nmessage": ["Team"]}}))
rejects(lambda: A["validate"]({"ok": True, "missing": {"gpt-new": ["a@example.test"]}}))

sent = []

def deliver(target, text):
    sent.append((target, text))
    return True

state = {}
small = {"ok": True, "missing": {"gpt-new": ["Team", "Plus"]}}
assert not A["process"](state, small, 100, deliver)
assert len(sent) == 2 and all("gpt-new [Plus, Team]" in text for _, text in sent)
A["process"](state, small, 101, deliver)
A["process"](state, small, 100 + A["DAY"] - 1, deliver)
assert len(sent) == 2
A["process"](state, small, 100 + A["DAY"], deliver)
assert len(sent) == 4
# A new tier for the same model does not trigger extra hourly messages.
A["process"](state, {"ok": True, "missing": {"gpt-new": ["Pro 20x"]}}, 101 + A["DAY"], deliver)
assert len(sent) == 4
A["process"](state, {"ok": True, "missing": {}}, 102 + A["DAY"], deliver)
assert not state["models"]
A["process"](state, small, 103 + A["DAY"], deliver)
assert len(sent) == 6

# Retry only the unsuccessful recipient, without re-alerting the successful one.
state = {}
assert A["process"](state, small, 100, lambda t, m: t == "lead") == ["orchestrator"]
sent.clear()
A["process"](state, small, 101, deliver)
assert [target for target, _ in sent] == ["orchestrator"]
assert all("@" not in text for _, text in sent)

# One notification per failure streak; a success rearms it, not an hourly failure.
state, sent = {}, []
failed = {"ok": False, "missing": {}}
for i in range(1, 5):
    A["process"](state, failed, i, deliver)
    assert len(sent) == (0 if i < 3 else 2)
A["process"](state, {"ok": True, "missing": {}}, 5, deliver)
for i in range(6, 9):
    A["process"](state, failed, i, deliver)
assert len(sent) == 4
# Partial failure must not resolve earlier missing models.
state = {"models": {"gpt-earlier": {"lead": 100, "orchestrator": 100}}}
A["process"](state, failed, 101, deliver)
assert "gpt-earlier" in state["models"]

# Atomic state round-trip and successful delivery persisted before the next target.
with tempfile.TemporaryDirectory() as tmp:
    path = Path(tmp) / "state.json"
    snapshots = []
    def save(s):
        A["atomic_json"](path, s)
        snapshots.append(json.loads(path.read_text()))
    A["process"]({}, small, 100, deliver, save)
    assert snapshots[1]["models"]["gpt-new"] == {"lead": 100}
    assert snapshots[2]["models"]["gpt-new"] == {"lead": 100, "orchestrator": 100}
    assert path.stat().st_mode & 0o777 == 0o600

# hc targeting is rediscovered, scoped to Claude + workspace/cwd, never forced.
class Response:
    def __init__(self, value):
        self.stdout = json.dumps(value).encode()
        self.returncode = 0

where, cwd = A["TARGETS"]["lead"]
terminal = {"address": "t-test", "where": where, "cwd": cwd, "agent": "claude-code", "state": "live"}
commands = []

def fake_run(command, **kwargs):
    commands.append(command)
    return Response({"terminals": [terminal]}) if "ls" in command else Response({"turn": 99})

with patch("subprocess.run", fake_run):
    assert A["send"]("lead", "TEST")
assert commands[1] == [A["HC"], "send", "--json", "t-test", "TEST"]
for rows in ([], [terminal, copy.deepcopy(terminal)], [dict(terminal, agent="hc-shell")],
             [dict(terminal, cwd="/other")]):
    with patch("subprocess.run", return_value=Response({"terminals": rows})):
        assert not A["send"]("lead", "TEST")
# Forced TEST takes the real collection path but never changes normal dedup state.
with tempfile.TemporaryDirectory() as tmp:
    root = Path(tmp)
    baseline = '{"models":{"gpt-real":{"lead":1}}}'
    (root / "state.json").write_text(baseline)
    test_messages = []
    def test_deliver(target, text):
        test_messages.append(text)
        return True
    with patch.dict(A["main"].__globals__, {"ROOT": root, "send": test_deliver}), \
         patch("sys.argv", ["alert.py", "--test-alert"]), \
         patch("subprocess.run", return_value=Response({"ok": True, "missing": {}})) as remote, \
         contextlib.redirect_stdout(io.StringIO()) as output:
        assert A["main"]() == 0
    assert remote.call_args.args[0] == A["SSH"]
    assert len(test_messages) == 2 and all(m.startswith("TEST ONLY: ") for m in test_messages)
    assert (root / "state.json").read_text() == baseline
    assert json.loads(output.getvalue())["test_delivery"] == {"lead": True, "orchestrator": True}

# Installer builds the real plist in a throwaway home; launchctl is stubbed.
I = runpy.run_path(str(HERE / "install.py"))
with tempfile.TemporaryDirectory() as tmp:
    with patch("pathlib.Path.home", return_value=Path(tmp)), patch("sys.platform", "darwin"), \
         patch("subprocess.run") as launch, contextlib.redirect_stdout(io.StringIO()):
        I["main"]()
    job = plistlib.loads((Path(tmp) / "Library/LaunchAgents/com.cpa-home.model-alert.plist").read_bytes())
    assert job["StartInterval"] == 3600 and job["RunAtLoad"] is True
    assert job["ProgramArguments"][1] == "-I"
    assert [call.args[0][1] for call in launch.call_args_list] == ["bootout", "enable", "bootstrap"]

# Optional lead-supplied redacted samples; fixtures remain outside the repository.
if len(sys.argv) == 2:
    folder = Path(sys.argv[1])
    credentials = json.loads((folder / "credentials.json").read_text())
    quotas = json.loads((folder / "quota-credentials.json").read_text())
    models = json.loads((folder / "models-available.json").read_text())
    upstream = json.loads((folder / "upstream-codex-models.json").read_text())
    if isinstance(upstream["body"], dict):
        upstream["body"] = json.dumps(upstream["body"])
    def recorded_request(path, payload=None):
        if path == "/credentials":
            return credentials
        if path.startswith("/quota/credentials?"):
            return quotas
        if path == "/models?scope=available":
            return models
        assert path == "/requests/api-call" and payload["method"] == "GET"
        return upstream
    observed = C["collect"](recorded_request)
    assert observed["ok"] and len(observed["checked"]) == 4
    assert {a["tier"] for a in observed["checked"]} == {"promax"}
    assert observed["missing"] == {} and observed["hidden_missing"] == {"gpt-reserve": ["promax"]}
    print("PASS: redacted real fixture replay: 4 promax accounts, 100 Home models, no listed missing; hidden_missing=gpt-reserve")
print("PASS: collector contract, all tiers, visibility, malformed/partial responses, diff, 24h dedup, delivery retry, 3-failure rearm, private atomic state, safe hc target discovery")
