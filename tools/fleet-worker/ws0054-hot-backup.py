#!/usr/bin/env python3
"""Prospective Ark-only retained HOT snapshot; one fresh absolute cache BASE argument.

No deployment, restore, clones or pruning. Filesystem roots in the short-lived
SQLite child are anchored by directory descriptors, not proc-fd path spellings.
"""
import datetime
import hashlib
import json
import os
import pathlib
import re
import signal
import sqlite3
import stat
import subprocess
import sys
import time

LIVE = pathlib.Path('/mnt/cache/appdata/cpa-home/data/home.db')
LIVE_URI = 'file:/mnt/cache/appdata/cpa-home/data/home.db?mode=ro'
CACHE_ROOT = pathlib.Path('/mnt/cache/appdata/cpa-home-build')
ARRAY_ROOT = pathlib.Path('/mnt/disk18/cpa-home-disaster-backups')
COPY_BOUND = 600
WAL_LIMIT = 1024 ** 3
SAMPLE_INTERVAL = 0.1
PROC_FD = None


class Rejected(Exception):
    pass


def require(condition, code):
    if not condition:
        raise Rejected(code)


def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def identity(s):
    return s.st_dev, s.st_ino


def fresh_base(raw):
    require(bool(re.fullmatch(r'/mnt/cache/appdata/cpa-home-build/ws0054-hot-[A-Za-z0-9][A-Za-z0-9_-]{0,95}', raw)), 'unsafe_base')
    base = pathlib.Path(raw)
    require(not os.path.lexists(base), 'base_exists')
    require(not os.path.lexists(ARRAY_ROOT / base.name), 'destination_exists')
    return base


class Directory:
    """Hold every no-follow component; writable mount ancestors are not rejected."""
    def __init__(self, path, parent=None):
        self.path, self.files, self.owned, self.chain = path, {}, [], []
        self.private = parent is not None
        try:
            if parent is None:
                fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
                self.owned.append(fd)
                self.chain.append((None, fd, os.fstat(fd)))
                for name in path.parts[1:]:
                    fd = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                    self.owned.append(fd)
                    self.chain.append((name, fd, os.fstat(fd)))
            else:
                parent.verify()
                os.mkdir(path.name, mode=0o700, dir_fd=parent.fd)
                fd = os.open(path.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent.fd)
                self.owned.append(fd)
                self.chain = parent.chain + [(path.name, fd, os.fstat(fd))]
                os.fsync(parent.fd)
            self.fd = fd
            s = os.fstat(fd)
            require(s.st_uid == 0 and not s.st_mode & 0o022, 'untrusted_directory')
            if parent is not None:
                require(stat.S_IMODE(s.st_mode) == 0o700, 'directory_not_private')
            self.verify()
        except Exception:
            self.close()
            raise

    def verify(self):
        previous = None
        for name, fd, original in self.chain:
            require(identity(os.fstat(fd)) == identity(original), 'directory_identity_changed')
            if previous is not None:
                s = os.stat(name, dir_fd=previous, follow_symlinks=False)
                require(stat.S_ISDIR(s.st_mode) and identity(s) == identity(original), 'directory_identity_changed')
            previous = fd
        s = os.fstat(self.fd)
        require(s.st_uid == 0 and not s.st_mode & 0o022, 'untrusted_directory')
        if self.private:
            require(stat.S_IMODE(s.st_mode) == 0o700, 'directory_not_private')

    def check_file(self, name):
        fd = self.files[name]
        opened = os.fstat(fd)
        named = os.stat(name, dir_fd=self.fd, follow_symlinks=False)
        require(stat.S_ISREG(named.st_mode) and identity(named) == identity(opened), 'file_identity_changed')
        require(opened.st_uid == 0 and opened.st_nlink == 1 and not opened.st_mode & 0o022, 'untrusted_file')
        return opened

    def open_file(self, name, create=False):
        require(name not in self.files, 'file_already_open')
        flags = os.O_RDWR | os.O_CREAT | os.O_EXCL if create else os.O_RDONLY
        fd = os.open(name, flags | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600, dir_fd=self.fd)
        self.files[name] = fd
        try:
            self.check_file(name)
            return fd
        except Exception:
            del self.files[name]
            os.close(fd)
            raise

    def remove(self, name):
        self.verify()
        self.check_file(name)
        os.unlink(name, dir_fd=self.fd)
        os.close(self.files.pop(name))
        os.fsync(self.fd)
        self.verify()

    def close(self):
        for fd in [*self.files.values(), *reversed(self.owned)]:
            os.close(fd)
        self.files.clear()
        self.owned.clear()


