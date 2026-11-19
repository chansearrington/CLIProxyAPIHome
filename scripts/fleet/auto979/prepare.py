#!/usr/bin/env python3
"""Prepare v8 manual script + UUID config without executing legacy source."""
import argparse
import ast
import importlib.util
import json
import os
from pathlib import Path
import sys

a = sys.modules.get("auto979")
if a is None:
    spec = importlib.util.spec_from_file_location("auto979", Path(__file__).with_name("auto979.py"))
    a = importlib.util.module_from_spec(spec)
    sys.modules["auto979"] = a
    spec.loader.exec_module(a)


def migrate(source, job=a.JOB):
    tree = ast.parse(source)
    targets = [node.value for node in tree.body if isinstance(node, ast.Assign)
               and any(isinstance(t, ast.Name) and t.id == "TARGET" for t in node.targets)]
    a.require(len(targets) == 1, "legacy_target_assignment_missing")
    target = ast.literal_eval(targets[0])
    a.require(isinstance(target, list) and target and all(isinstance(t, str) and "@" in t for t in target)
              and len(set(target)) == len(target), "legacy_target_list_invalid")
    # Only replace the list-route literal; TARGET and backup/PATCH behavior stay intact.
    edits = [node for node in ast.walk(tree) if isinstance(node, ast.Constant)
             and node.value == "/v0/management/auth-files"]
    a.require(len(edits) == 1, "legacy_list_route_unexpected")
    lines = source.splitlines(keepends=True)
    node = edits[0]
    a.require(node.lineno == node.end_lineno, "legacy_route_multiline")
    line = lines[node.lineno - 1].encode("utf-8")
    lines[node.lineno - 1] = (line[:node.col_offset] + b'"/v8/management/credentials"'
                              + line[node.end_col_offset:]).decode("utf-8")
    migrated = "".join(lines)
    a.require("/v0/management/" not in migrated, "other_legacy_routes_remain")
    guard = ('\nimport fcntl as _979_fcntl, pathlib as _979_pathlib, sys as _979_sys\n'
             'try:\n'
             '    _979_lock = open(' + repr(str(job / "979.lock")) + ', "a")\n'
             '    _979_fcntl.flock(_979_lock, _979_fcntl.LOCK_EX | _979_fcntl.LOCK_NB)\n'
             'except OSError:\n'
             '    _979_sys.exit("ABORT: auto979 lock unavailable")\n'
             'if _979_pathlib.Path(' + repr(str(job / "979-STOP.json")) + ').exists():\n'
             '    _979_sys.exit("ABORT: auto979 STOP requires operator reconciliation")\n\n')
    # Keep a shebang/encoding header first so the migrated script still runs directly.
    offset = 0
    for header in migrated.splitlines()[:2]:
        if not header.startswith("#"):
            break
        offset += 1
    for item in tree.body:
        if ((isinstance(item, ast.Expr) and isinstance(item.value, ast.Constant) and isinstance(item.value.value, str))
                or (isinstance(item, ast.ImportFrom) and item.module == "__future__")):
            offset = item.end_lineno
        else:
            break
    lines = migrated.splitlines(keepends=True)
    migrated = "".join(lines[:offset]) + guard + "".join(lines[offset:])
    compile(migrated, "apply979-v8.py", "exec")
    return migrated, target


def make_config(home, target):
    files = home.call("/credentials").get("files")
    a.require(isinstance(files, list), "credential_list_missing")
    rows = [r for r in files if r.get("provider") == "claude"]
    by_email = {r.get("email"): r for r in rows}
    a.require(len(rows) == len(target) + 1 and len(by_email) == len(rows) and all(t in by_email for t in target), "legacy_account_set_changed")
    other = [r for r in rows if r.get("email") not in target]
    a.require(len(other) == 1 and other[0].get("priority") == 1, "fallback_changed")
    cfg = {"shared_ids": [by_email[t]["id"] for t in target], "reserve_id": by_email[target[-1]]["id"],
           "fallback_id": other[0]["id"], "max_state_changes": min(3, len(target)), "holdouts": []}
    a.credentials(home, cfg)
    return cfg


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--legacy", required=True)
    parser.add_argument("--migrated-output", required=True)
    parser.add_argument("--config-output", required=True)
    args = parser.parse_args()
    os.umask(0o077)
    try:
        output, config = Path(args.migrated_output), Path(args.config_output)
        a.require(not output.exists() and not config.exists(), "outputs_already_exist")
        source = Path(args.legacy).read_text()
        migrated, target = migrate(source)
        cfg = make_config(a.Home(), target)
        with output.open("x", encoding="utf-8") as saved:
            saved.write(migrated)
        a.write_json(config, cfg)
        a.load_config(config)
        print(json.dumps({"migrated": str(output), "config": str(config), "shared_accounts": len(target),
                          "reserve": "last_TARGET_entry", "writes_to_home": 0}))
        return 0
    except Exception as error:
        print(json.dumps({"reason": str(error) if isinstance(error, a.Unsafe) else "prepare_" + type(error).__name__}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
