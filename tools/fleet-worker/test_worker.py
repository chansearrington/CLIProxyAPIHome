import importlib.util
from pathlib import Path
import json
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import worker
import catalog_sync

class WorkerTests(unittest.TestCase):
    def test_cached_receipt_reports_without_reexecuting_a_job(self):
        job={'id':'00000000-0000-4000-8000-000000000001','token':'fixture-token','operation':'catalog'}
        with tempfile.TemporaryDirectory() as directory:
            state=Path(directory);receipt={'id':job['id'],'token':job['token'],'status':'success'}
            (state/(job['id']+'.json')).write_text(json.dumps(receipt))
            with patch.object(worker,'inventory',return_value={}),patch.object(worker,'rpc',side_effect=[{'job':job},1]),patch.object(worker,'execute_job') as execute:
                result=worker.run({},state)
            execute.assert_not_called();self.assertTrue(result['ok']);self.assertNotIn('token',result)

    def test_job_failure_is_redacted_and_receipt_survives_failed_acknowledgement(self):
        job={'id':'00000000-0000-4000-8000-000000000001','token':'fixture-token','operation':'catalog'}
        with tempfile.TemporaryDirectory() as directory:
            state=Path(directory)
            with patch.object(worker,'inventory',return_value={}),patch.object(worker,'rpc',side_effect=[{'job':job},OSError('fixture')]),patch.object(worker,'execute_job',side_effect=RuntimeError('fake-sensitive-upstream-body')):
                with self.assertRaises(OSError):worker.run({},state)
            result=json.loads((state/(job['id']+'.json')).read_text());self.assertEqual(result['error'],'RuntimeError');self.assertNotIn('fake-sensitive',json.dumps(result))

    def test_private_provider_catalog_and_readiness_probe_are_excluded(self):
        ids=catalog_sync.catalog_ids({'data':[{'id':'shared'},{'id':'copilot/private'},{'id':'agent-os-home-canary'}]})
        self.assertEqual(ids,['shared'])

    def test_catalog_sync_preserves_operator_defaults_and_overrides(self):
        config={'models':{'providers':{'cpa-gui':{'baseUrl':'http://127.0.0.1:18317/v1','models':[{'id':'old','name':'Old'}]}}},'agents':{'defaults':{'model':{'primary':'cpa-gui/old'},'modelPolicy':{'allow':['cpa-gui/*']},'models':{'cpa-gui/old':{'alias':'Keep'}}},'entries':{'main':{'model':{'primary':'cpa-gui/old'}}}}}
        after,added=catalog_sync.updated_config(config,['new'])
        self.assertEqual(added,['new']);self.assertEqual(config['agents']['defaults']['model'],after['agents']['defaults']['model']);self.assertEqual(config['agents']['entries'],after['agents']['entries']);self.assertEqual(after['agents']['defaults']['models']['cpa-gui/old'],{'alias':'Keep'})

    def test_unsupported_home_operation_is_not_a_command_executor(self):
        job={'id':'fixture','operation':'shell','command':'fake-forbidden-command'}
        class Lease:
            def __enter__(self):return self
            def __exit__(self,*_):pass
            def check(self):pass
        with patch.object(worker,'Lease',return_value=Lease()):
            with self.assertRaisesRegex(worker.WorkerError,'unsupported_home_operation'):
                worker.execute_job({},job,Path('/fixture'))

    def test_post_update_quota_is_failed_acceptance_not_safe_precheck_block(self):
        with patch.object(worker,'probe',side_effect=worker.Blocked('upstream_quota')):
            with self.assertRaises(worker.WorkerError) as error:worker.post_upgrade_probe({})
        self.assertNotIsInstance(error.exception,worker.Blocked)
        self.assertEqual(str(error.exception),'post_update_quota_unverified')

    def test_home_acceptance_receipt_is_retained_for_enrollment_without_secrets(self):
        plan={'status':'current','accepted_release':{'version':'8.0.30','sha256':'a'*64,'size':100},'verified_node_ids':['fixture-node']}
        with tempfile.TemporaryDirectory() as directory:
            state=Path(directory)
            with patch.object(worker,'inventory',return_value={}),patch.object(worker,'rpc',return_value=plan):
                report=worker.run({},state)
            self.assertTrue(report['ok'])
            receipt=json.loads((state/'accepted-core.json').read_text())
            self.assertEqual(receipt['schema'],'home-fleet-accepted-core/v1')
            self.assertEqual(receipt['version'],'8.0.30')
            self.assertNotIn('token',receipt)

    def test_resumed_update_cannot_acknowledge_new_file_without_verified_restart_manifest(self):
        release={'version':'8.0.31','sha256':'b'*64,'size':100}
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            (root/'install-manifest.json').write_text(json.dumps({'core_version':'8.0.30','core_sha256':'a'*64}))
            with patch.object(worker,'inventory',return_value=release):
                with self.assertRaisesRegex(worker.WorkerError,'installed_binary_needs_reconciliation'):
                    worker.install_binary({'service_root':str(root)},{'operation':'upgrade','release':release},root,None)

if __name__=='__main__':unittest.main()
