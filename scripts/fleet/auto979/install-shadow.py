#!/usr/bin/env python3
"""Operator-run installer: existing Unraid User Scripts plugin, shadow only."""
import importlib.util
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess

SOURCE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("auto979", SOURCE / "auto979.py")
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)


def main():
    os.umask(0o077)
    try:
        a.require(os.geteuid() == 0, "root_required")
        plugin = Path("/boot/config/plugins/user.scripts")
        schedule_path = plugin / "schedule.json"
        a.require(schedule_path.is_file() and Path("/usr/local/emhttp/plugins/user.scripts/startCustom.php").is_file(), "user_scripts_plugin_missing")
        cfg = a.load_config(a.JOB / "979.json")
        a.require(len(cfg["shared_ids"]) == 12, "config_not_ready")
        a.private_dir(a.JOB)
        destination = a.JOB / "auto979"
        a.private_dir(destination)
        for name in ("auto979.py", "run-shadow.sh", "prepare.py", "selftest.py"):
            path = destination / name
            shutil.copyfile(SOURCE / name, path)
            path.chmod(0o700)
        runner = (destination / "run-shadow.sh").read_text()
        a.require("--shadow" in runner and "--apply" not in runner, "runner_not_shadow")
        script_dir = plugin / "scripts" / "cpa-auto979-shadow"
        script_dir.mkdir(parents=True, exist_ok=True)
        script = script_dir / "script"
        script.write_text("#!/bin/bash\n#arrayStarted=true\nexec /bin/bash " + str(destination / "run-shadow.sh") + "\n")
        (script_dir / "name").write_text("CPA decision 979 SHADOW\n")
        (script_dir / "description").write_text("GET-only priority computation every five minutes; no Home writes.\n")
        schedules = json.loads(schedule_path.read_text())
        a.require(isinstance(schedules, dict), "schedule_format")
        before = a.JOB / "979-schedule-before.json"
        if not before.exists():
            a.write_json(before, schedules)
        schedules[str(script)] = {"script": str(script), "frequency": "custom", "id": "schedulecpa-auto979-shadow", "custom": "*/5 * * * *"}
        a.write_json(schedule_path, schedules)
        runtime_schedule = Path("/tmp/user.scripts/schedule.json")
        runtime_schedule.parent.mkdir(exist_ok=True)
        a.write_json(runtime_schedule, schedules)
        cron = "# Generated cron schedule for user.scripts\n"
        for entry in schedules.values():
            if entry["frequency"] == "custom" and entry.get("custom"):
                cron += entry["custom"].strip() + " /usr/local/emhttp/plugins/user.scripts/startCustom.php " + shlex.quote(entry["script"]) + " > /dev/null 2>&1\n"
        (plugin / "customSchedule.cron").write_text(cron)
        subprocess.run(["/usr/local/sbin/update_cron"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        a.require(json.loads(schedule_path.read_text())[str(script)]["custom"] == "*/5 * * * *", "schedule_readback_failed")
        print(json.dumps({"installed": "shadow_only", "schedule": "*/5 * * * *", "home_writes": 0}))
        return 0
    except Exception as error:
        print(json.dumps({"reason": str(error) if isinstance(error, a.Unsafe) else "install_" + type(error).__name__}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
