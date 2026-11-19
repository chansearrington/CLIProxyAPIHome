#!/usr/bin/env python3
"""Hermetic snapshot checks: no host /proc, status command or build-root writes."""
import ast
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

SOURCE = Path(__file__).with_name('ws0054-io-status.py')
STATUS = '''sbSynced=1794737538
sbSyncErrs=0
mdState=STARTED
mdNumDisabled=0
mdNumInvalid=0
mdResyncAction=check P Q
mdResyncSize=15625879500
mdResyncCorr=0
mdResync=0
mdResyncPos=0
mdResyncDt=0
mdResyncDb=0
'''


class SnapshotTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        tree = ast.parse(SOURCE.read_text())
        # Never import the old Linux-at-import script on this Mac.
        if not any(isinstance(node, ast.If) and ast.unparse(node.test) ==
                   "__name__ == '__main__'" for node in tree.body):
            raise AssertionError('snapshot must be import-safe before tests load it')
        spec = importlib.util.spec_from_file_location('io_status', SOURCE)
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ('proc', 'cron'):
            (self.root / name).mkdir()
        self.paths = {'/proc': self.root / 'proc',
                      '/var/run/mover.pid': self.root / 'pid',
                      '/etc/cron.d': self.root / 'cron'}
        self.path_patch = mock.patch.object(
            self.module, 'pathlib', mock.Mock(Path=lambda value: self.paths.get(str(value), Path(value))))
        self.path_patch.start()
        self.addCleanup(self.path_patch.stop)
        self.run = mock.patch.object(self.module.subprocess, 'run').start()
        self.addCleanup(mock.patch.stopall)
        self.run.return_value = subprocess.CompletedProcess([], 0, STATUS, 'fake-stderr-not-exported')

    def process(self, argv=b'/bin/bash\0/usr/local/sbin/mover\0'):
        proc = self.root / 'proc' / '123'
        proc.mkdir()
        (proc / 'cmdline').write_bytes(argv)
        return proc

    def test_optional_exe_disappearance_keeps_positive_mover(self):
        proc = self.process()
        with mock.patch.object(self.module.os, 'readlink', side_effect=FileNotFoundError):
            result = self.module.snapshot()
        self.assertTrue(proc.exists())
        self.assertFalse(result['mover_idle'])
        self.assertEqual(result['mover_processes'], [
            {'pid': 123, 'executable_basename': None, 'script_basenames': ['mover']}])

    def test_exact_names_only_and_no_self_match(self):
        for name in ('mover', 'mover.old', 'mover.php', 'mover.tuning', 'age_mover', 'move'):
            with self.subTest(name=name):
                proc = self.process(b'/bin/bash\0/usr/local/sbin/' + name.encode() + b'\0')
                with mock.patch.object(self.module.os, 'readlink', return_value='/bin/bash'):
                    self.assertFalse(self.module.snapshot()['mover_idle'])
                (proc / 'cmdline').unlink()
                proc.rmdir()
        self.process(b'/bin/python3\0/ws0054-io-status.py\0/not-a-mover\0')
        self.assertTrue(self.module.snapshot()['mover_idle'])

    def test_cmdline_race_is_unknown_until_process_is_confirmed_gone(self):
        proc = self.process()
        (proc / 'cmdline').unlink()
        self.assertIsNone(self.module.snapshot()['mover_idle'])
        real_stat = Path.stat
        def gone(path, *args, **kwargs):
            if path == proc:
                raise FileNotFoundError()
            return real_stat(path, *args, **kwargs)
        with mock.patch.object(Path, 'stat', gone):
            self.assertTrue(self.module.snapshot()['mover_idle'])

    def test_unreadable_process_is_unknown(self):
        proc = self.process()
        real_read = Path.read_bytes
        def denied(path):
            if path == proc / 'cmdline':
                raise PermissionError('fake-sensitive-error')
            return real_read(path)
        with mock.patch.object(Path, 'read_bytes', denied):
            result = self.module.snapshot()
        self.assertIsNone(result['mover_idle'])
        self.assertEqual(result['unreadable_process_count'], 1)
        self.assertNotIn('fake-sensitive-error', json.dumps(result))

    def test_pidfile_numeric_bounded_and_live(self):
        (self.root / 'proc' / '123').mkdir()
        (self.root / 'proc' / '123' / 'cmdline').write_bytes(b'')
        for value in ('123\n', '0', '-1', '9' * 17, '2147483648', 'fake'):
            with self.subTest(value=value):
                (self.root / 'pid').write_text(value)
                result = self.module.snapshot()
                if value == '123\n':
                    self.assertFalse(result['mover_idle'])
                    self.assertTrue(result['mover_pidfile']['live'])
                else:
                    self.assertIsNone(result['mover_idle'])

    def test_canonical_mdcmd_ignores_path_and_action_alone_is_not_activity(self):
        with mock.patch.dict(os.environ, {'PATH': '/synthetic/untrusted-PATH'}):
            result = self.module.snapshot()
        self.run.assert_called_once_with(
            ['/usr/local/sbin/mdcmd', 'status'], capture_output=True, text=True, timeout=15)
        self.assertEqual(result['mdcmd_path'], '/usr/local/sbin/mdcmd')
        self.assertTrue(result['parity_idle'])
        self.assertIsNone(result['parity_error'])
        self.assertEqual(result['parity_status_fields']['mdResyncAction'], 'check P Q')
        self.run.return_value.stdout = STATUS.replace('mdResync=0', 'mdResync=1')
        self.assertFalse(self.module.snapshot()['parity_idle'])

    def test_status_failures_are_unknown_and_sanitized(self):
        failures = [(FileNotFoundError('fake-sensitive-error'), 'missing'),
                    (PermissionError('fake-sensitive-error'), 'unreadable'),
                    (subprocess.TimeoutExpired('fake-sensitive-error', 15,
                                               output='fake-sensitive-error', stderr='fake-sensitive-error'), 'timeout'),
                    (OSError('fake-sensitive-error'), 'exec_failed'),
                    (UnicodeDecodeError('ascii', b'\xff', 0, 1, 'fake-sensitive-error'), 'malformed_status')]
        for error, code in failures:
            with self.subTest(code=code):
                self.run.side_effect = error
                result = self.module.snapshot()
                self.assertIsNone(result['parity_idle'])
                self.assertEqual(result['parity_error'], code)
                self.assertEqual(result['parity_status_fields'], {})
                self.assertNotIn('fake-sensitive-error', json.dumps(result))
        self.run.side_effect = None
        self.run.return_value = subprocess.CompletedProcess([], 1, STATUS, 'fake-sensitive-error')
        result = self.module.snapshot()
        self.assertIsNone(result['parity_idle'])
        self.assertEqual(result['parity_error'], 'nonzero')
        self.assertEqual(result['parity_status_fields'], {})
        self.assertNotIn('fake-sensitive-error', json.dumps(result))

    def test_exact_parity_fields_numeric_validation_and_bounded_action(self):
        self.run.return_value.stdout = STATUS + 'mdResyncPrivate=fake-sensitive-error\n'
        result = self.module.snapshot()
        self.assertTrue(result['parity_idle'])
        self.assertNotIn('mdResyncPrivate', result['parity_status_fields'])
        for bad in ('', STATUS.replace('mdResync=0\n', ''),
                    STATUS.replace('mdResync=0', 'mdResync=bad'),
                    STATUS.replace('mdResync=0', 'mdResync=-1'),
                    STATUS.replace('mdResync=0', 'mdResync=٠'),
                    STATUS.replace('mdResync=0', 'mdResync=' + '9' * 21),
                    STATUS.replace('mdNumInvalid=0', 'mdNumInvalid=bad'),
                    STATUS.replace('mdState=STARTED', 'mdState=bad'),
                    STATUS.replace('check P Q', 'x' * 65),
                    STATUS.replace('check P Q', 'check\tP'),
                    STATUS + 'mdResync=0\n'):
            with self.subTest(status=bad):
                self.run.return_value.stdout = bad
                result = self.module.snapshot()
                self.assertIsNone(result['parity_idle'])
                self.assertEqual(result['parity_error'], 'malformed_status')

    def test_cron_exports_only_schedule_not_command(self):
        (self.root / 'cron' / 'root').write_text(
            '# no schedule\n0 */1 * * * /usr/local/sbin/mover fake-command-data\n'
            '0 2 * * * /usr/local/emhttp/plugins/parity.check.tuning/check\n')
        result = self.module.snapshot()
        self.assertEqual(result['cron_schedules'], [
            {'file': 'root', 'schedule_fields': ['0', '*/1', '*', '*', '*'], 'kind': 'mover'},
            {'file': 'root', 'schedule_fields': ['0', '2', '*', '*', '*'], 'kind': 'parity-check-tuning'}])
        self.assertNotIn('fake-command-data', json.dumps(result))
        self.assertFalse(result['production_writes'])

    def test_main_keeps_buildroot_file_and_stdout_contract(self):
        result = {'synthetic': True, 'production_writes': False}
        output = io.StringIO()
        with mock.patch.object(self.module, 'snapshot', return_value=result), \
                mock.patch.object(Path, 'write_text') as write, contextlib.redirect_stdout(output):
            self.module.main()
        write.assert_called_once_with(json.dumps(result, indent=2))
        self.assertEqual(json.loads(output.getvalue()), result)


if __name__ == '__main__':
    unittest.main()
