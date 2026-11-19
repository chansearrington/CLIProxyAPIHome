#!/bin/bash
# ARCHIVE of the 2026-11-19 window run. driver.sh still holds the two defective checks fixed in b5fe18d
# (footprint digest; broad-mount writer refusal). Regenerate from the current manual; never run this copy.
echo 'ARCHIVE: regenerate from the current ws-0054 manual; do not run' >&2; exit 1
# WS-0054 window driver: verbatim reviewed blocks from the manual (commit-pinned), chained by the manual's own NEXT latch.
PHASE="$1"; STATE=/mnt/cache/appdata/cpa-home-build/ws0054-window-today/state.env
# ---- core section 1 (verbatim)
set +e
set +x
umask 077
export BUILD=/mnt/cache/appdata/cpa-home-build/ws0054-20261118T2055Z
export WS="$BUILD/window-$(date -u +%Y%m%dT%H%M%SZ)"
export DATA=/mnt/cache/appdata/cpa-home/data/home.db
export COMPOSE=/mnt/cache/appdata/cpa-home/docker-compose.yml
export OLD_TAG=cpa-home:1.1.9-claude-fleet-13860b9
export OLD_IMAGE=sha256:0a4c3a7c21e139dc177d70daeb4885db2a9e163fa18de0b04e619f859927c018
export NEW_TAG=cpa-home:1.1.9-claude-fleet-7091c86
export NEW_IMAGE=sha256:43bac183f39e9324a91fe80a123b72e0bd5d5be4b103c45f3f56dda87cf35bf8
export READY_SECONDS=900 ROLLBACK_SECONDS=600
export TARGET_SCHEMA=17
export PROOF=/mnt/cache/appdata/cpa-home-build/ws0048-2d5fbf1/swap.py
export INPUT_SUMS=PENDING_REVIEWED_INPUT_CHECKSUM_FILE
NEXT=INPUTS
export INPUT_SUMS=/mnt/cache/appdata/cpa-home-build/ws0054-window-today/INPUT_SUMS.sha256
export HOT_BASE="$(cat /mnt/cache/appdata/cpa-home-build/ws0054-window-today/HOT_BASE)"
# ---- Annex V, I, N(verify_native), D(hot_backup) definitions (verbatim)
verify_home() {
python3 -I - "$@" <<'PY'
import os,sys,json,pathlib,hashlib,sqlite3,subprocess,time,urllib.error
p=pathlib.Path(os.environ['WS']);build=pathlib.Path(os.environ['BUILD']);mode=sys.argv[1]
source=pathlib.Path(os.environ['PROOF']).read_bytes()
assert hashlib.sha256(source).hexdigest()=='6f7c9b46d0f2a4c4bdf0d8e4567fa59de2aee838a09e909d63df84430252d190','STOP proof drift'
g={'__name__':'proof_library'};exec(compile(source,os.environ['PROOF'],'exec'),g)
g.update(DATA=os.environ['DATA'],BASE=p,_password=lambda:os.environ['MANAGEMENT_PASSWORD'])
def load(name): return json.loads((build/name).read_text())
def digest(x): return g['digest'](x)
def structure(c):
    rows=c.execute('select type,name,tbl_name,sql from sqlite_master order by type,name').fetchall();objects=[]
    for r in rows:
        item=dict(zip(('type','name','tbl_name','sql'),r))
        if r[0]=='index': item['index_xinfo']=[list(x) for x in c.execute('PRAGMA index_xinfo("'+r[1].replace('"','""')+'")')]
        objects.append(item)
    return objects

def schema(name):
    result=load(name);assert result['sqlite_stored_home_schema_version'].startswith('none;')
    return result['objects']
def rollback_reference(objects):
    before=build/'schema-before.json'
    assert hashlib.sha256(before.read_bytes()).hexdigest()=='4a3d6ffa2bfc00fabe1b65d774b3bd278201602b121fcfcff5e77deeae3404c5','STOP before-schema pin drift'
    baseline=schema('schema-before.json');prior={(x['type'],x['name']):x for x in baseline};current={(x['type'],x['name']):x for x in objects}
    assert len(prior)==len(baseline) and len(current)==len(objects)
    assert prior.keys()<=current.keys() and all(current[k]==v for k,v in prior.items()),'STOP rollback baseline structure changed'
    for kind,name in current.keys()-prior.keys():
        assert kind=='index' and name in clean and current[kind,name]==clean[name],'STOP unapproved rollback DDL'
    marker=p/'candidate-may-have-run';attempt=None
    if os.path.lexists(marker):
        assert marker.is_file() and not marker.is_symlink(),'STOP candidate marker unknown'
        attempt=hashlib.sha256(marker.read_bytes()).hexdigest()
    else: assert objects==baseline,'STOP no-attempt rollback differs from pinned before-schema'
    return {'old_image':os.environ['OLD_IMAGE'],'window':str(p),'candidate_attempt_sha256':attempt,'objects':objects}
def panel():
    m=load('panel-manifest.json');files=m['files']
    assert m['ok'] is True and m['source_image']==os.environ['OLD_IMAGE'] and m['file_count']==len(files)==1060
    for name,v in files.items():
        assert name and str(pathlib.PurePosixPath(name))==name and not any(x in name for x in ('\n','\r','?','#')) and not name.startswith('/') and '..' not in pathlib.PurePosixPath(name).parts and '\\' not in name
        assert set(v)=={'bytes','sha256'} and type(v['bytes']) is int and v['bytes']>=0
        assert isinstance(v['sha256'],str) and len(v['sha256'])==64 and all(c in '0123456789abcdef' for c in v['sha256'])
    assert sum(v['bytes'] for v in files.values())==m['total_bytes']
    recipe=''.join(v['sha256']+'  '+n+'\n' for n,v in sorted(files.items()))
    assert hashlib.sha256(recipe.encode()).hexdigest()==m['tree_sha256']
    served={n for n in files if n in ('index.html','management.html','user.html') or n.startswith('assets/')}
    assert set(files)-served=={'.gitkeep','panel-manifest.json'}
    assert {'index.html','management.html','user.html'}<=served
    return m,served
# Keep routing/concurrency settings; exclude only observed activity/status/timestamps.
volatile={'updated_at','modtime','status','status_message','unavailable','in_flight','admitted_in_flight','observed','limiter','remaining','total_saturated','saturated_model_count'}
def credentials():
    rows=g['api']('/credentials')['files'];out={}
    for a in rows:
        uid=a['id'];assert uid not in out and isinstance(a.get('reauth_required'),bool)
        stable={k:v for k,v in a.items() if k not in volatile}
        out[uid]={'provider':a['provider'],'disabled':bool(a['disabled']),'reauth_required':a['reauth_required'],'settings_sha256':digest(stable),'status':a.get('status'),'message_sha256':digest(a.get('status_message')),'unavailable':bool(a.get('unavailable'))}
    assert len(out)==20 and sum(a['provider']=='claude' for a in out.values())==13 and sum(a['provider']=='codex' for a in out.values())==4
    return out

def snapshot(reference=None):
    s=g['snapshot'](reference);s['accounts']=credentials();s['billing_settings']=digest(s['billing_settings'])
    c=g['db']()
    try:
        for table in ('channel_group','model_group'):
            s['stable'][table]=digest(c.execute('select * from '+table+' order by id').fetchall())
        # Inventory is21 undeleted identities; hash ALL historical rows too, without reading grants.
        rows=c.execute('SELECT uuid,id,"index",provider,label,prefix,disabled,base_url,api_key_hash,compat_name,provider_key,models_hash,created_at,deleted_at,quota_identity_version,json_extract(auth_json,\'$.proxy_url\'),json_extract(auth_json,\'$.attributes\') FROM auth ORDER BY uuid').fetchall()
        assert len({r[0] for r in rows})==len(rows)
        s['auth_db_ids']=[r[0] for r in rows if r[13] is None];assert len(s['auth_db_ids'])==21
        s['stable']['auth_identity_settings']=digest(rows)
        assert set(s['accounts'])<=set(s['auth_db_ids']),'STOP API credential outside DB inventory'
    finally:c.close()
    # Parent disabled/deleted state and all free text/keys stay behind hashes; no raw rows are saved.
    return json.loads(json.dumps(s,default=str))

m,served=panel();c=g['db']()
try:
    actual_structure=structure(c)
    required={'idx_usage_dashboard_time_v1','idx_usage_dashboard_provider_time_v1','idx_usage_service_tier_pending_v1','idx_billing_charge_base_amount_pending_v1'}
    clean={x['name']:x for x in schema('index-reference.json') if x['type']=='index'}
    assert set(clean)==required,'STOP clean-migration index reference incomplete'
    current={x['name']:x for x in actual_structure if x['type']=='index'}
    for name in required & current.keys(): assert current[name]==clean[name],'STOP existing index SQL/xinfo wrong'
    if mode not in ('preflight','baseline','rollback-schema'):
        assert mode in ('ready','stable')
        if os.environ['ACCEPT_TARGET']=='rollback':
            assert os.environ['ACCEPT_LABEL']=='rollback' and os.environ['ACCEPT_IMAGE']==os.environ['OLD_IMAGE'] and os.environ['ACCEPT_SCHEMA_FILE']=='rollback-schema.json','STOP wrong rollback target'
            frozen=p/'rollback-schema.json';assert frozen.is_file() and not frozen.is_symlink()
            assert json.loads(frozen.read_text())==rollback_reference(actual_structure),'STOP rollback differs from frozen stopped-writer schema/attempt'
        else:
            assert os.environ['ACCEPT_TARGET']==os.environ['ACCEPT_LABEL']=='forward' and os.environ['ACCEPT_IMAGE']==os.environ['NEW_IMAGE'] and os.environ['ACCEPT_SCHEMA_FILE']=='schema-after-candidate.json','STOP wrong forward target'
            assert required<=current.keys(),'STOP forward indexes incomplete'
            assert actual_structure==schema('schema-after-candidate.json'),'STOP forward schema differs'
    assert c.execute('select count(*) from home_schema_migration').fetchone()[0]==0
    if mode=='rollback-schema':
        reference=rollback_reference(actual_structure);frozen=p/'rollback-schema.json'
        if os.path.lexists(frozen):
            assert frozen.is_file() and not frozen.is_symlink() and json.loads(frozen.read_text())==reference,'STOP never overwrite frozen rollback schema'
        else:
            with frozen.open('x') as out:json.dump(reference,out,sort_keys=True)
    elif mode=='preflight':
        assert actual_structure==schema('schema-before.json')
        final=schema('schema-after-candidate.json')
        before_indexes={r['name']:r for r in actual_structure if r['type']=='index'}
        final_indexes={r['name']:r for r in final if r['type']=='index'}
        for name in required: assert final_indexes[name]==clean[name],'STOP after-reference differs from clean expected definition'
        for name in before_indexes.keys() & final_indexes.keys():
            assert before_indexes[name]==final_indexes[name],'STOP existing index SQL/xinfo differs'
        # Chosen17 reference covers exact PR36/38 and ALL preserved baseline objects; no new dedupe/Guardian requirement.
        assert {'idx_usage_dashboard_time_v1','idx_usage_dashboard_provider_time_v1','idx_usage_service_tier_pending_v1','idx_billing_charge_base_amount_pending_v1'}<=final_indexes.keys()
        proof=load('candidate-embed-proof.json');built=load('production-build.json');rehearsal=load('rehearsal.json')
        assert proof['ok'] is True and proof['image_id']==os.environ['NEW_IMAGE']
        assert proof['embedded_panel_files_matched']==m['file_count'] and proof['embedded_panel_bytes_matched']==m['total_bytes']
        binary=build/'out/image-home-binary';raw=binary.read_bytes()
        assert proof['binary_path']==str(binary) and hashlib.sha256(raw).hexdigest()==proof['binary_sha256']
        models=(build/'src/internal/registry/models/models.json').read_bytes()
        modelsha='4a52db3f1e4cdc10d045347f758b875fb2c4cbc148ae48ded43afb27037bd0e9'
        assert len(models)==132057 and hashlib.sha256(models).hexdigest()==modelsha and models in raw,'STOP exact model JSON not embedded'
        assert proof['models_sha256']==built['models_sha256']==modelsha
        assert built['ok'] is True and built['image_id']==proof['image_id'] and built['tag']==os.environ['NEW_TAG']
        assert built['candidate_embed_proof']==proof and built['models_exact_bytes_verified_in_live_and_candidate'] is True
        assert built['panel_tree_sha256']==m['tree_sha256'] and all(x['ok'] is True for x in built['checks'].values())
        assert {'format','vet','focused','full','compile'}<=built['checks'].keys()
        assert rehearsal['ok'] is True and rehearsal['image_id']==proof['image_id']
        assert proof['source_commit']==built['source_commit']==rehearsal['source_commit']=='7091c86b562063b5c401f63d6d04c8c4f70658a2'
        assert all(rehearsal['protected_preserved'].values()) and rehearsal['parallel_seed_integrity']['integrity']==['ok'] and rehearsal['parallel_seed_integrity']['exit']==0
        for phase in ('first','second','reupgrade'): assert rehearsal['phases'][phase]['ok'] is True and rehearsal['phases'][phase]['image_id']==proof['image_id']
        assert rehearsal['phases']['old']['ok'] is True and rehearsal['phases']['old']['image_id']==os.environ['OLD_IMAGE']
        # mirror.py independently enumerates every candidate embed record against the pinned full manifest.
        # Independent final review must also bind the actual panel/model input SHA and measured image; no source-label substitution.
        tree=build/'panel/static-full'
        assert {str(x.relative_to(tree)) for x in tree.rglob('*') if x.is_file()}==set(m['files']) and not any(x.is_symlink() for x in tree.rglob('*'))
        for n,want in m['files'].items():
            raw=(tree/n).read_bytes();assert len(raw)==want['bytes'] and hashlib.sha256(raw).hexdigest()==want['sha256']
        v=g['inspect']();assert v['Image']==os.environ['OLD_IMAGE'] and v['State']['Running'] and not v['State']['Paused'] and v['RestartCount']==0
        env=dict(x.split('=',1) for x in v['Config']['Env']);assert len(env)==len(v['Config']['Env'])
        assert env['TMPDIR']==env['SQLITE_TMPDIR']=='/CLIProxyAPIHome/data/sqlite-tmp' and v['HostConfig']['ReadonlyRootfs']
        assert 'size=64m' in v['HostConfig']['Tmpfs']['/tmp'].lower() or 'size=67108864' in v['HostConfig']['Tmpfs']['/tmp'].lower()
        mounts={x['Destination']:x['Source'] for x in v['Mounts']}
        for name in ('cluster.yaml','data','logs','plugins'): assert mounts['/CLIProxyAPIHome/'+name]=='/mnt/cache/appdata/cpa-home/'+name
        raw=pathlib.Path(os.environ['COMPOSE']).read_bytes();text=raw.decode()
        import re
        # Conservative supported Compose shape: one service, one plain image scalar; reject ambiguous YAML.
        assert len(re.findall(r'(?m)^services:\s*$',text))==1 and len(re.findall(r'(?m)^  home:\s*$',text))==1
        assert len(re.findall(r'''(?m)^\s*["']?image["']?\s*:''',text))==1
        assert not re.search(r'(?m)^\s*[^#\n]*[&*]|^\s*<<\s*:',text)
        match=re.search(r'(?m)^    image: ([^\s#]+)[ \t]*$',text)
        assert match and match.group(1)==os.environ['OLD_TAG']
        after=(text[:match.start(1)]+os.environ['NEW_TAG']+text[match.end(1):]).encode()
        for filename,data,want in (('compose.before.yml',raw,'OLD_TAG'),('compose.candidate.yml',after,'NEW_TAG')):
            (p/filename).write_bytes(data)
            resolved=json.loads(g['docker']('compose','-f',str(p/filename),'--project-directory',str(pathlib.Path(os.environ['COMPOSE']).parent),'-p','cpa-home','config','--format','json'))
            assert set(resolved['services'])=={'home'} and resolved['services']['home']['image']==os.environ[want]
        (p/'old-runtime.json').write_text(json.dumps({'id':v['Id'],'started_at':v['State']['StartedAt'],'footprint':digest(g['footprint'](v)),'logging':digest(v['HostConfig']['LogConfig']),'restart':v['HostConfig']['RestartPolicy']}))
    elif mode=='baseline':
        inventory=load('expected-inventory.json');s=snapshot();assert s['auth_db_ids']==sorted(inventory['credential_ids'])
        keyids=[r[0] for r in c.execute('select id from api_key where deleted_at is null order by id')];assert len(keyids)==6 and keyids==inventory['api_key_ids']
        a=s['accounts'];claude=[v for v in a.values() if v['provider']=='claude'];codex=[v for v in a.values() if v['provider']=='codex']
        assert sum(not v['disabled'] and not v['reauth_required'] for v in claude)==12
        assert sum(v['disabled'] and v['reauth_required'] for v in claude)==1
        assert all(not v['disabled'] and not v['reauth_required'] for v in codex)
        f=g['api']('/fleet');assert f['policy']['enabled'] is False and f['policy']['patch_updates'] is False and f.get('active') is None
        policy=f['policy']['nodes'];required={n['id'] for n in policy if n['required']}
        assert {n['name'].lower() for n in policy if n['required']}=={'moxy','chip','lara','hyper'}
        observed=load('awake-nodes.json');awake=set(observed['awake']);sleep=set(observed['sleeping'])
        assert not awake&sleep and awake|sleep=={n['id'] for n in policy} and required<=awake
        nodes=g['api']('/nodes')['nodes'];byid={n['node_id']:n for n in nodes};assert len(byid)==len(nodes)
        assert {nid for nid,n in byid.items() if n['healthy']}<=awake and awake<=byid.keys()
        (p/'baseline.json').write_text(json.dumps(s,sort_keys=True));(p/'expected-nodes.json').write_text(json.dumps({'awake':sorted(awake),'sleeping':sorted(sleep)}))
    else:
        assert mode in ('ready','stable') # Exact target structure was checked above, including frozen rollback.
        b=json.loads((p/'baseline.json').read_text());s=snapshot(b)
        for key,want in b.items():
            if key not in ('live_usage_highwater','accounts'): assert s[key]==want,'STOP protected state changed'
        assert s['live_usage_highwater']>=b['live_usage_highwater']
        assert s['accounts'].keys()==b['accounts'].keys()
        for uid,before in b['accounts'].items():
            after=s['accounts'][uid]
            for key in ('provider','disabled','reauth_required','settings_sha256'): assert after[key]==before[key],'STOP credential settings/authentication changed'
            # Quota availability is separate. Any changed non-active status/message needs review, never normalization.
            assert after['status']=='active' or (after['status']==before['status'] and after['message_sha256']==before['message_sha256']),'STOP classify changed account error before acceptance'
        v=g['inspect']();old=json.loads((p/'old-runtime.json').read_text())
        assert v['Image']==os.environ['ACCEPT_IMAGE'] and v['State']['Running'] and not v['State']['Paused'] and v['RestartCount']==0
        assert digest(g['footprint'](v))==old['footprint'] and digest(v['HostConfig']['LogConfig'])==old['logging']
        f=g['api']('/fleet');assert f['policy']['enabled'] is False and f['policy']['patch_updates'] is False and f.get('active') is None
        expected=json.loads((p/'expected-nodes.json').read_text());nodes=g['api']('/nodes')['nodes'];byid={n['node_id']:n for n in nodes};assert len(byid)==len(nodes)
        for nid in expected['awake']:
            n=byid[nid];assert n['healthy'] and n['plugin_report_state']=='reported_ok'
            assert any(x['id']=='cliproxyapi-copilot' and x['version']=='0.3.7' and x['load_status']=='loaded' for status in n.get('plugin_report_statuses',[]) for x in status.get('plugins',[]))
        g['api']('/models?scope=available')
        for name in sorted(served):
            for path in (['/','/index.html'] if name=='index.html' else ['/'+name]):
                with g['opener'].open('http://100.110.133.6:8327'+path,timeout=30) as r:
                    assert r.status==200;raw=r.read()
                want=m['files'][name];assert len(raw)==want['bytes'] and hashlib.sha256(raw).hexdigest()==want['sha256']
        for name in ('.gitkeep','panel-manifest.json'):
            try: g['opener'].open('http://100.110.133.6:8327/'+name,timeout=15)
            except urllib.error.HTTPError as e: assert e.code==404
            else: raise AssertionError('STOP nonroute falsely served')
        boundary=p/('ready-'+os.environ['ACCEPT_LABEL']+'.json')
        if mode=='ready':
            assert time.time()<int(os.environ['READINESS_DEADLINE']),'STOP readiness deadline expired'
            import datetime
            assert not boundary.exists(),'STOP never overwrite native boundary'
            boundary.write_text(json.dumps({'id':s['live_usage_highwater'],'timestamp':datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%d %H:%M:%S'),'container_id':v['Id'],'started_at':v['State']['StartedAt'],'image_id':v['Image'],'expected_nodes':expected['awake'],'ready_monotonic':time.monotonic()}))
        else:
            z=json.loads(boundary.read_text());assert time.monotonic()-z['ready_monotonic']>=60
            assert (v['Id'],v['State']['StartedAt'],v['Image'],expected['awake'])==(z['container_id'],z['started_at'],z['image_id'],z['expected_nodes'])
    print(mode+'=PASS; panel_embed=1060; http_subset='+str(len(served)))
finally: c.close()
PY
}
export -f verify_home
io_advisory() {
if python3 -I - <<'PY'
import datetime,hashlib,json,os,pathlib,subprocess,sys,time
receipt={'decision':'proceed','samples':0,'elapsed_seconds':0,'last_mover_running':None,'mover_overlap':None,'error':None,'parity_idle':None,'parity_error':None,'parity_status_fields':{},'parity_sample_elapsed_seconds':None}
start=time.monotonic()
def unique(pairs):
    out={}
    for key,value in pairs:
        if key in out:raise ValueError('duplicate')
        out[key]=value
    return out
def observe(v):
    if not isinstance(v,dict):return None,'malformed_output'
    # Only fixed status keys and bounded known values may enter the receipt.
    numeric={'sbSynced','sbSyncErrs','mdNumDisabled','mdNumInvalid','mdResyncSize','mdResyncCorr','mdResync','mdResyncPos','mdResyncDt','mdResyncDb'}
    fields=v.get('parity_status_fields');safe={};bad=False
    if isinstance(fields,dict):
        for key,value in fields.items():
            if key not in numeric|{'mdState','mdResyncAction'}:continue
            valid=isinstance(value,str) and ((key in numeric and value.isascii() and value.isdigit() and len(value)<=20) or (key=='mdState' and value in ('STARTED','STOPPED')) or (key=='mdResyncAction' and value in ('','idle','check','check P','check Q','check P Q','recon P','recon Q','sync','resync','clear')))
            if valid:safe[key]=value
            else:bad=True
    else:bad=True
    parity=v.get('parity_idle');error=v.get('parity_error')
    if type(parity) is not bool and parity is not None:parity=None;bad=True
    if error not in (None,'missing','unreadable','timeout','exec_failed','nonzero','malformed_status','array_not_started'):error='malformed_status'
    receipt.update(parity_idle=parity,parity_error='malformed_status' if bad else error,parity_status_fields=safe,parity_sample_elapsed_seconds=round(time.monotonic()-start,3))
    try:
        stamp=datetime.datetime.fromisoformat(v['utc'])
        if stamp.utcoffset() is None or not 0<=(datetime.datetime.now(datetime.timezone.utc)-stamp).total_seconds()<=30:return None,'stale_sample'
        if v['production_writes'] is not False or v['mdcmd_path']!='/usr/local/sbin/mdcmd':return None,'malformed_output'
        idle=v['mover_idle'];pid=v['mover_pidfile'];processes=v['mover_processes']
        valid=type(idle) is bool and type(v['unreadable_process_count']) is int and v['unreadable_process_count']==0 and isinstance(pid,dict) and pid['path']=='/var/run/mover.pid' and pid['readable'] is True and type(pid['exists']) is bool and type(pid['live']) is bool and isinstance(processes,list)
        valid=valid and ((pid['pid'] is None and not pid['live']) if not pid['exists'] else type(pid['pid']) is int and 0<pid['pid']<=2147483647)
        if not valid or idle!=(not processes and not pid['live']):return None,'mover_unknown'
        return not idle,None
    except (KeyError,TypeError,ValueError,OverflowError):return None,'mover_unknown'
try:
    script=pathlib.Path(os.environ['BUILD'])/'scripts/io-status.py'
    try:source=script.read_bytes()
    except OSError:receipt['error']='helper_unavailable';source=None
    if source is not None and hashlib.sha256(source).hexdigest()!='804fbeeea51debe0146613a94086ffe826a8c3dd47f1e7545f393d35f381e4a1':
        receipt['error']='helper_hash_mismatch';source=None
    while source is not None:
        remaining=600-(time.monotonic()-start)
        if remaining<=0:break
        try:result=subprocess.run(['python3','-I','-B','-c',source.decode('utf-8')],capture_output=True,text=True,timeout=min(20,remaining))
        except subprocess.TimeoutExpired:receipt.update(last_mover_running=None,mover_overlap=None,error='helper_timeout');break
        except (OSError,UnicodeError):receipt.update(last_mover_running=None,mover_overlap=None,error='helper_exec_failed');break
        receipt['samples']+=1
        if result.returncode!=0:receipt.update(last_mover_running=None,mover_overlap=None,error='helper_nonzero');break
        try:v=json.loads(result.stdout,object_pairs_hook=unique)
        except (ValueError,TypeError):receipt.update(last_mover_running=None,mover_overlap=None,error='malformed_output');break
        running,error=observe(v)
        receipt.update(last_mover_running=running,mover_overlap=running,error=error)
        if running is not True:break
        remaining=600-(time.monotonic()-start)
        if remaining<=0:break
        time.sleep(min(5,remaining))
except Exception:receipt.update(last_mover_running=None,mover_overlap=None,error='advisory_failed')
receipt['elapsed_seconds']=round(time.monotonic()-start,3)
try:(pathlib.Path(os.environ['WS'])/'io-prestop-advisory.json').write_text(json.dumps(receipt,sort_keys=True))
except Exception:
    receipt['log_error']='log_write_failed'
    try:print(json.dumps(receipt,sort_keys=True),file=sys.stderr)
    except Exception:pass
PY
then :; else printf '%s\n' 'IO_ADVISORY error=wrapper_failed; proceed' >&2 || :; fi
return 0
}
export -f io_advisory
verify_native() {
python3 -I - <<'PY'
import json,os,pathlib,re
p=pathlib.Path(os.environ['WS']);label=os.environ['ACCEPT_LABEL'];assert label in ('forward','rollback')
def unique(pairs):
    out={}
    for k,v in pairs: assert k not in out,'STOP duplicate native JSON key';out[k]=v
    return out
def load(path): return json.loads(path.read_text(),object_pairs_hook=unique)
v=load(p/('native-'+label+'.json'));code=(p/('native-'+label+'.exit')).read_text().strip()
if code!='0':
    assert set(v)=={'ok','error'} and v['ok'] is False and re.fullmatch(r'[a-z_0-9]{1,160}',v['error']),'STOP invalid abort contract'
    raise AssertionError('STOP native helper aborted: '+v['error'])
assert set(v)=={'ok','candidate_image','boundary','nodes','native_requests','stability_seconds','scope','macbook'} and v['ok'] is True
assert type(v['stability_seconds']) is int and v['stability_seconds']==60
assert v['scope']=='native_usage_and_membership_only; panel_and_preservation_are_separate_gates'
b=load(p/('ready-'+label+'.json'));projection={k:b[k] for k in ('id','timestamp','container_id','started_at','expected_nodes')}
assert v['boundary']==projection and v['candidate_image']==b['image_id']==os.environ['ACCEPT_IMAGE'],'STOP wrong native boundary/image'
assert len(b['expected_nodes'])==len(set(b['expected_nodes']))
assert isinstance(v['nodes'],list) and len(v['nodes'])==len(b['expected_nodes'])
assert {n['node_id'] for n in v['nodes']}==set(b['expected_nodes']),'STOP awake native node set changed'
ips={'100.110.240.90','100.89.64.86','100.71.124.48','100.70.69.110'}
actual_ips={n['ip'] for n in v['nodes']};assert len(actual_ips)==len(v['nodes']) and ips<=actual_ips<=ips|{'100.88.81.10'}
assert v['macbook']==('tested' if '100.88.81.10' in actual_ips else 'not_connected_optional_skip')
for n in v['nodes']:
    assert set(n)=={'node_id','ip','version','sha256','size','platform','catalog'} and n['version']=='8.0.69'
    assert re.fullmatch(r'[a-fA-F0-9]{64}',n['sha256']) and type(n['size']) is int and n['size']>0
    assert n['platform']=='darwin/arm64' and type(n['catalog']) is bool
requests=v['native_requests'];assert isinstance(requests,list) and len(requests)==2*len(v['nodes'])
assert {(q['node_id'],q['provider']) for q in requests}=={(n['node_id'],provider) for n in v['nodes'] for provider in ('claude','codex')}
assert len({q['request_id'] for q in requests})==len(requests) and len({q['usage_id'] for q in requests})==len(requests)
for q in requests:
    assert set(q)=={'provider','model','request_id','response_id','http_status','latency_ms','node_id','usage_id','credential_label'}
    assert re.fullmatch(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}',q['request_id'])
    assert type(q['http_status']) is int and q['http_status']==200 and type(q['latency_ms']) is int and q['latency_ms']>=0
    assert type(q['usage_id']) is int and q['usage_id']>b['id'] and re.fullmatch(r'[0-9a-f]{6}',q['credential_label'])
    assert re.fullmatch(('msg_' if q['provider']=='claude' else 'resp_')+r'[A-Za-z0-9_-]{1,128}',q['response_id'])
    assert re.fullmatch(r'claude-haiku-\d[\d.-]*',q['model']) if q['provider']=='claude' else q['model']=='gpt-6.2-sol'
print('native_actual_contract_same_ready_60s=PASS; preservation_and_panel=SEPARATE')
PY
}
export -f verify_native
hot_backup() {
python3 -I - "$@" <<'PY'
import datetime,hashlib,json,math,os,pathlib,re,stat,subprocess,sys,types
PIN='ba6aa596a76eef218fdb7cadbabe50dadf1513c3f487a228d795f312f5658447'
TEST_PIN='cca6ebb4464942011eca7a0c400df4efa8829be93ad892d834c9701da6aafb17'
METHOD='readonly sqlite3_backup API BEGIN+count(auth)+backup+ROLLBACK; descriptor-rooted child'
WINDOW_DAY='2026-11-19'  # Chanse 11-19 15:20Z: run today once Hype Orchestrator OKs a time
DAY=datetime.datetime.fromisoformat(WINDOW_DAY).replace(tzinfo=datetime.timezone.utc)
NOTICE=DAY+datetime.timedelta(hours=20)  # HOT must be verified before 20:00Z (14:00 CT)
def unique(pairs):
    out={}
    for key,value in pairs:assert key not in out,'duplicate_json';out[key]=value
    return out
def decode(raw):return json.loads(raw,object_pairs_hook=unique,parse_constant=lambda _:(_ for _ in ()).throw(ValueError('nonfinite_json')))
def stamp(value):
    v=datetime.datetime.fromisoformat(value);assert v.utcoffset() is not None,'naive_time';return v

def contract(v,started,finished,stdout,base,now):
    target=pathlib.Path('/mnt/disk18/cpa-home-disaster-backups')/base.name
    assert set(started)=={'base','helper_sha256','started_utc'} and started['base']==str(base) and started['helper_sha256']==PIN,'invocation_binding'
    assert set(finished)=={'exit','completed_utc'} and type(finished['exit']) is int and finished['exit']==0,'producer_exit'
    begin,end=stamp(started['started_utc']),stamp(finished['completed_utc'])
    assert DAY<=begin<=stamp(v['started_utc'])<=stamp(v['completed_utc'])<=end<=now<DAY+datetime.timedelta(days=1) and end<NOTICE,'stale_or_late_invocation'
    assert type(v['schema_version']) is int and v['schema_version']==1 and v['ok'] is True and v['error'] is None,'receipt_failed'
    assert v['method']==METHOD and v['source_direct_path']=='/mnt/cache/appdata/cpa-home/data/home.db' and v['source_uri']=='file:/mnt/cache/appdata/cpa-home/data/home.db?mode=ro','source_or_method'
    paths={'base':base,'stage_path':base/'home-hot.db','result_path':base/'hot-backup.json','destination_path':target/'home-hot.db','retained_result_path':target/'hot-backup.json','checksum_path':target/'home-hot.db.sha256'}
    assert all(v[key]==str(path) for key,path in paths.items()),'receipt_paths'
    keys=('ok','error','result_path','destination_path','bytes','retained_copy','stage_cleanup_state')
    assert set(stdout)==set(keys) and stdout=={key:v[key] for key in keys} and stdout['ok'] is True and stdout['retained_copy'] is True,'stdout_mismatch'
    for key in ('pin_selfcheck_ok','stage_verified','retained_copy','destination_match','wal_guard_active_before_begin'):assert v[key] is True,key
    assert v['integrity_check']=='FULL' and v['integrity']==['ok'] and type(v['integrity_exit']) is int and v['integrity_exit']==0,'FULL_required'
    for key in ('bytes','destination_bytes'):assert type(v[key]) is int and v[key]>0,'positive_bytes'
    assert v['destination_bytes']==v['bytes'] and re.fullmatch('[0-9a-f]{64}',v['sha256']) and v['destination_sha256']==v['sha256'],'destination_match'
    assert v['stage_cleanup_state']=='deleted_after_match' and v['destination_state']=='verified_retained','retained_state'
    for key in ('copy_exit','transfer_exit'):assert type(v[key]) is int and v[key]==0,key
    assert v['abort_reason'] is None and v['copy_bound_seconds']==600 and v['wal_growth_limit_bytes']==1024**3 and v['wal_sample_interval_seconds']==0.1,'guard_contract'
    for key in ('copy_seconds','integrity_seconds','transfer_seconds'):assert type(v[key]) in (int,float) and math.isfinite(v[key]) and v[key]>=0,key
    assert v['copy_seconds']<=600,'copy_bound'
    for key in ('live_wal_start_bytes','live_wal_peak_observed_bytes','live_wal_growth_bytes'):assert type(v[key]) is int and v[key]>=0,key
    assert v['live_wal_peak_observed_bytes']-v['live_wal_start_bytes']==v['live_wal_growth_bytes']<=1024**3,'wal_growth'
    pin=v['source_pin'];assert set(pin)=={'db_dev_ino','wal_dev_ino','directory_dev_ino','sqlite_open_uri','same_direct_directory','readonly','begin_and_select'},'source_pin_keys'
    assert pin['sqlite_open_uri']=='file:/home.db?mode=ro' and all(pin[key] is True for key in ('same_direct_directory','readonly','begin_and_select')),'source_pin_flags'
    for key in ('db_dev_ino','directory_dev_ino','wal_dev_ino'):
        value=pin[key]
        if key=='wal_dev_ino' and value is None:assert all(v[name]==0 for name in ('live_wal_start_bytes','live_wal_peak_observed_bytes','live_wal_growth_bytes'));continue
        assert isinstance(value,list) and len(value)==2 and all(type(x) is int for x in value) and value[0]==44 and value[1]>0,'source_pin_identity'
    assert pin['db_dev_ino']!=pin['directory_dev_ino'] and (pin['wal_dev_ino'] is None or pin['wal_dev_ino'] not in (pin['db_dev_ino'],pin['directory_dev_ino'])),'source_pin_alias'
    devices=v['devices'];cache={'mount':'/mnt/cache','filesystem':'btrfs','device':'/dev/nvme1n1p1','major_minor':v['devices']['cache_mount'].get('major_minor')};array={'mount':'/mnt/disk18','filesystem':'xfs','device':'/dev/md18p1','major_minor':'9:18'}
    assert devices=={'source_dev':44,'stage_dev':44,'destination_dev':2322,'different_devices':True,'cache_mount':cache,'destination_mount':array} and devices['different_devices'] is True,'device_proof'
    head=v['headroom'];assert head['pass'] is True and head['minimum_free_fraction']==0.2,'headroom_failed'
    for key in ('cache_total_bytes','cache_free_bytes_before','reserve_bytes','cache_free_bytes_after_reserve'):assert type(head[key]) is int and head[key]>=0,key
    assert head['cache_total_bytes']>0 and head['reserve_bytes']>=2*max(45*1024**3,v['bytes']) and head['cache_free_bytes_after_reserve']==head['cache_free_bytes_before']-head['reserve_bytes'] and head['cache_free_bytes_after_reserve']*5>=head['cache_total_bytes'],'headroom_reserve'
    assert set(v['tools'])=={'python3'} and isinstance(v['tools']['python3'],str),'runtime_proof'

def verify(h,base,invocation,initial=False):
    directories=[]
    try:
        for path in (invocation,base,h.ARRAY_ROOT/base.name):
            directory=h.Directory(path);directories.append(directory)
            assert stat.S_IMODE(os.fstat(directory.fd).st_mode)==0o700,'private_directory'
        run,stage,target=directories
        def read(directory,name):
            fd=directory.open_file(name);assert stat.S_IMODE(directory.check_file(name).st_mode)==0o600,'private_file'
            return os.pread(fd,os.fstat(fd).st_size,0)
        raw_started=read(run,'started.json');raw_finished=read(run,'finished.json');raw_stdout=read(run,'stdout.json')
        stage_raw=read(stage,'hot-backup.json');target_raw=read(target,'hot-backup.json')
        v=decode(stage_raw);assert stage_raw==target_raw,'receipt_pair_mismatch'
        contract(v,decode(raw_started),decode(raw_finished),decode(raw_stdout),base,datetime.datetime.now(datetime.timezone.utc))
        assert v['tools']['python3']==h.trusted_tool('python3'),'runtime_path'
        mounts=pathlib.Path('/proc/self/mountinfo').read_text()
        assert h.mount_for(base,mounts)==v['devices']['cache_mount'] and h.mount_for(target.path,mounts)==v['devices']['destination_mount'],'current_mounts'
        assert os.fstat(stage.fd).st_dev==44 and os.fstat(target.fd).st_dev==2322,'current_devices'
        free=os.fstatvfs(stage.fd);assert (free.f_bavail*free.f_frsize-v['headroom']['reserve_bytes'])*5>=free.f_blocks*free.f_frsize,'current_headroom'
        checksum=read(target,'home-hot.db.sha256');assert checksum==(v['sha256']+'  home-hot.db\n').encode(),'checksum_file'
        fd=target.open_file('home-hot.db');before=target.check_file('home-hot.db')
        assert stat.S_IMODE(before.st_mode)==0o600 and before.st_dev==2322 and before.st_size==v['bytes'],'physical_bytes_or_device'
        assert h.sha256(fd)==v['sha256'],'physical_sha'
        after=target.check_file('home-hot.db');assert (before.st_size,before.st_mtime_ns,before.st_ctime_ns)==(after.st_size,after.st_mtime_ns,after.st_ctime_ns),'physical_changed'
        try:os.stat('home-hot.db',dir_fd=stage.fd,follow_symlinks=False)
        except FileNotFoundError:pass
        else:raise AssertionError('stage_not_deleted')
        binding={'base':str(base),'helper_sha256':PIN,'invocation_sha256':[hashlib.sha256(raw).hexdigest() for raw in (raw_started,raw_finished,raw_stdout)],'receipt_sha256':hashlib.sha256(stage_raw).hexdigest(),'checksum_sha256':hashlib.sha256(checksum).hexdigest(),'retained_identity':[before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns,before.st_ctime_ns],'invocation_identity':[os.fstat(run.fd).st_dev,os.fstat(run.fd).st_ino]}
        expected={run:{'started.json':raw_started,'finished.json':raw_finished,'stdout.json':raw_stdout},stage:{'hot-backup.json':stage_raw},target:{'hot-backup.json':target_raw,'home-hot.db.sha256':checksum}}
        if not initial:
            raw_success=read(run,'physical-verified.json');success=decode(raw_success)
            verified=stamp(success.pop('verified_utc'))
            assert success==binding and stamp(decode(raw_finished)['completed_utc'])<=verified<=datetime.datetime.now(datetime.timezone.utc) and verified<NOTICE,'initial_verification_binding_or_time'
            expected[run]['physical-verified.json']=raw_success
        for directory in directories:
            directory.verify()
            for name,fd in directory.files.items():
                directory.check_file(name)
                if name in expected[directory]:assert os.pread(fd,os.fstat(fd).st_size,0)==expected[directory][name],'receipt_changed'
        if initial:
            verified=datetime.datetime.now(datetime.timezone.utc)
            assert stamp(decode(raw_finished)['completed_utc'])<=verified<NOTICE,'physical_verification_missed_notice'
            fd=run.open_file('physical-verified.json',create=True)
            try:
                raw_success=json.dumps(dict(binding,verified_utc=verified.isoformat())).encode()
                h.write_all(fd,raw_success,0);os.fsync(fd);os.fsync(run.fd);run.verify();run.check_file('physical-verified.json')
                assert os.pread(fd,os.fstat(fd).st_size,0)==raw_success,'success_record_changed'
                assert datetime.datetime.now(datetime.timezone.utc)<NOTICE,'physical_verification_missed_notice'
            except Exception:
                run.remove('physical-verified.json');raise
    finally:
        for directory in reversed(directories):directory.close()

try:
    assert len(sys.argv)==2 and sys.argv[1] in ('create','verify') and sys.platform.startswith('linux') and os.geteuid()==0,'invalid_action_or_platform'
    base=pathlib.Path(os.environ['HOT_BASE']);assert re.fullmatch(r'/mnt/cache/appdata/cpa-home-build/ws0054-hot-[A-Za-z0-9][A-Za-z0-9_-]{0,95}',str(base)),'unsafe_base'
    invocation=base.with_name(base.name+'.invocation');folder=pathlib.Path(os.environ['BUILD'])/'approved-hot-btrfsfix'
    source=(folder/'ws0054-hot-backup.py').read_bytes();assert hashlib.sha256(source).hexdigest()==PIN,'helper_hash'
    namespace={'__name__':'hot_library'};exec(compile(source,str(folder/'ws0054-hot-backup.py'),'exec'),namespace);h=types.SimpleNamespace(**namespace)
    approved=h.Directory(folder)
    try:
        assert stat.S_IMODE(os.fstat(approved.fd).st_mode)==0o700,'source_directory_mode'
        for name,want in (('ws0054-hot-backup.py',PIN),('ws0054_hot_backup_test.py',TEST_PIN)):
            fd=approved.open_file(name);assert stat.S_IMODE(approved.check_file(name).st_mode)==0o600 and h.sha256(fd)==want,'source_or_test_pin'
        approved.verify()
    finally:approved.close()
    if sys.argv[1]=='create':
        h.fresh_base(str(base));cache=h.Directory(h.CACHE_ROOT);run=None
        try:
            run=h.Directory(invocation,cache)
            def save(name,data):
                fd=run.open_file(name,create=True);h.write_all(fd,data,0);os.fsync(fd);os.fsync(run.fd);run.verify();run.check_file(name)
            save('started.json',json.dumps({'base':str(base),'helper_sha256':PIN,'started_utc':h.utc()}).encode())
            result=subprocess.run([sys.executable,'-I','-B','-c',source.decode('utf-8'),str(base)],capture_output=True,env={'PATH':'/usr/bin:/bin'})
            save('stdout.json',result.stdout)
            save('finished.json',json.dumps({'exit':result.returncode,'completed_utc':h.utc()}).encode())
        finally:
            if run is not None:run.close()
            cache.close()
    verify(h,base,invocation,sys.argv[1]=='create')
    print('actual_HOT_invocation_FULL_sourcepin_guard_devices_headroom_retained_bytes_SHA=PASS')
except Exception as error:
    print('HOT_REJECTED '+type(error).__name__,file=sys.stderr);sys.exit(1)
PY
}
export -f hot_backup

save(){ printf 'export WS=%q NEXT=%q ACCEPT_TARGET=%q ACCEPT_LABEL=%q\n' "$WS" "$NEXT" "${ACCEPT_TARGET:-}" "${ACCEPT_LABEL:-}" > "$STATE.tmp" && mv "$STATE.tmp" "$STATE"; echo "STATE NEXT=$NEXT TARGET=${ACCEPT_TARGET:-} LABEL=${ACCEPT_LABEL:-} $(date -u +%FT%TZ)"; }
recover(){
if test ! -e "$WS/outage-start" && test ! -e "$WS/candidate-may-have-run"; then
echo "=== route A (old never stopped) $(date -u +%FT%TZ)"
# ---- core route A (verbatim)
if test "$NEXT" = RECOVERY && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
python3 -I - <<'PY'
import os,json,pathlib,subprocess
p=pathlib.Path(os.environ['WS']);old=json.loads((p/'old-runtime.json').read_text())
assert not (p/'outage-start').exists() and not (p/'candidate-may-have-run').exists()
v=json.loads(subprocess.check_output(['docker','inspect','cpa-home'],text=True))[0]
assert v['Id']==old['id'] and v['Image']==os.environ['OLD_IMAGE'] and v['State']['StartedAt']==old['started_at']
assert v['State']['Running'] and not v['State']['Paused'] and not v['State'].get('Restarting') and v['RestartCount']==0
PY
test -f "$WS/policy-before.json"
SH
then NEXT=RESTORE; ACCEPT_LABEL=abort-old; export ACCEPT_LABEL; else NEXT=RECOVERY; printf '%s\n' 'Abort unverified: retain ownership; inspect actual old/policy state, no fallthrough.'; fi
# ---- Annex S (verbatim)
if test "$NEXT" = RESTORE && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
if test "$ACCEPT_LABEL" != abort-old; then
  verify_native # exact real JSON/exit/boundary contract, not a bare ok file
  verify_home stable # separate preservation/full panel; abort-old was positively unchanged
fi
python3 -I - <<'PY'
import os,json,pathlib,time,urllib.request,urllib.error
p=pathlib.Path(os.environ['WS']);saved=json.loads((p/'policy-before.json').read_text());target=dict(saved,patch_updates=False)
op=urllib.request.build_opener(urllib.request.ProxyHandler({}));base='http://100.110.133.6:8327/v8/management'
def api(path,method='GET',body=None):
    req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),method=method,headers={'Authorization':'Bearer '+os.environ['MANAGEMENT_PASSWORD'],'Content-Type':'application/json'})
    with op.open(req,timeout=15) as r:return json.load(r)
end=time.monotonic()+180
while True:
    assert time.monotonic()<end,'STOP restore timeout; keep locks'
    f=api('/fleet')
    assert f['policy'] in (dict(saved,enabled=False,patch_updates=False),target),'STOP foreign policy drift'
    if f.get('active') is not None:time.sleep(2);continue
    try:api('/fleet/policy','PUT',target)
    except urllib.error.HTTPError:time.sleep(2);continue
    if api('/fleet')['policy']==target:break
    time.sleep(2)
print('saved_policy_exact_readback_patch_updates_false=PASS')
PY
SH
then NEXT=CLOSEOUT; else NEXT=RECOVERY; printf '%s\n' 'STOP restore unverified; keep lock ownership and reconcile policy.'; fi
else
echo "=== route B: Annex R $(date -u +%FT%TZ)"
# ---- Annex R (verbatim)
if test "$NEXT" = RECOVERY && bash -eu <<'SH'
python3 -I - <<'PY'
import os,pathlib,subprocess,json,time,sqlite3
p=pathlib.Path(os.environ['WS']);path=pathlib.Path(os.environ['COMPOSE'])
def docker(*a): return subprocess.check_output(['docker',*a],text=True,timeout=180)
ids=docker('ps','-aq','--filter','label=com.docker.compose.project=cpa-home','--filter','label=com.docker.compose.service=home').split();assert len(ids)<=1
if ids:
    cid=ids[0];v=json.loads(docker('inspect',cid))[0];assert v['Image'] in (os.environ['OLD_IMAGE'],os.environ['NEW_IMAGE'])
    docker('update','--restart=no',cid) # FIRST, before unpause/kill, including crash loops.
    assert json.loads(docker('inspect',cid))[0]['HostConfig']['RestartPolicy']['Name']=='no'
    end=time.monotonic()+90
    while True:
        v=json.loads(docker('inspect',cid))[0];s=v['State'];assert v['HostConfig']['RestartPolicy']['Name']=='no'
        if not s['Running'] and not s['Paused'] and not s.get('Restarting'):break
        assert time.monotonic()<end,'STOP writer not stopped/unpaused'
        if s['Paused']:docker('unpause',cid)
        if s['Running']:
            r=subprocess.run(['docker','kill','--signal','KILL',cid],capture_output=True)
            # A kill error is not success: the next positive inspect is mandatory.
        time.sleep(1)
# Zero containers is allowed ONLY with positive no-other-writer checks.
root=pathlib.Path(os.environ['DATA']).parent
running=docker('ps','-q').split()
for cid in running:
    for m in json.loads(docker('inspect',cid))[0]['Mounts']:
        src=pathlib.Path(m['Source'])
        assert not m.get('RW',False) or not (src==root or src in root.parents or root in src.parents),'STOP another container can write live data'
files=[str(x) for x in (pathlib.Path(os.environ['DATA']),pathlib.Path(os.environ['DATA']+'-wal'),pathlib.Path(os.environ['DATA']+'-shm')) if x.exists()]
r=subprocess.run(['fuser',*files],capture_output=True);assert r.returncode==1 and not r.stdout and not r.stderr,'STOP live DB user/no-writer proof unknown'
before=(p/'compose.before.yml').read_bytes();after=(p/'compose.candidate.yml').read_bytes();current=path.read_bytes()
original=json.loads((p/'old-runtime.json').read_text());reuse=bool(ids) and v['Image']==os.environ['OLD_IMAGE'] and not (p/'candidate-may-have-run').exists() and v['Id']==original['id']
assert reuse or current in (before,after),'STOP foreign Compose drift; do not overwrite'
assert docker('image','inspect',os.environ['OLD_TAG'],'--format','{{.Id}}').strip()==os.environ['OLD_IMAGE']
# Only here, after positive stopped/no-writer proof: freeze exact allowlisted schema for this attempt.
subprocess.run(['bash','-eu','-c','verify_home rollback-schema'],check=True,timeout=120)
# Ordinary mode=ro sees WAL. Missing/unreadable/nonempty all throw, NO old start.
c=sqlite3.connect('file:'+os.environ['DATA']+'?mode=ro',uri=True,timeout=45)
try:
    assert c.execute("select count(*) from sqlite_master where type='table' and name='codex_refresh_journal'").fetchone()[0]==1
    assert c.execute('select count(*) from codex_refresh_journal').fetchone()[0]==0,'STOP journal nonempty'
finally:c.close()
# No candidate resume from this point. Only restore owned Compose, or reuse exact original stopped old.
if reuse:
    docker('start',original['id']) # Leaves foreign Compose drift untouched; report it, never overwrite.
else:
    if current==after:
        tmp=path.with_name('.docker-compose.ws0054.rollback')
        with tmp.open('xb') as out:out.write(before);out.flush();os.fsync(out.fileno())
        tmp.chmod(path.stat().st_mode&0o777);assert path.read_bytes()==after;tmp.replace(path)
    docker('compose','-f',str(path),'-p','cpa-home','up','-d','--no-deps','--force-recreate','--pull','never','home')
v=json.loads(docker('inspect','cpa-home'))[0];assert v['Image']==os.environ['OLD_IMAGE']
policy=original['restart'];assert policy['Name'] in ('no','always','unless-stopped','on-failure')
restart=policy['Name']+(':'+str(policy['MaximumRetryCount']) if policy['Name']=='on-failure' and policy.get('MaximumRetryCount') else '')
docker('update','--restart='+restart,v['Id'])
print('old_started_current_DB_after_fresh_stopped_empty_WAL_journal_gate')
PY
SH
then NEXT=ACCEPT; ACCEPT_TARGET=rollback; export ACCEPT_TARGET; else NEXT=RECOVERY; printf '%s\n' 'OLD START BLOCKED/FAILED: C. Preserve current DB/journal; independent operator recovery.'; fi
# ---- core section 6 (verbatim)
case "$ACCEPT_TARGET" in
  forward) ACCEPT_IMAGE=$NEW_IMAGE; ACCEPT_SCHEMA_FILE=schema-after-candidate.json; ACCEPT_LABEL=forward; ACCEPT_SECONDS=$READY_SECONDS ;;
  rollback) ACCEPT_IMAGE=$OLD_IMAGE; ACCEPT_LABEL=rollback; ACCEPT_SECONDS=$ROLLBACK_SECONDS; ACCEPT_SCHEMA_FILE=rollback-schema.json ;;
  *) NEXT=RECOVERY; printf '%s\n' 'STOP: no accepted forward/rollback target.' ;;
