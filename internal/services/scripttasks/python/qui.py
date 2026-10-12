"""QUI's trusted-script SDK. All byte counters and limits use bytes."""
import json
import sys
import time

_wire = sys.stdout
sys.stdout = sys.stderr


class Context:
    def __init__(self):
        config = json.loads(sys.stdin.readline())
        self.params = config["parameters"]
        self.dry_run = config["dry_run"]
        self.task_id = config["task_id"]
        self.tracker_domains = config["tracker_domains"] or []

    def _call(self, method, **args):
        _wire.write(json.dumps({"method": method, **args}) + "\n")
        _wire.flush()
        line = sys.stdin.readline()
        if not line:
            raise RuntimeError("QUI host disconnected")
        reply = json.loads(line)
        if reply.get("error"):
            raise RuntimeError(reply["error"])
        return reply.get("result")

    def now(self):
        return time.time()

    def log(self, message):
        print(str(message), flush=True)

    def instances(self):
        return self._call("instances")

    def torrents(self, instance_id):
        return self._call("torrents", instance_id=instance_id)

    def torrent(self, instance_id, torrent_hash):
        return self._call("torrent", instance_id=instance_id, hash=torrent_hash)

    def trackers(self, instance_id, torrent_hash):
        """Return tracker hostnames, never passkeys or announce URLs."""
        return self._call("trackers", instance_id=instance_id, hash=torrent_hash)

    def instance(self, instance_id):
        return self._call("instance", instance_id=instance_id)

    def transfer(self, instance_id):
        return self._call("transfer", instance_id=instance_id)

    def categories(self, instance_id):
        return self._call("categories", instance_id=instance_id)

    def tags(self, instance_id):
        return self._call("tags", instance_id=instance_id)

    def properties(self, instance_id, torrent_hash):
        return self._call("properties", instance_id=instance_id, hash=torrent_hash)

    def files(self, instance_id, torrent_hash):
        return self._call("files", instance_id=instance_id, hash=torrent_hash)

    def set_tags(self, instance_id, torrent_hash, tags):
        return self._call("set_tags", instance_id=instance_id, hash=torrent_hash, tags=tags)

    def create_tags(self, instance_id, tags):
        return self._call("create_tags", instance_id=instance_id, tags=tags)

    def delete_tags(self, instance_id, tags):
        return self._call("delete_tags", instance_id=instance_id, tags=tags)

    def set_category(self, instance_id, torrent_hash, category):
        return self._call("set_category", instance_id=instance_id, hash=torrent_hash, category=category)

    def create_category(self, instance_id, category, save_path=""):
        return self._call("create_category", instance_id=instance_id, category=category, path=save_path)

    def edit_category(self, instance_id, category, save_path=""):
        return self._call("edit_category", instance_id=instance_id, category=category, path=save_path)

    def delete_categories(self, instance_id, categories):
        return self._call("delete_categories", instance_id=instance_id, categories=categories)

    def recheck(self, instance_id, torrent_hash):
        return self._call("recheck", instance_id=instance_id, hash=torrent_hash)

    def state_get(self, key, default=None):
        result = self._call("state_get", key=key)
        return default if result is None else result

    def state_set(self, key, value):
        return self._call("state_set", key=key, value=value)

    def wake_at(self, timestamp):
        return self._call("wake_at", timestamp=int(timestamp))

    def upload_limit(self, instance_id, torrent_hash, bytes_per_second):
        return self._call("upload_limit", instance_id=instance_id,
                          hash=torrent_hash, limit=bytes_per_second)

    def download_limit(self, instance_id, torrent_hash, bytes_per_second):
        return self._call("download_limit", instance_id=instance_id,
                          hash=torrent_hash, limit=bytes_per_second)

    def add_tags(self, instance_id, torrent_hash, tags):
        return self._call("add_tags", instance_id=instance_id, hash=torrent_hash, tags=tags)

    def remove_tags(self, instance_id, torrent_hash, tags):
        return self._call("remove_tags", instance_id=instance_id, hash=torrent_hash, tags=tags)

    def reannounce(self, instance_id, torrent_hash):
        return self._call("reannounce", instance_id=instance_id, hash=torrent_hash)

    def pause(self, instance_id, torrent_hash):
        return self._call("pause", instance_id=instance_id, hash=torrent_hash)

    def resume(self, instance_id, torrent_hash):
        return self._call("resume", instance_id=instance_id, hash=torrent_hash)

    def delete(self, instance_id, torrent_hash, delete_files=False):
        return self._call("delete_files" if delete_files else "delete",
                          instance_id=instance_id, hash=torrent_hash)

    def notify(self, title, message):
        return self._call("notify", title=title, message=message)


ctx = Context()
