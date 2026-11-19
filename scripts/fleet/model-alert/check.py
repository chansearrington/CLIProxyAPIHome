#!/usr/bin/env python3
"""Ark-side read-only collector. Home retains every provider token."""
import json
import os
import re
import sys
import urllib.parse
import urllib.request

HOME = "http://100.110.133.6:8327/v8/management"
# Measured successful manual discovery request, 2026-11-17; not Home's catalog URL.
MODELS_URL = "https://chatgpt.com/backend-api/codex/models?client_version=0.167.0"
MODEL_ID = re.compile(r"[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}\Z")
TIER = re.compile(r"[A-Za-z0-9][A-Za-z0-9 +_-]{0,31}\Z")


def model_ids(payload, key):
    rows = payload["models"]
    if not isinstance(rows, list) or not rows:
        raise ValueError("empty_models")
    result = set()
    for row in rows:
        value = row[key]
        if not isinstance(value, str) or not MODEL_ID.fullmatch(value):
            raise ValueError("invalid_model_id")
        result.add(value)
    return result


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("unexpected_redirect")


def home_request():
    password = os.environ["MANAGEMENT_PASSWORD"]
    if not password:
        raise ValueError("missing_management_auth")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(path, payload=None):
        data = None if payload is None else json.dumps(payload).encode()
        req = urllib.request.Request(HOME + path, data=data, headers={
            "Authorization": "Bearer " + password, "Content-Type": "application/json"})
        with opener.open(req, timeout=75) as response:
            raw = response.read(8 * 1024 * 1024 + 1)
        if len(raw) > 8 * 1024 * 1024:
            raise ValueError("response_too_large")
        return json.loads(raw)

    return request


def collect(request):
    """Injectable request(path, optional_json_body) keeps tests off live Home."""
    credentials = request("/credentials")["files"]
    accounts = [a for a in credentials if a.get("provider", a.get("type")) == "codex"
                and not a.get("disabled", False)]
    if not accounts:
        raise ValueError("no_enabled_codex_credentials")
    plans = {}
    offset = 0
    while True:
        page = request("/quota/credentials?provider=codex&limit=200&offset=" + str(offset))
        items = page["items"]
        for item in items:
            plans[item["credential_id"]] = (item.get("plan") or {}).get("name")
        offset += len(items)
        if offset >= page["total"]:
            break
        if not items:
            raise ValueError("incomplete_quota_page")
    available = model_ids(request("/models?scope=available"), "id")
    missing, hidden_missing, checked, failed = {}, {}, [], []
    for account in accounts:
        # Only this short label leaves Ark; never return names, IDs, or response bodies.
        label = re.sub(r"[^A-Za-z0-9]", "", str(account.get("email") or account["id"]))[:6]
        try:
            tier = plans[account["id"]]
            if not isinstance(tier, str) or not TIER.fullmatch(tier):
                raise ValueError("unknown_tier")
            response = request("/requests/api-call", {
                "auth_index": account["auth_index"], "method": "GET", "url": MODELS_URL,
                "header": {"Authorization": "Bearer $TOKEN$", "Accept": "application/json"}})
            if response["status_code"] != 200:
                raise ValueError("provider_http_failure")
            catalog = json.loads(response["body"])
            offered = model_ids(catalog, "slug")
            for row in catalog["models"]:
                visibility = row["visibility"]
                if visibility not in ("list", "hide"):
                    raise ValueError("unknown_visibility")
            # Upstream explicitly hides reserve/internal models from its own picker.
            for row in catalog["models"]:
                if row["slug"] not in available:
                    target = missing if row["visibility"] == "list" else hidden_missing
                    target.setdefault(row["slug"], set()).add(tier)
            checked.append({"label": label, "tier": tier, "offered_count": len(offered)})
        except Exception:
            # Never serialize exceptions: provider errors can contain account/token data.
            failed.append(label)
    return {"ok": not failed, "missing": {m: sorted(t) for m, t in sorted(missing.items())},
            "hidden_missing": {m: sorted(t) for m, t in sorted(hidden_missing.items())},
            "checked": checked, "failed": failed, "home_count": len(available)}


if __name__ == "__main__":
    try:
        result = collect(home_request())
    except Exception:
        result = {"ok": False, "missing": {}, "checked": [], "failed": ["collector"]}
    print(json.dumps(result, sort_keys=True))
    sys.exit(0 if result["ok"] else 1)