esac
export ACCEPT_IMAGE ACCEPT_SCHEMA_FILE ACCEPT_LABEL ACCEPT_SECONDS
if test "$NEXT" = ACCEPT && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
export READINESS_DEADLINE=$(($(date +%s)+ACCEPT_SECONDS))
while :; do
  REMAINING=$((READINESS_DEADLINE-$(date +%s)))
  test "$REMAINING" -gt 0 || exit 1
  if timeout --signal=KILL "$REMAINING" bash -eu -c 'verify_home ready'; then
    test "$(date +%s)" -lt "$READINESS_DEADLINE" || exit 1
    break
  fi
  test "$(date +%s)" -lt "$READINESS_DEADLINE" || exit 1
  # Crash loops/paused/foreign image are NOT tolerated until a timeout; inspect and route B.
  test "$(docker inspect cpa-home --format '{{.RestartCount}} {{.State.Paused}}')" = '0 false' || exit 1
  sleep 5
done
SH
then NEXT=NATIVE; else NEXT=RECOVERY; printf '%s\n' 'STOP readiness: route B/C; locks remain owned.'; fi
fi
}
case "$PHASE" in
forward)
echo "=== forward start $(date -u +%FT%TZ)"
# ---- core section 2 (verbatim)
if test "$NEXT" = INPUTS && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
python3 -I - <<'PY'
import os,re,subprocess
assert re.fullmatch(r'cpa-home:1\.1\.9-claude-fleet-[0-9a-f]{7,40}',os.environ['NEW_TAG'])
assert re.fullmatch(r'sha256:[0-9a-f]{64}',os.environ['NEW_IMAGE']) and os.environ['NEW_IMAGE']!=os.environ['OLD_IMAGE']
assert os.environ['TARGET_SCHEMA']=='17'
for key in ('READY_SECONDS','ROLLBACK_SECONDS'): assert os.environ[key].isdigit() and int(os.environ[key])>0
for tag,key in (('OLD_TAG','OLD_IMAGE'),('NEW_TAG','NEW_IMAGE')):
    assert subprocess.check_output(['docker','image','inspect',os.environ[tag],'--format','{{.Id}}'],text=True).strip()==os.environ[key]
