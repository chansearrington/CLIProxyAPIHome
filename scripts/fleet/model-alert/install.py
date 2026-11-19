#!/usr/bin/env python3
"""Install the MacBook hourly runner; run only after the Ark wrapper is ready."""
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys

LABEL = "com.cpa-home.model-alert"


def main():
    if sys.platform != "darwin":
        raise RuntimeError("macOS_required")
    root = Path.home() / "Library/Application Support/cpa-model-alert"
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(root, 0o700)
    source = Path(__file__).resolve().with_name("alert.py")
    destination = root / "alert.py"
    if source != destination:
        temporary = root / "alert.py.new"
        shutil.copyfile(source, temporary)
        os.chmod(temporary, 0o700)
        os.replace(temporary, destination)
    plist = Path.home() / "Library/LaunchAgents" / (LABEL + ".plist")
    plist.parent.mkdir(parents=True, exist_ok=True)
    job = {"Label": LABEL, "ProgramArguments": [sys.executable, "-I", str(destination)],
           "StartInterval": 3600, "RunAtLoad": True, "ProcessType": "Background",
           "WorkingDirectory": str(root), "StandardOutPath": str(root / "runner.log"),
           "StandardErrorPath": str(root / "runner.log")}
    with plist.open("wb") as stream:
        plistlib.dump(job, stream)
    domain = "gui/" + str(os.getuid())
    subprocess.run(["/bin/launchctl", "bootout", domain + "/" + LABEL], capture_output=True)
    subprocess.run(["/bin/launchctl", "enable", domain + "/" + LABEL], check=True)
    subprocess.run(["/bin/launchctl", "bootstrap", domain, str(plist)], check=True)
    print("Installed " + LABEL + ": on load, then every 3600 seconds while the MacBook is awake.")
    print("Runner and state: " + str(root))


if __name__ == "__main__":
    main()