def trusted_tool(name):
    for folder in ('/usr/bin', '/bin'):
        path = pathlib.Path(folder, name)
        if not path.exists():
            continue
        path = path.resolve(strict=True)
        for part in [*reversed(path.parents), path]:
            s = part.stat()
            require(s.st_uid == 0 and not s.st_mode & 0o022, 'untrusted_tool')
        require(path.is_file() and os.access(path, os.X_OK), 'unusable_tool')
        return str(path)
    raise Rejected('missing_' + name)


def mount_for(path, text):
    matches = []
    for line in text.splitlines():
        left, right = line.split(' - ', 1)
        fields, fs = left.split(), right.split()
        mount = pathlib.Path(re.sub(r'\\([0-7]{3})', lambda m: chr(int(m[1], 8)), fields[4]))
        if path == mount or mount in path.parents:
            matches.append((len(mount.parts), {'mount': str(mount), 'filesystem': fs[0],
                                              'device': fs[1], 'major_minor': fields[2]}))
    require(bool(matches), 'mount_proof_missing')
    return max(matches, key=lambda item: item[0])[1]


def check_devices(source_dev, stage_dev, target_dev, cache, array):
    require(source_dev == stage_dev == 44 and target_dev == 2322 and source_dev != target_dev, 'wrong_device')
    require(cache == {'mount': '/mnt/cache', 'filesystem': 'btrfs', 'device': '/dev/nvme1n1p1',
                      'major_minor': f'{os.major(44)}:{os.minor(44)}'}, 'wrong_cache_mount')
    require(array == {'mount': '/mnt/disk18', 'filesystem': 'xfs', 'device': '/dev/md18p1',
                      'major_minor': f'{os.major(2322)}:{os.minor(2322)}'}, 'wrong_array_mount')
    return {'source_dev': source_dev, 'stage_dev': stage_dev, 'destination_dev': target_dev,
            'different_devices': True, 'cache_mount': cache, 'destination_mount': array}


class Source:
    def __init__(self, directory):
        self.directory = directory
        require(stat.S_IMODE(os.fstat(directory.fd).st_mode) == 0o700, 'source_directory_not_private')
        parent = directory.chain[-2][2]
        require(parent.st_uid == 0 and stat.S_IMODE(parent.st_mode) == 0o700, 'source_parent_not_private')
        self.db = directory.open_file('home.db')
        self.wal = self.shm = None
        for name, attribute in (('home.db-wal', 'wal'), ('home.db-shm', 'shm')):
            try:
                setattr(self, attribute, directory.open_file(name))
            except FileNotFoundError:
                pass
        self.verify()

    def verify(self):
        self.directory.verify()
        require(stat.S_IMODE(os.fstat(self.directory.fd).st_mode) == 0o700, 'source_directory_not_private')
        parent = os.fstat(self.directory.chain[-2][1])
        require(parent.st_uid == 0 and stat.S_IMODE(parent.st_mode) == 0o700, 'source_parent_not_private')
        self.directory.check_file('home.db')
        if self.shm is not None:
            self.directory.check_file('home.db-shm')
            require(os.fstat(self.shm).st_dev == os.fstat(self.db).st_dev, 'source_shm_device')
        if self.wal is not None:
            self.directory.check_file('home.db-wal')
            require(os.fstat(self.wal).st_dev == os.fstat(self.db).st_dev, 'source_wal_device')
        else:
            try:
                os.stat('home.db-wal', dir_fd=self.directory.fd, follow_symlinks=False)
            except FileNotFoundError:
                return
            raise Rejected('source_wal_changed')

    def walbytes(self):
        self.verify()
        return os.fstat(self.wal).st_size if self.wal is not None else 0