PY
test -f "$INPUT_SUMS"
sha256sum -c "$INPUT_SUMS"
mkdir -m 700 "$WS"
verify_home preflight
SH
then NEXT=PAUSE; else NEXT=RECOVERY; printf '%s\n' 'STOP inputs: old unchanged; route A.'; fi
echo "--- after inputs NEXT=$NEXT $(date -u +%FT%TZ)"
# ---- Annex P (verbatim)
if test "$NEXT" = PAUSE && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
python3 -I - <<'PY'
import os,json,pathlib,time,urllib.request,urllib.error
p=pathlib.Path(os.environ['WS']);op=urllib.request.build_opener(urllib.request.ProxyHandler({}));base='http://100.110.133.6:8327/v8/management'
def api(path,method='GET',body=None):
    req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),method=method,headers={'Authorization':'Bearer '+os.environ['MANAGEMENT_PASSWORD'],'Content-Type':'application/json'})
    with op.open(req,timeout=15) as r:return json.load(r)
f=api('/fleet');saved=f['policy'];assert saved['patch_updates'] is False
with (p/'policy-before.json').open('x') as out:json.dump(saved,out)
target=dict(saved,enabled=False,patch_updates=False);deadline=time.monotonic()+180
while True:
    assert time.monotonic()<deadline,'STOP pause timeout: reconcile policy; do not stop Home'
    f=api('/fleet');assert f['policy'] in (saved,target),'STOP foreign policy drift'
    if f.get('active') is not None: time.sleep(2);continue
    try: api('/fleet/policy','PUT',target)
    except urllib.error.HTTPError:
        # The racing task may already have finished before reread; bounded retry does not cancel/steal it.
        f=api('/fleet');assert f['policy'] in (saved,target),'STOP foreign policy drift'
        time.sleep(2);continue
    f=api('/fleet')
    if f['policy']==target and f.get('active') is None:break
    time.sleep(2)
