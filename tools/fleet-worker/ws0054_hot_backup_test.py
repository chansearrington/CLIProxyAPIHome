#!/usr/bin/env python3
"""Owned temporary fixtures only. macOS never calls chroot or production main."""
import contextlib
import importlib.util
import io
import json
import os
import pathlib
import sqlite3
import subprocess
import tempfile
import types
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('hot_backup', pathlib.Path(__file__).with_name('ws0054-hot-backup.py'))
hot = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(hot)
CACHE = {'mount': '/mnt/cache', 'filesystem': 'btrfs', 'device': '/dev/nvme1n1p1',
         'major_minor': f'{os.major(44)}:{os.minor(44)}'}
ARRAY = {'mount': '/mnt/disk18', 'filesystem': 'xfs', 'device': '/dev/md18p1',
         'major_minor': f'{os.major(2322)}:{os.minor(2322)}'}


def synthetic_bytes(value=42):
    db = sqlite3.connect(':memory:')
    try:
        db.execute('CREATE TABLE auth(value INTEGER)')
        db.execute('INSERT INTO auth VALUES(?)', (value,))
        db.commit()
        return db.serialize()
    finally:
        db.close()


class HotBackupTests(unittest.TestCase):
    @contextlib.contextmanager
    def fixtures(self):
        # Realistic public ancestor/private children, all owned disposable test data.
        with tempfile.TemporaryDirectory() as folder, contextlib.ExitStack() as stack:
            parent = pathlib.Path(folder).resolve()
            parent.chmod(0o777)
            cache, array = parent / 'build', parent / 'array'
            source_parent = parent / 'source'
            cache.mkdir(mode=0o755)
            array.mkdir(mode=0o700)
            source_parent.mkdir(mode=0o700)
            data = source_parent / 'data'
            data.mkdir(mode=0o700)
            (data / 'home.db').write_bytes(synthetic_bytes())
            array_inodes = {array.stat().st_ino}
            real_open, real_stat, real_fstat = os.open, os.stat, os.fstat
            mask = os.umask(0o077)
            stack.callback(os.umask, mask)

            def facts(s):
                values = list(s)
                values[2] = 2322 if s.st_ino in array_inodes else 44
                values[4] = 0
                return os.stat_result(values)

            def opened(path, flags, *args, **kwargs):
                fd = real_open(path, flags, *args, **kwargs)
                dir_fd = kwargs.get('dir_fd')
                if (dir_fd is not None and real_fstat(dir_fd).st_ino in array_inodes) or str(path) == 'array':
                    array_inodes.add(real_fstat(fd).st_ino)
                return fd

            stack.enter_context(mock.patch.object(hot.os, 'open', side_effect=opened))
            stack.enter_context(mock.patch.object(hot.os, 'stat', side_effect=lambda *a, **kw: facts(real_stat(*a, **kw))))
            stack.enter_context(mock.patch.object(hot.os, 'fstat', side_effect=lambda fd: facts(real_fstat(fd))))
            yield types.SimpleNamespace(parent=parent, cache=cache, array=array, data=data,
                                        base=cache / 'ws0054-hot-synthetic')

    @contextlib.contextmanager
    def synthetic_run(self, mode='success'):
        with self.fixtures() as f, contextlib.ExitStack() as stack:
            proof = {'stage_dev': 44, 'source_dev': 44, 'destination_dev': 2322,
                     'different_devices': True, 'cache_mount': CACHE, 'destination_mount': ARRAY}

            def backup(source, base, fd, result):
                hot.write_all(fd, synthetic_bytes(), 0)
                result.update(copy_exit=0, wal_guard_active_before_begin=True, abort_reason=None,
                              source_pin={'same_direct_directory': True})

            def worker(callback, bound, observe=None):
                if bound == 30:
                    return {'pin_selfcheck_ok': True}
                if bound == 1800:
                    return callback()
                self.assertEqual(bound, 900)
                directory = callback.__closure__[0].cell_contents
                # FULL check uses the real in-memory SQLite engine on held-fd bytes.
                db = sqlite3.connect(':memory:')
                try:
                    db.deserialize(os.pread(directory.files['home-hot.db'], 1024 * 1024, 0))
                    rows = db.execute('PRAGMA integrity_check').fetchall()
                finally:
                    db.close()
                hot.require(mode != 'integrity_failure' and rows == [('ok',)], 'full_integrity_failed')
                return {'integrity': ['ok'], 'integrity_exit': 0}

            for name, value in [('LIVE', f.data / 'home.db'), ('CACHE_ROOT', f.cache), ('ARRAY_ROOT', f.array),
                                ('fresh_base', lambda raw: f.base), ('device_proof', lambda *a: proof),
                                ('cache_headroom', lambda *a: {'pass': True}),
                                ('trusted_tool', lambda name: str(pathlib.Path(hot.sys.executable).resolve())),
                                ('guarded_backup', backup), ('run_worker', worker)]:
                stack.enter_context(mock.patch.object(hot, name, value))
            stack.enter_context(mock.patch.object(hot.sys, 'platform', 'linux'))
            stack.enter_context(mock.patch.object(hot.os, 'geteuid', return_value=0))
            yield f, f.array / f.base.name

    def test_fresh_base_rejects_alias_traversal_existing_base_and_destination(self):
        good = '/mnt/cache/appdata/cpa-home-build/ws0054-hot-20261119T1200Z'
        with mock.patch.object(hot.os.path, 'lexists', return_value=False):
            self.assertEqual(hot.fresh_base(good), pathlib.Path(good))
            for bad in ('relative', good + '/', good + '/child', good + ';cmd',
                        good.replace('/mnt/cache', '/mnt/user'), good.replace('hot-', '../hot-')):
                with self.subTest(bad=bad), self.assertRaises(hot.Rejected):
                    hot.fresh_base(bad)
        for states in ((True,), (False, True)):
            with mock.patch.object(hot.os.path, 'lexists', side_effect=states), self.assertRaises(hot.Rejected):
                hot.fresh_base(good)

    def test_public_ancestor_private_child_and_exclusive_no_follow_files(self):
        with self.fixtures() as f:
            root = hot.Directory(f.array)
            child = hot.Directory(f.array / 'new', root)
            try:
                fd = child.open_file('ordinary', create=True)
                hot.write_all(fd, b'synthetic', 0)
                self.assertEqual(os.pread(fd, 20, 0), b'synthetic')
                self.assertEqual(child.check_file('ordinary').st_mode & 0o777, 0o600)
                self.assertEqual(os.fstat(child.fd).st_mode & 0o777, 0o700)
                with self.assertRaises(hot.Rejected):
                    child.open_file('ordinary', create=True)
                (child.path / 'alias').symlink_to('ordinary')
                with self.assertRaises(OSError):
                    child.open_file('alias', create=True)
                with self.assertRaises(OSError):
                    child.open_file('alias')
                self.assertEqual(os.pread(fd, 20, 0), b'synthetic')
            finally:
                child.close()
                root.close()

    def test_symlink_directory_and_writable_final_root_rejected_without_chmod(self):
        with self.fixtures() as f:
            alias = f.parent / 'alias'
            alias.symlink_to(f.array, target_is_directory=True)
            with self.assertRaises(OSError):
                hot.Directory(alias)
            f.array.chmod(0o777)
            with self.assertRaisesRegex(hot.Rejected, 'untrusted_directory'):
                hot.Directory(f.array)
            self.assertEqual(f.array.stat().st_mode & 0o777, 0o777)

    def test_held_directory_identity_drift_rejects_and_keeps_own_open_file(self):
        with self.fixtures() as f:
            directory = hot.Directory(f.array)
            fd = directory.open_file('ordinary', create=True)
            try:
                f.array.rename(f.parent / 'moved')
                f.array.mkdir(mode=0o700)
                with self.assertRaisesRegex(hot.Rejected, 'directory_identity_changed'):
                    directory.verify()
                hot.write_all(fd, b'owned synthetic bytes', 0)
                self.assertEqual((f.parent / 'moved' / 'ordinary').read_bytes(), b'owned synthetic bytes')
                self.assertEqual(list(f.array.iterdir()), [])
            finally:
                directory.close()

    def test_metadata_never_overwrites_existing_file_or_follows_receipt_symlink(self):
        for alias in (False, True):
            with self.subTest(alias=alias), self.fixtures() as f:
                directory = hot.Directory(f.array)
                try:
                    if alias:
                        (f.array / 'ordinary').write_bytes(b'owned existing bytes')
                        (f.array / 'hot-backup.json').symlink_to('ordinary')
                    else:
                        (f.array / 'hot-backup.json').write_bytes(b'owned existing bytes')
                    with self.assertRaisesRegex(hot.Rejected, 'result_exists'):
                        hot.write_result(directory, {'ok': False})
                    self.assertEqual((f.array / ('ordinary' if alias else 'hot-backup.json')).read_bytes(),
                                     b'owned existing bytes')
                finally:
                    directory.close()

    def test_source_sidecar_symlinks_rejected_before_sqlite(self):
        for name in ('home.db-wal', 'home.db-shm'):
            with self.subTest(name=name), self.fixtures() as f:
                (f.data / name).symlink_to('home.db')
                directory = hot.Directory(f.data)
                try:
                    with self.assertRaises(OSError):
                        hot.Source(directory)
                finally:
                    directory.close()

    def test_wrong_source_and_untrusted_tool_rejected(self):
        with mock.patch.object(hot, 'LIVE', pathlib.Path('/mnt/user/appdata/cpa-home/data/home.db')):
            with self.assertRaisesRegex(hot.Rejected, 'wrong_source'):
                hot.device_proof(None, None, None)
        with mock.patch.object(hot.pathlib.Path, 'exists', return_value=True), \
                mock.patch.object(hot.pathlib.Path, 'resolve', return_value=pathlib.Path('/usr/bin/python3')), \
                mock.patch.object(hot.pathlib.Path, 'stat', return_value=types.SimpleNamespace(st_uid=501, st_mode=0o755)):
            with self.assertRaisesRegex(hot.Rejected, 'untrusted_tool'):
                hot.trusted_tool('python3')

    def test_devices_require_exact_positive_mount_proof(self):
        self.assertTrue(hot.check_devices(44, 44, 2322, CACHE, ARRAY)['different_devices'])
        for args in ((44, 44, 44, CACHE, ARRAY), (44, 45, 2322, CACHE, ARRAY),
                     (44, 44, 2322, dict(CACHE, filesystem='fuse.shfs'), ARRAY),
                     (44, 44, 2322, CACHE, dict(ARRAY, device='/dev/md17p1')),
                     (44, 44, 2322, CACHE, dict(ARRAY, filesystem='btrfs'))):
            with self.subTest(args=args), self.assertRaises(hot.Rejected):
                hot.check_devices(*args)
        mounts = ('1 0 0:44 / /mnt/cache rw - btrfs /dev/nvme1n1p1 rw\n'
                  '2 0 9:18 / /mnt/disk18 rw - xfs /dev/md18p1 rw\n'
                  '3 1 0:99 / /mnt/cache/appdata rw - fuse.shfs shfs rw\n')
        self.assertEqual(hot.mount_for(hot.LIVE, mounts)['filesystem'], 'fuse.shfs')

    def test_headroom_reserves_hot_and_cold_at_larger_actual_size(self):
        gib = 1024 ** 3
        self.assertEqual(hot.headroom(1, 1, 1000 * gib, 500 * gib)['reserve_bytes'], 90 * gib)
        self.assertEqual(hot.headroom(60 * gib, 2 * gib, 1000 * gib, 500 * gib)['reserve_bytes'], 124 * gib)
        with self.assertRaisesRegex(hot.Rejected, 'cache_headroom'):
            hot.headroom(1, 1, 1000 * gib, 289 * gib)
        self.assertTrue(hot.headroom(1, 1, 1000 * gib, 290 * gib)['pass'])

    def guard_case(self, clocks, wals):
        events, result = [], {}
        source = types.SimpleNamespace(walbytes=lambda: events.append('wal') or next(wals))

        def clock():
            events.append('clock')
            return next(clocks)

        def worker(callback, bound, observe):
            self.assertTrue(result['wal_guard_active_before_begin'])
            events.append('worker')
            self.assertEqual(bound, 600)
            observe(0)
            return {'source_pin': {'same_direct_directory': True}}
        with mock.patch.object(hot.time, 'monotonic', side_effect=clock), \
                mock.patch.object(hot, 'run_worker', side_effect=worker):
            try:
                hot.guarded_backup(source, None, 123, result)
            except hot.Rejected:
                pass
        return result, events

    def test_guard_baseline_before_worker_and_both_abort_bounds(self):
        result, events = self.guard_case(iter((0, 0.1)), iter((10, 20)))
        self.assertEqual(events[:3], ['wal', 'clock', 'worker'])
        self.assertEqual(result['wal_sample_interval_seconds'], 0.1)
        self.assertEqual(result['copy_exit'], 0)
        cases = [((0, 600), (0, 0), 'copy_duration_over_600s'),
                 ((0, 0.1), (100, 100 + hot.WAL_LIMIT + 1), 'live_wal_growth_over_1GiB')]
        for times, sizes, expected in cases:
            with self.subTest(expected=expected):
                result, _ = self.guard_case(iter(times), iter(sizes))
                self.assertEqual(result['abort_reason'], expected)
                self.assertEqual(result['copy_exit'], 1)

    def test_stop_only_own_child_group_and_escalate(self):
        p = types.SimpleNamespace(pid=12345, poll=mock.Mock(return_value=None),
                                  wait=mock.Mock(side_effect=[subprocess.TimeoutExpired('synthetic', 10), 0]))
        with mock.patch.object(hot.os, 'killpg') as kill:
            hot.stop_own(p)
        self.assertEqual(kill.call_args_list, [mock.call(12345, hot.signal.SIGTERM), mock.call(12345, hot.signal.SIGKILL)])

    def test_success_retains_verified_copy_checksum_result_then_deletes_stage(self):
        with self.synthetic_run() as (f, root):
            result = hot.execute('synthetic-only')
            self.assertTrue(result['ok'], result)
            self.assertEqual(result['integrity'], ['ok'])
            self.assertEqual(result['integrity_exit'], 0)
            self.assertTrue(result['destination_match'])
            self.assertTrue(result['retained_copy'])
            self.assertFalse((f.base / 'home-hot.db').exists())
            self.assertEqual(result['stage_cleanup_state'], 'deleted_after_match')
            self.assertEqual((root / 'home-hot.db.sha256').read_text(), result['sha256'] + '  home-hot.db\n')
            self.assertTrue(json.loads((root / 'hot-backup.json').read_text())['ok'])
            self.assertTrue(json.loads((f.base / 'hot-backup.json').read_text())['ok'])
            self.assertEqual(root.stat().st_mode & 0o777, 0o700)
            self.assertEqual((root / 'home-hot.db').stat().st_mode & 0o777, 0o600)

    def test_copy_failure_keeps_verified_stage_and_partial_target(self):
        with self.synthetic_run() as (f, root):
            original = hot.write_all

            def write(fd, data, offset):
                if os.fstat(fd).st_dev == 2322:
                    raise OSError('synthetic I/O failure')
                original(fd, data, offset)
            with mock.patch.object(hot, 'write_all', side_effect=write):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertTrue(result['stage_verified'])
            self.assertEqual(result['stage_cleanup_state'], 'verified_retained')
            self.assertTrue((f.base / 'home-hot.db').exists())
            self.assertTrue((root / 'home-hot.db').exists())
            self.assertFalse(result['retained_copy'])
            self.assertFalse(json.loads((f.base / 'hot-backup.json').read_text())['ok'])

    def test_mismatch_keeps_verified_stage_and_no_false_ok(self):
        with self.synthetic_run() as (f, root):
            original = hot.sha256
            with mock.patch.object(hot, 'sha256', side_effect=lambda fd: '0' * 64 if os.fstat(fd).st_dev == 2322 else original(fd)):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertEqual(result['error'], 'destination_mismatch')
            self.assertTrue((f.base / 'home-hot.db').exists())
            self.assertTrue((root / 'home-hot.db').exists())

    def test_post_match_cleanup_failure_keeps_both_good_copies(self):
        with self.synthetic_run() as (f, root):
            with mock.patch.object(hot.Directory, 'remove', side_effect=OSError('synthetic private error')):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertEqual(result['error'], 'OSError')
            self.assertTrue(result['retained_copy'])
            self.assertTrue((f.base / 'home-hot.db').exists())
            self.assertTrue((root / 'home-hot.db').exists())
            self.assertFalse(json.loads((root / 'hot-backup.json').read_text())['ok'])

    def test_identity_drift_during_copy_fails_closed_preserves_owned_stage(self):
        with self.synthetic_run() as (f, root):
            original, changed = hot.write_all, False

            def write(fd, data, offset):
                nonlocal changed
                if os.fstat(fd).st_dev == 2322 and not changed:
                    changed = True
                    root.rename(f.array / 'moved-owned')
                    root.mkdir(mode=0o700)
                original(fd, data, offset)
            with mock.patch.object(hot, 'write_all', side_effect=write):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertTrue((f.base / 'home-hot.db').exists())
            self.assertEqual(list(root.iterdir()), [])
            self.assertTrue((f.array / 'moved-owned' / 'home-hot.db').exists())

    def test_postcommit_metadata_failure_keeps_retained_target_and_false_receipts(self):
        with self.synthetic_run() as (f, root):
            original, failed = hot.write_result, False

            def write(directory, result):
                nonlocal failed
                if directory.path == root and result['ok'] and not failed:
                    failed = True
                    raise OSError('synthetic metadata failure')
                original(directory, result)
            with mock.patch.object(hot, 'write_result', side_effect=write):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertEqual(result['error'], 'result_persist_failed')
            self.assertTrue((root / 'home-hot.db').exists())
            self.assertFalse(json.loads((f.base / 'hot-backup.json').read_text())['ok'])
            self.assertFalse(json.loads((root / 'hot-backup.json').read_text())['ok'])

    def test_ancestor_identity_drift_after_commit_fails_and_retains_good_target(self):
        with self.synthetic_run() as (f, root):
            original, changed = hot.write_result, False

            def write(directory, result):
                nonlocal changed
                original(directory, result)
                if directory.path == root and result['ok'] and not changed:
                    changed = True
                    f.array.rename(f.parent / 'moved-array')
                    f.array.mkdir(mode=0o700)
            with mock.patch.object(hot, 'write_result', side_effect=write):
                result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertEqual(result['error'], 'result_persist_failed')
            self.assertEqual(list(f.array.iterdir()), [])
            retained = f.parent / 'moved-array' / f.base.name / 'home-hot.db'
            self.assertTrue(retained.exists())
            self.assertEqual(retained.stat().st_size, result['bytes'])
            self.assertFalse(json.loads((f.base / 'hot-backup.json').read_text())['ok'])

    def test_preflight_existing_destination_and_full_integrity_reject(self):
        for gate in ('device_proof', 'cache_headroom'):
            with self.subTest(gate=gate), self.synthetic_run() as (f, root):
                with mock.patch.object(hot, gate, side_effect=hot.Rejected('synthetic_preflight')):
                    result = hot.execute('synthetic-only')
                self.assertFalse(result['ok'])
                self.assertFalse(f.base.exists())
                self.assertFalse(root.exists())
        with self.synthetic_run() as (f, root):
            root.mkdir(mode=0o700)
            (root / 'home-hot.db').write_bytes(b'existing owned synthetic backup')
            result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertEqual((root / 'home-hot.db').read_bytes(), b'existing owned synthetic backup')
            self.assertTrue((f.base / 'home-hot.db').exists())
        with self.synthetic_run('integrity_failure') as (f, root):
            result = hot.execute('synthetic-only')
            self.assertFalse(result['ok'])
            self.assertFalse(result['stage_verified'])
            self.assertFalse(root.exists())

    def test_sqlite_open_must_match_new_db_descriptor_before_any_write(self):
        db = mock.Mock()
        with mock.patch.object(hot, 'open_fds', side_effect=[{10: (44, 1)}, {10: (44, 1), 11: (44, 2)}]), \
                mock.patch.object(hot.os, 'fstat', return_value=types.SimpleNamespace(st_dev=44, st_ino=1)), \
                mock.patch.object(hot.sqlite3, 'connect', return_value=db):
            with self.assertRaisesRegex(hot.Rejected, 'sqlite_db_identity_mismatch'):
                hot.sqlite_open('file:/home-hot.db?mode=rw', 10)
        db.execute.assert_not_called()
        db.close.assert_called_once()

    def test_source_wal_identity_drift_rejects(self):
        with self.fixtures() as f:
            (f.data / 'home.db-wal').write_bytes(b'synthetic wal fixture')
            directory = hot.Directory(f.data)
            try:
                source = hot.Source(directory)
                self.assertEqual(source.walbytes(), 21)
                (f.data / 'home.db-wal').rename(f.data / 'moved-wal')
                (f.data / 'home.db-wal').write_bytes(b'new owned fixture')
                with self.assertRaisesRegex(hot.Rejected, 'file_identity_changed'):
                    source.walbytes()
            finally:
                directory.close()

    def test_fd_enumeration_uses_held_proc_directory_not_reopened_proc_path(self):
        with mock.patch.object(hot, 'PROC_FD', 123), \
                mock.patch.object(hot.os, 'listdir', return_value=['7']) as listed, \
                mock.patch.object(hot.os, 'fstat', return_value=types.SimpleNamespace(st_dev=44, st_ino=9)):
            self.assertEqual(hot.open_fds(), {7: (44, 9)})
        listed.assert_called_once_with(123)
        with mock.patch.object(hot, 'PROC_FD', None), self.assertRaisesRegex(hot.Rejected, 'proc_fd_not_anchored'):
            hot.open_fds()

    def test_child_root_uses_held_fd_not_public_path(self):
        directory = types.SimpleNamespace(fd=123, verify=mock.Mock())
        with mock.patch.object(hot.os, 'fstat', return_value=types.SimpleNamespace(st_mode=0o40700)), \
                mock.patch.object(hot.os, 'fchdir') as fchdir, \
                mock.patch.object(hot.os, 'chroot') as chroot, \
                mock.patch.object(hot.os, 'chdir') as chdir:
            hot.enter_root(directory)
        directory.verify.assert_called_once()
        fchdir.assert_called_once_with(123)
        chroot.assert_called_once_with('.')
        chdir.assert_called_once_with('/')

    def test_real_sqlite_api_pin_snapshot_and_full_check_on_owned_fixtures(self):
        # macOS adapter only: actual SQLite/FD proof, without claiming Linux chroot/proc.
        with self.fixtures() as f:
            cache = hot.Directory(f.cache)
            base = hot.Directory(f.base, cache)
            source_dir = hot.Directory(f.data)
            current_root = [None]
            original_open = hot.sqlite_open

            def enter(directory):
                directory.verify()
                current_root[0] = directory.path

            def connect(uri, fd):
                local = 'file:' + str(current_root[0]) + uri[len('file:'):]
                return original_open(local, fd)

            def fds():
                found = {}
                for name in os.listdir('/dev/fd'):
                    try:
                        fd = int(name)
                        found[fd] = hot.identity(os.fstat(fd))
                    except (OSError, ValueError):
                        pass
                return found

            writer = None
            try:
                with mock.patch.object(hot, 'enter_root', side_effect=enter), \
                        mock.patch.object(hot, 'sqlite_open', side_effect=connect), \
                        mock.patch.object(hot, 'open_fds', side_effect=fds):
                    self.assertTrue(hot.pin_selfcheck(base)['pin_selfcheck_ok'])
                    self.assertEqual(list(f.base.iterdir()), [])
                    writer = sqlite3.connect(f.data / 'home.db')
                    writer.execute('PRAGMA journal_mode=WAL')
                    writer.execute('UPDATE auth SET value=0')
                    writer.commit()
                    source = hot.Source(source_dir)
                    stage_fd = base.open_file('home-hot.db', create=True)
                    original_proof = hot.sqlite_pin_proof

                    def prove(source, before):
                        proof = original_proof(source, before)
                        writer.execute('UPDATE auth SET value=1')
                        writer.commit()
                        return proof
                    with mock.patch.object(hot, 'sqlite_pin_proof', side_effect=prove):
                        result = hot.snapshot(source, base, stage_fd)
                    self.assertTrue(result['source_pin']['readonly'])
                    self.assertIsNotNone(result['source_pin']['wal_dev_ino'])
                    self.assertEqual(hot.full_integrity(base, stage_fd), {'integrity': ['ok'], 'integrity_exit': 0})
                    checked = sqlite3.connect(':memory:')
                    try:
                        checked.deserialize(os.pread(stage_fd, 1024 * 1024, 0))
                        self.assertEqual(checked.execute('SELECT value FROM auth').fetchone(), (0,))
                        self.assertEqual(writer.execute('SELECT value FROM auth').fetchone(), (1,))
                    finally:
                        checked.close()
            finally:
                if writer is not None:
                    writer.close()
                source_dir.close()
                base.close()
                cache.close()

    @unittest.skipUnless(hot.sys.platform.startswith('linux') and os.geteuid() == 0,
                         'requires explicit tiny Linux/root run; never uses production paths')
    def test_tiny_linux_real_chroot_proc_pin_and_backup(self):
        with tempfile.TemporaryDirectory() as folder:
            parent = pathlib.Path(folder).resolve()
            parent.chmod(0o777)
            cache_path, source_parent = parent / 'cache', parent / 'source'
            cache_path.mkdir(mode=0o755)
            source_parent.mkdir(mode=0o700)
            data = source_parent / 'data'
            data.mkdir(mode=0o700)
            writer = sqlite3.connect(data / 'home.db')
            writer.execute('PRAGMA journal_mode=WAL')
            writer.execute('CREATE TABLE auth(value INTEGER)')
            writer.execute('INSERT INTO auth VALUES(7)')
            writer.commit()
            cache = hot.Directory(cache_path)
            base = hot.Directory(cache_path / 'private', cache)
            source_dir = hot.Directory(data)
            try:
                self.assertTrue(hot.run_worker(lambda: hot.pin_selfcheck(base), 10)['pin_selfcheck_ok'])
                source = hot.Source(source_dir)
                stage_fd = base.open_file('home-hot.db', create=True)
                result = {}
                hot.guarded_backup(source, base, stage_fd, result)
                self.assertEqual(result['copy_exit'], 0)
                self.assertTrue(result['source_pin']['same_direct_directory'])
                self.assertIsNotNone(result['source_pin']['wal_dev_ino'])
                self.assertEqual(hot.run_worker(lambda: hot.full_integrity(base, stage_fd), 10),
                                 {'integrity': ['ok'], 'integrity_exit': 0})
                self.assertTrue(os.fstat(stage_fd).st_size > 0)
            finally:
                source_dir.close()
                base.close()
                cache.close()
                writer.close()

    def test_parent_worker_ipc_is_bounded_and_failure_sanitized(self):
        self.assertEqual(hot.run_worker(lambda: {'synthetic': 1}, 5), {'synthetic': 1})

        def failure():
            raise OSError('synthetic private text must not escape')
        with self.assertRaisesRegex(hot.Rejected, '^OSError$'):
            hot.run_worker(failure, 5)

    def test_main_failure_is_nonzero_and_one_sanitized_json(self):
        with mock.patch.object(hot.sys, 'argv', ['ws0054-hot-backup.py']), \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(hot.main(), 1)
        self.assertEqual(len(output.getvalue().splitlines()), 1)
        self.assertFalse(json.loads(output.getvalue())['ok'])
        with mock.patch.object(hot.sys, 'platform', 'darwin'):
            self.assertEqual(hot.execute('ignored')['error'], 'linux_root_required')


if __name__ == '__main__':
    unittest.main()
