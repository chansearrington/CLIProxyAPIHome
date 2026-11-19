#!/usr/bin/env python3
"""Operator-run installer: dedicated persistent Unraid cron, shadow only."""
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


def install_cron(directory, runner):
    a.require(directory.is_dir(), "dynamix_cron_directory_missing")
    path = directory / "auto979.cron"
    line = "*/5 * * * * /bin/bash " + shlex.quote(str(runner)) + " > /dev/null 2>&1\n"
    path.write_text("# Decision 979 shadow-only job\n" + line)
    a.require(path.read_text().splitlines()[-1] + "\n" == line, "schedule_readback_failed")


def main():
    os.umask(0o077)
    try:
        a.require(os.geteuid() == 0, "root_required")
        a.load_config(a.JOB / "979.json")
        a.private_dir(a.JOB)
        destination = a.JOB / "auto979"
        a.private_dir(destination)
        for name in ("auto979.py", "run-shadow.sh", "prepare.py", "selftest.py"):
            path = destination / name
            shutil.copyfile(SOURCE / name, path)
            path.chmod(0o700)
        runner = (destination / "run-shadow.sh").read_text()
        a.require("--shadow" in runner and "--apply" not in runner, "runner_not_shadow")
        install_cron(Path("/boot/config/plugins/dynamix"), destination / "run-shadow.sh")
        # Unraid ships update_cron with "#/bin/bash" (no "!"), so exec it through bash.
        subprocess.run(["/bin/bash", "/usr/local/sbin/update_cron"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        print(json.dumps({"installed": "shadow_only", "schedule": "*/5 * * * *", "home_writes": 0}))
        return 0
    except Exception as error:
        print(json.dumps({"reason": str(error) if isinstance(error, a.Unsafe) else "install_" + type(error).__name__}))
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