print('paused_patch_updates_false_active_null=PASS')
PY
verify_home baseline
SH
then NEXT=STOP; else NEXT=RECOVERY; printf '%s\n' 'STOP pause/baseline: route A; restore saved policy only after old verification.'; fi
echo "--- after pause/baseline NEXT=$NEXT $(date -u +%FT%TZ)"
# ---- core section 4 (verbatim)
if test "$NEXT" = STOP && bash -eu <<'SH'
hot_backup verify # actual fresh invocation + matching private receipts + physical retained DB/SHA; Annex D
if io_advisory; then :; else printf '%s\n' 'IO_ADVISORY error=caller_failed; proceed' >&2 || :; fi
python3 -I - <<'PY'
import os,json,pathlib,subprocess
p=pathlib.Path(os.environ['WS']);old=json.loads((p/'old-runtime.json').read_text())
def docker(*a): return subprocess.check_output(['docker',*a],text=True,timeout=120)
v=json.loads(docker('inspect',old['id']))[0]
assert v['Image']==os.environ['OLD_IMAGE'] and v['State']['Running'] and not v['State']['Paused'] and v['State']['StartedAt']==old['started_at']
(p/'outage-start').write_text(str(__import__('time').time()))
docker('update','--restart=no',v['Id']);assert json.loads(docker('inspect',v['Id']))[0]['HostConfig']['RestartPolicy']['Name']=='no'
docker('stop','--time','90',v['Id']);s=json.loads(docker('inspect',v['Id']))[0]
assert s['HostConfig']['RestartPolicy']['Name']=='no' and not s['State']['Running'] and not s['State']['Paused'] and not s['State'].get('Restarting')
(p/'old-stopped').write_text(s['Id'])
PY
BACKUP=/mnt/cache/appdata/cpa-home/data/backups/home-cold-ws0054-$(date -u +%Y%m%dT%H%M%SZ).db
export BACKUP
test ! -e "$BACKUP"
python3 -I - <<'PY'
import os,pathlib,json,subprocess
p=pathlib.Path(os.environ['WS']);db=pathlib.Path(os.environ['DATA'])
users=subprocess.run(['fuser',*[str(x) for x in (db,pathlib.Path(str(db)+'-wal'),pathlib.Path(str(db)+'-shm')) if x.exists()]],capture_output=True)
assert users.returncode==1 and not users.stdout and not users.stderr,'STOP no-writer proof unknown'
state={s:{'exists':pathlib.Path(str(db)+s).exists(),'bytes':pathlib.Path(str(db)+s).stat().st_size if pathlib.Path(str(db)+s).exists() else 0} for s in ('','-wal','-shm')}
assert state['-wal']['bytes']==0,'STOP nonempty WAL: main-file reflink is unsafe; use separately reviewed WAL-aware backup route'
stat=db.stat();state['source_identity']={'device':stat.st_dev,'inode':stat.st_ino,'mtime_ns':stat.st_mtime_ns}
(p/'cold-source-state.json').write_text(json.dumps(state))
PY
# Approved COLD reflink only: never silently fall back to a normal raw copy.
cp --reflink=always --preserve=mode,timestamps -- "$DATA" "$BACKUP"
python3 -I - <<'PY'
import os,pathlib,json
p=pathlib.Path(os.environ['WS']);db=pathlib.Path(os.environ['DATA']);before=json.loads((p/'cold-source-state.json').read_text())
after={s:{'exists':pathlib.Path(str(db)+s).exists(),'bytes':pathlib.Path(str(db)+s).stat().st_size if pathlib.Path(str(db)+s).exists() else 0} for s in ('','-wal','-shm')}
stat=db.stat();after['source_identity']={'device':stat.st_dev,'inode':stat.st_ino,'mtime_ns':stat.st_mtime_ns}
assert after==before,'STOP stopped source identity/WAL-SHM state changed during copy'
copy=pathlib.Path(os.environ['BACKUP']).stat()
assert copy.st_dev==stat.st_dev and copy.st_ino!=stat.st_ino,'STOP reflink must be same filesystem, different inode'
assert copy.st_size==before['']['bytes']>0,'STOP cold copy/source size mismatch'
(p/'cold-source-after.json').write_text(json.dumps(after));(p/'cold-backup-path').write_text(os.environ['BACKUP'])
print('cold_reflink_exit0_distinct_inode_equal_size_source_WAL_SHM_unchanged=PASS')
PY
SH
then NEXT=CAS; else NEXT=RECOVERY; printf '%s\n' 'STOP backup/stop failure: route B if old stopped, otherwise A.'; fi
echo "--- after stop/cold copy NEXT=$NEXT $(date -u +%FT%TZ)"
# ---- core section 5 (verbatim)
if test "$NEXT" = CAS && bash -eu <<'SH'
python3 -I - <<'PY'
import os,pathlib,subprocess,json
p=pathlib.Path(os.environ['WS']);path=pathlib.Path(os.environ['COMPOSE']);before=(p/'compose.before.yml').read_bytes();after=(p/'compose.candidate.yml').read_bytes()
v=json.loads(subprocess.check_output(['docker','inspect',(p/'old-stopped').read_text()],text=True))[0]
assert not v['State']['Running'] and not v['State']['Paused'] and not v['State'].get('Restarting') and v['HostConfig']['RestartPolicy']['Name']=='no'
assert (p/'cold-backup-path').exists() and path.read_bytes()==before,'STOP foreign Compose drift'
tmp=path.with_name('.docker-compose.ws0054.next')
with tmp.open('xb') as out:out.write(after);out.flush();os.fsync(out.fileno())
tmp.chmod(path.stat().st_mode&0o777);assert path.read_bytes()==before;tmp.replace(path)
PY
test "$(docker image inspect "$NEW_TAG" --format '{{.Id}}')" = "$NEW_IMAGE"
test "$(docker image inspect "$OLD_TAG" --format '{{.Id}}')" = "$OLD_IMAGE"
date -u +%Y-%m-%dT%H:%M:%SZ > "$WS/candidate-may-have-run"
docker compose -f "$COMPOSE" -p cpa-home up -d --no-deps --force-recreate --pull never home
# Candidate has started. Paste Annex C promptly in another Ark shell to observe
# coordinator v8 catalog within10min; missing6.2 is NONBLOCKING, never rollback.
# Verification is low-priority and never a pre-start delay. If heartbeat/IO
# telemetry shows pressure, stagger these three jobs until nodes reconnect;
# do not waive any of their final PASS gates.
BACKUP=$(cat "$WS/cold-backup-path")
(
  if ionice -c 3 nice -n 19 timeout 1800 sqlite3 -readonly "file:$BACKUP?mode=ro" 'PRAGMA quick_check;' > "$WS/cold-quick.output" && test "$(cat "$WS/cold-quick.output")" = ok; then
    printf 'PASS\n' > "$WS/cold-quick.status"
  else printf 'FAIL\n' > "$WS/cold-quick.status"; fi
) &
printf '%s\n' "$!" > "$WS/cold-quick.pid"
(
  if ionice -c 3 nice -n 19 timeout 1800 sha256sum "$BACKUP" > "$BACKUP.sha256" && ionice -c 3 nice -n 19 timeout 1800 sha256sum -c "$BACKUP.sha256" > "$WS/cold-sha.output"; then
    printf 'PASS\n' > "$WS/cold-sha.status"
  else printf 'FAIL\n' > "$WS/cold-sha.status"; fi
) &
printf '%s\n' "$!" > "$WS/cold-sha.pid"
(
  if ionice -c 3 nice -n 19 timeout 1800 sqlite3 -readonly "file:$BACKUP?mode=ro" 'PRAGMA integrity_check;' > "$WS/cold-full.output" && test "$(cat "$WS/cold-full.output")" = ok; then
    printf 'PASS\n' > "$WS/cold-full.status"
  else printf 'FAIL\n' > "$WS/cold-full.status"; fi
) &
printf '%s\n' "$!" > "$WS/cold-full.pid"
SH
then NEXT=ACCEPT; ACCEPT_TARGET=forward; export ACCEPT_TARGET; else NEXT=RECOVERY; printf '%s\n' 'STOP CAS/recreate failure: route B; never infer candidate did not run.'; fi
echo "--- after recreate NEXT=$NEXT $(date -u +%FT%TZ)"
if test "$NEXT" = ACCEPT; then
# ---- Annex C (verbatim, nonblocking)
(
set +e
set +x
umask 077
if bash -eu <<'SH'
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
python3 -I - <<'PY'
import datetime,json,os,pathlib,time,urllib.request
p=pathlib.Path(os.environ['WS']);start=datetime.datetime.fromisoformat((p/'candidate-may-have-run').read_text().strip().replace('Z','+00:00')).timestamp();deadline=start+600
op=urllib.request.build_opener(urllib.request.ProxyHandler({}));missing={'gpt-6.2-astra','gpt-6.2-sol'}
while time.time()<deadline:
    try:
        req=urllib.request.Request('http://100.110.133.6:8327/v8/management/models?scope=available',headers={'Authorization':'Bearer '+os.environ['MANAGEMENT_PASSWORD']})
        with op.open(req,timeout=min(15,max(.1,deadline-time.time()))) as response:v=json.load(response)
        assert v['scope']=='available' and isinstance(v['models'],list)
        missing={'gpt-6.2-astra','gpt-6.2-sol'}-{x['id'] for x in v['models']}
        if not missing: print('coordinator_v8_catalog_both_6.2_within10min=PASS');break
    except Exception: pass # Nonblocking observation only; never log upstream bodies or auth.
    time.sleep(min(15,max(0,deadline-time.time())))
else: print('NONBLOCKING_KNOWN_STARTUP_FETCH_ISSUE; recheck_next3hour_refresh; NOT_ROLLBACK_TRIGGER; missing='+','.join(sorted(missing)))
PY
SH
then :
else printf '%s\n' 'NONBLOCKING catalog observation failed; record/recheck next3hour refresh; NOT rollback trigger.'; fi
) > "$WS/coordinator-catalog-observation.txt" 2>&1 &
printf '%s\n' "$!" > "$WS/coordinator-catalog-observation.pid"
fi
# ---- core section 6 (verbatim)
case "$ACCEPT_TARGET" in
  forward) ACCEPT_IMAGE=$NEW_IMAGE; ACCEPT_SCHEMA_FILE=schema-after-candidate.json; ACCEPT_LABEL=forward; ACCEPT_SECONDS=$READY_SECONDS ;;
  rollback) ACCEPT_IMAGE=$OLD_IMAGE; ACCEPT_LABEL=rollback; ACCEPT_SECONDS=$ROLLBACK_SECONDS; ACCEPT_SCHEMA_FILE=rollback-schema.json ;;
  *) NEXT=RECOVERY; printf '%s\n' 'STOP: no accepted forward/rollback target.' ;;