def device_proof(source, cache, array):
    require(str(LIVE) == '/mnt/cache/appdata/cpa-home/data/home.db' and
            LIVE_URI == 'file:/mnt/cache/appdata/cpa-home/data/home.db?mode=ro', 'wrong_source')
    source.verify()
    cache.verify()
    array.verify()
    mounts = pathlib.Path('/proc/self/mountinfo').read_text()
    cache_mount = mount_for(LIVE, mounts)
    require(mount_for(CACHE_ROOT, mounts) == cache_mount, 'nested_cache_mount')
    array_mount = mount_for(ARRAY_ROOT, mounts)
    proof = check_devices(os.fstat(source.db).st_dev, os.fstat(cache.fd).st_dev,
                          os.fstat(array.fd).st_dev, cache_mount, array_mount)
    for device in (cache_mount['device'], array_mount['device']):
        require(stat.S_ISBLK(pathlib.Path(device).stat().st_mode), 'mount_device_not_block')
    return proof


def headroom(db_bytes, wal_bytes, total, free):
    reserve = 2 * max(45 * 1024 ** 3, db_bytes + wal_bytes)
    require(db_bytes > 0 and total > 0 and (free - reserve) * 5 >= total, 'cache_headroom')
    return {'cache_total_bytes': total, 'cache_free_bytes_before': free, 'reserve_bytes': reserve,
            'cache_free_bytes_after_reserve': free - reserve,
            'minimum_free_fraction': 0.2, 'pass': True}


def cache_headroom(source, cache):
    v = os.fstatvfs(cache.fd)
    return headroom(os.fstat(source.db).st_size, source.walbytes(), v.f_blocks * v.f_frsize, v.f_bavail * v.f_frsize)


def enter_root(directory):
    directory.verify()
    require(stat.S_IMODE(os.fstat(directory.fd).st_mode) == 0o700, 'worker_root_not_private')
    os.fchdir(directory.fd)
    os.chroot('.')
    os.chdir('/')


def open_fds():
    require(PROC_FD is not None, 'proc_fd_not_anchored')
    found = {}
    for name in os.listdir(PROC_FD):
        try:
            fd = int(name)
            found[fd] = identity(os.fstat(fd))
        except (ValueError, OSError):
            pass
    return found


def sqlite_open(uri, owned_fd):
    before = open_fds()
    db = sqlite3.connect(uri, uri=True, timeout=1.5)
    try:
        require(any(fd not in before and item == identity(os.fstat(owned_fd))
                    for fd, item in open_fds().items()), 'sqlite_db_identity_mismatch')
        return db, before
    except Exception:
        db.close()
        raise


def sqlite_pin_proof(source, before):
    source.verify()
    opened = {item for fd, item in open_fds().items() if fd not in before}
    require(identity(os.fstat(source.db)) in opened, 'sqlite_source_db_mismatch')
    require(source.wal is None or identity(os.fstat(source.wal)) in opened, 'sqlite_source_wal_mismatch')
    return {'db_dev_ino': list(identity(os.fstat(source.db))),
            'wal_dev_ino': list(identity(os.fstat(source.wal))) if source.wal is not None else None,
            'directory_dev_ino': list(identity(os.fstat(source.directory.fd))),
            'sqlite_open_uri': 'file:/home.db?mode=ro',
            'same_direct_directory': True, 'readonly': True, 'begin_and_select': True}


