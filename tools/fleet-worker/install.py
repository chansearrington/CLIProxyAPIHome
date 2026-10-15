#!/usr/bin/env python3
"""Install a node-local executor; policy and scheduling remain in CPA Home."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host',required=True)
    parser.add_argument('--agent',choices=['moxy','hyper','lara','chip'])
    args=parser.parse_args()
    source=Path(__file__).resolve().parent
    root=Path.home()/'.openclaw/services/cpa-home-fleet-worker'
    root.mkdir(parents=True,exist_ok=True,mode=0o700)
    files=['worker.py','catalog_sync.py','upgrade-node-core.sh','fleet-lock','fleet-hosts.conf']
    hashes={name:hashlib.sha256((source/name).read_bytes()).hexdigest() for name in files}
    identity=hashlib.sha256(json.dumps(hashes,sort_keys=True).encode()).hexdigest()[:16]
    bundle=root/identity
    bundle.mkdir(mode=0o700,exist_ok=True)
    for name in files:
        target=bundle/name
        shutil.copyfile(source/name,target)
        target.chmod(0o700 if name in ('fleet-lock','upgrade-node-core.sh') else 0o600)
    (bundle/'install-manifest.json').write_text(json.dumps(hashes,indent=2)+'\n')
    (bundle/'install-manifest.json').chmod(0o600)
    pending=root/('.current-'+str(os.getpid()))
    pending.symlink_to(bundle.name)
    pending.replace(root/'current')
    settings={'host':args.host,'port':8327,'certificate_dir':str(Path.home()/'.cli-proxy-api'),
              'service_root':str(Path.home()/'.openclaw/services/cpa-home-node'),
              'state_dir':str(Path.home()/'.openclaw/state/cpa-home-fleet-worker')}
    if args.agent:settings['agent']=args.agent
    settings_file=root/'settings.json'
    settings_file.write_text(json.dumps(settings,indent=2)+'\n');settings_file.chmod(0o600)
    state=Path(settings['state_dir']);state.mkdir(parents=True,exist_ok=True,mode=0o700)
    # Resolve the node's existing CLI/runtime paths once; launchd does not run a login shell.
    runtime_paths=[]
    for tool in ['openclaw','node']:
        found=subprocess.run(['/bin/zsh','-lc','command -v '+tool],capture_output=True,text=True,timeout=20)
        lines=found.stdout.strip().splitlines()
        if found.returncode==0 and lines and Path(lines[-1]).is_absolute():
            runtime_paths.append(str(Path(lines[-1]).parent.resolve()))
    runtime_paths.extend(['/opt/homebrew/bin','/usr/local/bin','/usr/bin','/bin','/usr/sbin','/sbin'])
    runtime_path=':'.join(dict.fromkeys(runtime_paths))
    label='ai.openclaw.cpa-home-fleet-worker'
    config={'Label':label,'ProgramArguments':['/usr/bin/python3','-B',str(root/'current/worker.py'),'--settings',str(settings_file)],
            'RunAtLoad':True,'StartInterval':60,'ProcessType':'Background','LowPriorityIO':True,
            'EnvironmentVariables':{'PATH':runtime_path},
            'StandardOutPath':str(state/'launchd.out.log'),'StandardErrorPath':str(state/'launchd.err.log')}
    plist=Path.home()/'Library/LaunchAgents'/(label+'.plist')
    domain='gui/'+str(os.getuid())
    subprocess.run(['launchctl','bootout',domain+'/'+label],capture_output=True)
    plist.write_bytes(plistlib.dumps(config));plist.chmod(0o600)
    subprocess.run(['launchctl','bootstrap',domain,str(plist)],check=True)
    print(json.dumps({'installed':label,'bundle':identity,'home':args.host,'agent':args.agent}))


if __name__=='__main__':
    main()