esac
export ACCEPT_IMAGE ACCEPT_SCHEMA_FILE ACCEPT_LABEL ACCEPT_SECONDS
if test "$NEXT" = ACCEPT && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
export READINESS_DEADLINE=$(($(date +%s)+ACCEPT_SECONDS))
while :; do
  REMAINING=$((READINESS_DEADLINE-$(date +%s)))
  test "$REMAINING" -gt 0 || exit 1
  if timeout --signal=KILL "$REMAINING" bash -eu -c 'verify_home ready'; then
    test "$(date +%s)" -lt "$READINESS_DEADLINE" || exit 1
    break
  fi
  test "$(date +%s)" -lt "$READINESS_DEADLINE" || exit 1
  # Crash loops/paused/foreign image are NOT tolerated until a timeout; inspect and route B.
  test "$(docker inspect cpa-home --format '{{.RestartCount}} {{.State.Paused}}')" = '0 false' || exit 1
  sleep 5
done
SH
then NEXT=NATIVE; else NEXT=RECOVERY; printf '%s\n' 'STOP readiness: route B/C; locks remain owned.'; fi
echo "--- after readiness NEXT=$NEXT $(date -u +%FT%TZ)"
if test "$NEXT" = RECOVERY; then recover; fi
save;;
finish)
. "$STATE"; export WS ACCEPT_TARGET ACCEPT_LABEL
echo "=== finish start target=$ACCEPT_TARGET $(date -u +%FT%TZ)"
# ---- core section 6 target selection only (verbatim first lines)
case "$ACCEPT_TARGET" in
  forward) ACCEPT_IMAGE=$NEW_IMAGE; ACCEPT_SCHEMA_FILE=schema-after-candidate.json; ACCEPT_LABEL=forward; ACCEPT_SECONDS=$READY_SECONDS ;;
  rollback) ACCEPT_IMAGE=$OLD_IMAGE; ACCEPT_LABEL=rollback; ACCEPT_SECONDS=$ROLLBACK_SECONDS; ACCEPT_SCHEMA_FILE=rollback-schema.json ;;
  *) NEXT=RECOVERY; printf '%s\n' 'STOP: no accepted forward/rollback target.' ;;