def pin_selfcheck(base):
    enter_root(base)
    source_fd = base.open_file('pin-source.db', create=True)
    copy_fd = base.open_file('pin-copy.db', create=True)
    writer, _ = sqlite_open('file:/pin-source.db?mode=rw', source_fd)
    reader = copy = None
    try:
        writer.execute('PRAGMA journal_mode=WAL')
        writer.execute('CREATE TABLE synthetic(value INTEGER)')
        writer.execute('INSERT INTO synthetic VALUES(0)')
        writer.commit()
        reader, _ = sqlite_open('file:/pin-source.db?mode=ro', source_fd)
        reader.execute('BEGIN')
        require(reader.execute('SELECT value FROM synthetic').fetchone() == (0,), 'pin_selfcheck_select')
        writer.execute('UPDATE synthetic SET value=1')
        writer.commit()
        copy, _ = sqlite_open('file:/pin-copy.db?mode=rw', copy_fd)
        copy.execute('PRAGMA journal_mode=OFF')
        reader.backup(copy)
        require(copy.execute('SELECT value FROM synthetic').fetchone() == (0,), 'pin_selfcheck_snapshot')
        require(writer.execute('SELECT value FROM synthetic').fetchone() == (1,), 'pin_selfcheck_writer')
    finally:
        for db in (copy, reader, writer):
            if db is not None:
                db.close()
    for name in ('pin-source.db', 'pin-copy.db'):
        base.remove(name)
    # SQLite removes its own WAL/SHM after the final synthetic connection closes.
    for name in ('pin-source.db-wal', 'pin-source.db-shm', 'pin-copy.db-wal', 'pin-copy.db-shm'):
        try:
            os.stat(name, dir_fd=base.fd, follow_symlinks=False)
        except FileNotFoundError:
            continue
        raise Rejected('pin_selfcheck_sidecar_remains')
    return {'pin_selfcheck_ok': True}


def snapshot(source, base, stage_fd):
    enter_root(source.directory)
    reader, before = sqlite_open('file:/home.db?mode=ro', source.db)
    copy = None
    try:
        reader.execute('PRAGMA query_only=ON')
        reader.execute('BEGIN')
        reader.execute('SELECT count(*) FROM auth').fetchone()
        proof = sqlite_pin_proof(source, before)
        # The pinned DB/WAL stay open while only the child changes its root.
        enter_root(base)
        copy, _ = sqlite_open('file:/home-hot.db?mode=rw', stage_fd)
        copy.execute('PRAGMA journal_mode=OFF')
        copy.execute('PRAGMA temp_store=MEMORY')
        reader.backup(copy, pages=256, sleep=SAMPLE_INTERVAL)
        require(copy.execute('PRAGMA journal_mode=DELETE').fetchone() == ('delete',), 'stage_journal_mode')
        return {'source_pin': proof}
    finally:
        try:
            enter_root(source.directory)
            reader.rollback()
            reader.close()
        finally:
            if copy is not None:
                enter_root(base)
                copy.close()


class Child:
    def __init__(self, pid):
        self.pid, self.returncode = pid, None

    def poll(self):
        if self.returncode is None:
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.returncode = os.waitstatus_to_exitcode(status)
        return self.returncode

    def wait(self, timeout):
        until = time.monotonic() + timeout
        while self.poll() is None:
            if time.monotonic() >= until:
                raise subprocess.TimeoutExpired('sqlite_worker', timeout)
            time.sleep(0.01)
        return self.returncode