esac
export ACCEPT_IMAGE ACCEPT_SCHEMA_FILE ACCEPT_LABEL ACCEPT_SECONDS
# ---- core section 6b (verbatim)
if test "$NEXT" = NATIVE && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
verify_native
verify_home stable # separate ALL-account/key/parent-group/config/history/schema/full-panel gate
if test "$ACCEPT_LABEL" = forward; then
  END=$(($(date +%s)+1800))
  while :; do
    DONE=1
    for CHECK in quick sha full; do
      STATUS=$(cat "$WS/cold-$CHECK.status" 2>/dev/null || true)
      case "$STATUS" in PASS) ;; '') DONE=0 ;; *) exit 1 ;; esac
    done
    test "$DONE" = 0 || break
    test "$(date +%s)" -lt "$END" || exit 1
    sleep 5
  done
else
  # Failed/partial COLD never counts; keep HOT and prior verified backup.
  hot_backup verify # SAME actual retained-disaster-copy contract; no cold PASS substitution
fi
SH
then NEXT=RESTORE; else NEXT=RECOVERY; printf '%s\n' 'STOP native/preservation/backup: route B/C; locks remain owned.'; fi
echo "--- after native/stable/backups NEXT=$NEXT $(date -u +%FT%TZ)"
# ---- Annex S (verbatim)
if test "$NEXT" = RESTORE && bash -eu <<'SH'
set +x
set -a; . /mnt/user/appdata/cpa-home/.env; set +a
export MANAGEMENT_PASSWORD
if test "$ACCEPT_LABEL" != abort-old; then
  verify_native # exact real JSON/exit/boundary contract, not a bare ok file
  verify_home stable # separate preservation/full panel; abort-old was positively unchanged
fi
python3 -I - <<'PY'
import os,json,pathlib,time,urllib.request,urllib.error
p=pathlib.Path(os.environ['WS']);saved=json.loads((p/'policy-before.json').read_text());target=dict(saved,patch_updates=False)
op=urllib.request.build_opener(urllib.request.ProxyHandler({}));base='http://100.110.133.6:8327/v8/management'
def api(path,method='GET',body=None):
    req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),method=method,headers={'Authorization':'Bearer '+os.environ['MANAGEMENT_PASSWORD'],'Content-Type':'application/json'})
    with op.open(req,timeout=15) as r:return json.load(r)
end=time.monotonic()+180
while True:
    assert time.monotonic()<end,'STOP restore timeout; keep locks'
    f=api('/fleet')
    assert f['policy'] in (dict(saved,enabled=False,patch_updates=False),target),'STOP foreign policy drift'
    if f.get('active') is not None:time.sleep(2);continue
    try:api('/fleet/policy','PUT',target)
    except urllib.error.HTTPError:time.sleep(2);continue
    if api('/fleet')['policy']==target:break
    time.sleep(2)
print('saved_policy_exact_readback_patch_updates_false=PASS')
PY
SH
then NEXT=CLOSEOUT; else NEXT=RECOVERY; printf '%s\n' 'STOP restore unverified; keep lock ownership and reconcile policy.'; fi
echo "--- after policy restore NEXT=$NEXT $(date -u +%FT%TZ)"
if test "$NEXT" = RECOVERY; then
  if test "$ACCEPT_TARGET" = forward; then recover; else echo 'C: rollback acceptance failed; stay stopped/escalate'; fi
fi
save;;
*) echo usage; exit 2;;
esac
touch "$STATE.done-$PHASE"