def stop_own(p):
    if p.poll() is not None:
        return
    # The child may not have completed setsid when a preflight check fails.
    try:
        os.killpg(p.pid, signal.SIGTERM)
    except ProcessLookupError:
        try:
            os.kill(p.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    try:
        p.wait(timeout=10)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(p.pid, signal.SIGKILL)
        except ProcessLookupError:
            try:
                os.kill(p.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        p.wait(timeout=10)


def run_worker(callback, bound, observe=None):
    global PROC_FD
    read_fd, write_fd = os.pipe()
    try:
        pid = os.fork()
    except Exception:
        os.close(read_fd)
        os.close(write_fd)
        raise
    if pid == 0:
        os.close(read_fd)
        try:
            os.setsid()
            os.environ.clear()
            if sys.platform.startswith('linux'):
                # Open in this child before chroot; no /proc mount is needed inside it.
                PROC_FD = os.open('/proc/self/fd', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            value = callback()
            packet = {'ok': True, 'value': value}
        except BaseException as error:
            packet = {'ok': False, 'error': str(error) if isinstance(error, Rejected) else type(error).__name__}
        try:
            payload = json.dumps(packet, separators=(',', ':')).encode()
            require(len(payload) < 4096, 'worker_packet_too_large')
            os.write(write_fd, payload)
        finally:
            os._exit(0 if packet['ok'] else 1)
    os.close(write_fd)
    p, started = Child(pid), time.monotonic()
    try:
        while True:
            elapsed = time.monotonic() - started
            if observe is not None:
                observe(elapsed)
            require(elapsed < bound, 'worker_timeout')
            if p.poll() is not None:
                break
            time.sleep(SAMPLE_INTERVAL)
        payload = os.read(read_fd, 4096)
        packet = json.loads(payload)
        require(p.returncode == 0 and packet.get('ok') is True,
                packet.get('error', 'worker_failed'))
        return packet['value']
    finally:
        stop_own(p)
        os.close(read_fd)


def guarded_backup(source, base, stage_fd, result):
    start_wal = source.walbytes()
    started = time.monotonic()
    result.update(wal_guard_active_before_begin=True, live_wal_start_bytes=start_wal,
                  live_wal_peak_observed_bytes=start_wal, live_wal_growth_bytes=0,
                  wal_sample_interval_seconds=SAMPLE_INTERVAL, copy_bound_seconds=COPY_BOUND,
                  wal_growth_limit_bytes=WAL_LIMIT, abort_reason=None)

    def observe(_elapsed):
        peak = max(result['live_wal_peak_observed_bytes'], source.walbytes())
        elapsed = time.monotonic() - started
        result.update(live_wal_peak_observed_bytes=peak, live_wal_growth_bytes=peak - start_wal,
                      copy_seconds=round(elapsed, 3))
        if peak - start_wal > WAL_LIMIT or elapsed >= COPY_BOUND:
            result['abort_reason'] = 'live_wal_growth_over_1GiB' if peak - start_wal > WAL_LIMIT else 'copy_duration_over_600s'
            raise Rejected('guarded_backup_failed')
    try:
        result.update(run_worker(lambda: snapshot(source, base, stage_fd), COPY_BOUND, observe))
        result['copy_exit'] = 0
    except Exception:
        result['copy_exit'] = 1
        raise


def full_integrity(base, stage_fd):
    enter_root(base)
    db, _ = sqlite_open('file:/home-hot.db?mode=ro', stage_fd)
    try:
        rows = db.execute('PRAGMA integrity_check').fetchall()
        require(rows == [('ok',)], 'full_integrity_failed')
        return {'integrity': ['ok'], 'integrity_exit': 0}
    finally:
        db.close()


def sha256(fd):
    digest = hashlib.sha256()
    offset = 0
    if hasattr(os, 'posix_fadvise'):
        os.posix_fadvise(fd, 0, 0, os.POSIX_FADV_DONTNEED)
    while True:
        block = os.pread(fd, 16 * 1024 * 1024, offset)
        if not block:
            return digest.hexdigest()
        digest.update(block)
        offset += len(block)


def write_all(fd, data, offset):
    while data:
        written = os.pwrite(fd, data, offset)
        require(written > 0, 'short_write')
        offset += written
        data = data[written:]


def write_result(directory, result):
    directory.verify()
    name, temp = 'hot-backup.json', 'hot-backup.json.tmp'
    fd = directory.open_file(temp, create=True)
    data = (json.dumps(result, indent=2) + '\n').encode()
    write_all(fd, data, 0)
    os.fsync(fd)
    directory.check_file(temp)
    if name in directory.files:
        directory.check_file(name)
    else:
        try:
            os.stat(name, dir_fd=directory.fd, follow_symlinks=False)
        except FileNotFoundError:
            pass
        else:
            raise Rejected('result_exists')
    os.replace(temp, name, src_dir_fd=directory.fd, dst_dir_fd=directory.fd)
    old = directory.files.pop(name, None)
    directory.files[name] = directory.files.pop(temp)
    if old is not None:
        os.close(old)
    os.fsync(directory.fd)
    directory.verify()
    directory.check_file(name)


def plain_copy(stage_fd, target_fd, directory_fd):
    offset = 0
    while True:
        block = os.pread(stage_fd, 16 * 1024 * 1024, offset)
        if not block:
            break
        write_all(target_fd, block, offset)
        offset += len(block)
    os.fsync(target_fd)
    os.fsync(directory_fd)
    return {}


def transfer(base, root, result):
    require(result['stage_verified'] and result['integrity'] == ['ok'] and result['integrity_exit'] == 0,
            'stage_not_full_verified')
    base.verify()
    root.verify()
    s = base.check_file('home-hot.db')
    require(s.st_size == result['bytes'] and s.st_dev == result['devices']['stage_dev'], 'stage_changed')
    require(os.fstat(root.fd).st_dev == result['devices']['destination_dev'] != s.st_dev, 'wrong_transfer_device')
    v = os.fstatvfs(root.fd)
    require(v.f_bavail * v.f_frsize >= s.st_size, 'array_headroom')
    target = root.open_file('home-hot.db', create=True)
    result['destination_state'] = 'unverified_partial_retained'
    started = time.monotonic()
    try:
        run_worker(lambda: plain_copy(base.files['home-hot.db'], target, root.fd), 1800)
        result['transfer_exit'] = 0
    except Exception:
        result['transfer_exit'] = 1
        raise
    finally:
        result['transfer_seconds'] = round(time.monotonic() - started, 3)
    base.verify()
    root.verify()
    result['destination_bytes'] = root.check_file('home-hot.db').st_size
    result['destination_sha256'] = sha256(target)
    result['destination_match'] = (result['destination_bytes'] == result['bytes'] and
                                   result['destination_sha256'] == result['sha256'])
    require(result['destination_match'], 'destination_mismatch')
    result.update(destination_state='verified_retained', retained_copy=True)
    checksum = root.open_file('home-hot.db.sha256', create=True)
    write_all(checksum, (result['sha256'] + '  home-hot.db\n').encode(), 0)
    os.fsync(checksum)
    os.fsync(root.fd)
    result['stage_cleanup_state'] = 'verified_retained_cleanup_pending'
    write_result(root, result)
    base.verify()
    root.verify()
    root.check_file('home-hot.db')
    root.check_file('home-hot.db.sha256')
    base.remove('home-hot.db')
    result['stage_cleanup_state'] = 'deleted_after_match'


def execute(raw):
    result = {'schema_version': 1, 'ok': False, 'error': None, 'started_utc': utc(),
              'completed_utc': None, 'source_direct_path': str(LIVE), 'source_uri': LIVE_URI,
              'method': 'readonly sqlite3_backup API BEGIN+count(auth)+backup+ROLLBACK; descriptor-rooted child',
              'base': None, 'stage_path': None, 'destination_path': None, 'result_path': None,
              'retained_result_path': None, 'checksum_path': None, 'bytes': 0,
              'integrity_check': 'FULL', 'integrity': [], 'integrity_exit': None, 'sha256': None,
              'destination_bytes': None, 'destination_sha256': None, 'destination_match': False,
              'stage_verified': False, 'retained_copy': False, 'stage_cleanup_state': 'not_created',
              'destination_state': 'not_created', 'devices': None, 'headroom': None,
              'wal_guard_active_before_begin': False, 'abort_reason': None, 'pin_selfcheck_ok': False}
    directories, base, root = [], None, None
    try:
        require(sys.platform.startswith('linux') and os.geteuid() == 0, 'linux_root_required')
        require(sys.flags.isolated, 'isolated_python_required')
        candidate = fresh_base(raw)
        for path in (LIVE.parent, CACHE_ROOT, ARRAY_ROOT):
            directories.append(Directory(path))
        source_dir, cache, array = directories
        source = Source(source_dir)
        result['devices'] = device_proof(source, cache, array)
        result['headroom'] = cache_headroom(source, cache)
        tools = {name: trusted_tool(name) for name in ('python3',)}
        require(pathlib.Path(sys.executable).resolve() == pathlib.Path(tools['python3']), 'wrong_python_runtime')
        base = Directory(candidate, cache)
        directories.append(base)
        stage = base.path / 'home-hot.db'
        target = ARRAY_ROOT / base.path.name
        result.update(base=str(base.path), stage_path=str(stage), destination_path=str(target / 'home-hot.db'),
                      result_path=str(base.path / 'hot-backup.json'), retained_result_path=str(target / 'hot-backup.json'),
                      checksum_path=str(target / 'home-hot.db.sha256'), tools=tools)
        result.update(run_worker(lambda: pin_selfcheck(base), 30))
        source.verify()
        result['headroom'] = cache_headroom(source, cache)
        stage_fd = base.open_file('home-hot.db', create=True)
        result['stage_cleanup_state'] = 'unverified_retained'
        guarded_backup(source, base, stage_fd, result)
        source.verify()
        base.verify()
        result['bytes'] = base.check_file('home-hot.db').st_size
        require(result['bytes'] > 0, 'empty_stage')
        started = time.monotonic()
        result.update(run_worker(lambda: full_integrity(base, stage_fd), 900))
        result['integrity_seconds'] = round(time.monotonic() - started, 3)
        result['sha256'] = sha256(stage_fd)
        os.fsync(stage_fd)
        result.update(stage_verified=True, stage_cleanup_state='verified_retained')
        write_result(base, result)
        require(device_proof(source, cache, array) == result['devices'], 'devices_changed')
        root = Directory(target, array)
        directories.append(root)
        transfer(base, root, result)
        result['ok'] = True
    except Exception as error:
        result['ok'] = False
        result['error'] = str(error) if isinstance(error, Rejected) else type(error).__name__
    result['completed_utc'] = utc()
    if base is not None:
        try:
            # Every successful receipt is checked on both sides of its final commit.
            for directory in directories:
                directory.verify()
            write_result(base, result)
            if result['retained_copy']:
                write_result(root, result)
            for directory in directories:
                directory.verify()
            for directory in (base, root):
                if directory is not None:
                    for name in directory.files:
                        directory.check_file(name)
        except Exception:
            result.update(ok=False, error='result_persist_failed')
            # Use held directories only; never follow a replacement path to repair a receipt.
            for directory in (base, root):
                if directory is not None:
                    try:
                        write_result(directory, result)
                    except Exception:
                        pass
    for directory in reversed(directories):
        try:
            directory.close()
        except Exception:
            result.update(ok=False, error='handle_close_failed')
    return result


def main():
    os.umask(0o077)
    if len(sys.argv) != 2:
        result = {'ok': False, 'error': 'expected_one_fresh_absolute_base', 'result_path': None}
    else:
        result = execute(sys.argv[1])
    print(json.dumps({key: result.get(key) for key in
                      ('ok', 'error', 'result_path', 'destination_path', 'bytes', 'retained_copy', 'stage_cleanup_state')},
                     separators=(',', ':')), flush=True)
    return 0 if result['ok'] else 1


if __name__ == '__main__':
    sys.exit(main())
