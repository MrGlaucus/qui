---
title: Script tasks
---

Script tasks run Python independently of the instance workflow engine. Open **Script tasks** from the navigation menu to create a script, then create one or more tasks from it. A script contains code and default JSON parameters. Each task has its own parameters, instance selection, schedule, permissions, and persistent state.

## Runtime and trust

The Docker image includes Python 3. For binary installations, install Python 3 and set `scriptPythonPath` in `config.toml` to the interpreter executable if `python3` is not on PATH. Restart QUI after changing it. You can point this setting at a virtual environment's interpreter when your scripts need additional libraries.

Only execute code you trust. Scripts run with the operating-system permissions of QUI. A separate process, SDK permissions, and virtual environments are **not a security sandbox**. Dry run blocks mutations made through the QUI SDK and uses temporary state, but cannot prevent Python code from writing files or making its own network requests. Do not paste unreviewed scripts into an internet-accessible installation.

The SDK does not expose client credentials or tracker passkeys. Instance scope and operation permissions apply to SDK calls. No instances are selected by default. Custom scripts may select all active instances or run without any instances. The built-in HDSKY task requires explicit instance selection.

## Tasks and scheduling

New tasks are disabled. Save a task and choose **Dry run** to inspect planned SDK operations. Enable it after checking the result. Disabled tasks can still be run manually.

- An interval of zero and no one-shot time means manual execution only.
- A positive interval schedules the next run after the previous run finishes.
- A one-shot time runs once; it is mutually exclusive with an interval.
- A task cannot overlap with itself. Separate tasks can run concurrently. Do not target the same torrents with conflicting tasks or workflows.
- `ctx.wake_at()` can bring the next interval run forward, for example to a torrent's age boundary. The scheduler checks once per second. Python startup, client requests, and load introduce delay; this is not a hard real-time scheduler.
- A timeout or cancellation stops the Python process. Completed external actions are not rolled back. Persistent state already saved remains available on the next run.

The history displays the latest 100 runs. Logs are limited to 256 KiB per run and retained for 30 days. Python stdout/stderr are captured. History is saved when the run finishes. State survives QUI restarts. Deleting a task also deletes its state and history; it does not undo speed limits or tags already applied.

## Python SDK

```python
from qui import ctx

for instance in ctx.instances():
    torrents = ctx.torrents(instance["id"])
    ctx.log(f"{instance['name']}: {len(torrents)} torrents")

count = ctx.state_get("run_count", 0)
ctx.state_set("run_count", count + 1)
```

`ctx.params` is the task's JSON object; `ctx.dry_run` identifies preview runs. Use `ctx.now()` for the current Unix timestamp. State is isolated per task and stores JSON values; use a key containing instance ID, torrent hash, and `added_on` for per-torrent state.

The in-app **Python API reference** is available from the Script tasks page and the script editor. It includes signatures, required permissions, return fields, and a runnable example.

- `ctx.instances()` → list[dict]. Selected active instances; enable All instances on the task to enumerate every active client.
- `ctx.instance(instance_id)` → dict. One selected instance. Metadata is captured at run start; no connection is required.
- `ctx.transfer(instance_id)` → dict. Fetch live client-wide transfer counters, speeds, limits and connection status. Counters follow qBittorrent semantics, not QUI daily history.
- `ctx.torrents(instance_id)` → list[dict]. Fetch a fresh torrent list from the client. Filter the returned list in Python.
- `ctx.torrent(instance_id, torrent_hash)` → dict. Fetch a fresh single-torrent snapshot. Missing torrents raise RuntimeError.
- `ctx.properties(instance_id, torrent_hash)` → dict. Fetch extended properties including average speeds, pieces and connections. Field names differ from torrent snapshots.
- `ctx.files(instance_id, torrent_hash)` → list[dict]. Get files through QUI's file cache. Not guaranteed to be a fresh snapshot.
- `ctx.trackers(instance_id, torrent_hash)` → list[str]. Get deduplicated tracker hostnames only; no announce URLs or passkeys.
- `ctx.categories(instance_id)` → dict[str, dict]. Categories keyed by name, each with name and savePath. May use QUI's cached metadata.
- `ctx.tags(instance_id)` → list[str]. List instance tag definitions; may use QUI's cached metadata.
- `ctx.upload_limit(instance_id, torrent_hash, bytes_per_second)` → None · limits. Set torrent upload limit. Nonnegative multiple of 1024 B/s; 0 removes the torrent limit.
- `ctx.download_limit(instance_id, torrent_hash, bytes_per_second)` → None · limits. Set torrent download limit using the same units as upload_limit.
- `ctx.add_tags(instance_id, torrent_hash, tags)` → None · tags. Add a list of strings, preserving existing tags.
- `ctx.remove_tags(instance_id, torrent_hash, tags)` → None · tags. Remove the specified tags from this torrent only. An empty list is a no-op.
- `ctx.set_tags(instance_id, torrent_hash, tags)` → None · tags. Replace all tags on this torrent; [] clears them.
- `ctx.create_tags(instance_id, tags)` → None · tags. Create instance-wide tag definitions.
- `ctx.delete_tags(instance_id, tags)` → None · tags. Delete tag definitions across the instance, affecting all torrents that use them.
- `ctx.set_category(instance_id, torrent_hash, category)` → None · categories. Assign an existing category; an empty string clears the torrent category. Automatic management may move files.
- `ctx.create_category(instance_id, category, save_path='')` → None · categories. Create an instance category. save_path is a path on the qBittorrent machine.
- `ctx.edit_category(instance_id, category, save_path='')` → None · categories. Change the existing category's save path, not its name. Automatic management may move files.
- `ctx.delete_categories(instance_id, categories)` → None · categories. Delete a nonempty list of category definitions from the instance; does not delete torrents or files.
- `ctx.pause(instance_id, torrent_hash) / resume(instance_id, torrent_hash)` → None · pause_resume. Pause or resume one torrent (mapped to stop/start on newer clients).
- `ctx.reannounce(instance_id, torrent_hash)` → None · reannounce. Request tracker reannounce. Completion means the request was accepted, not that the tracker credited traffic.
- `ctx.recheck(instance_id, torrent_hash)` → None · recheck. Request a torrent data recheck; it may interrupt transfer while checking.
- `ctx.delete(instance_id, torrent_hash, delete_files=False)` → None · delete. Delete through QUI with traffic compensation. True also deletes files. There is no interactive confirmation in scripts.
- `ctx.state_get(key, default=None) / state_set(key, value)` → JSON / None. Task-local persistent JSON state. Keys: 1–512 characters. None reads as the supplied default. Dry runs use an in-memory overlay.
- `ctx.wake_at(timestamp)` → None. Request an earlier next interval run using Unix seconds. Future timestamps only; earliest request wins. No exact-time guarantee; disabled/manual-only tasks are not enabled by this call.
- `ctx.notify(title, message)` → None · notify. Send through notification targets subscribed to automation actions. Dry run logs only.
- `ctx.now() / log(message)` → float / None. Current Unix seconds / append text to the run log. Logs are capped at 256 KiB per run.

### Returned fields

instance / instances: id (int), name (str), country_code (str), is_active (bool), has_local_filesystem_access (bool), daily_traffic_enabled (bool). Credentials and client URLs are not exposed.

torrent / torrents: hash, name, category, tags (comma-separated string), state (qBittorrent state string), save_path, content_path; added_on, completion_on, last_activity (Unix seconds); size (selected size), total_size (entire torrent), amount_left, uploaded, downloaded (bytes); upspeed, dlspeed, up_limit, dl_limit (B/s); progress (0–1), ratio (client upload/download ratio), seeding_time, time_active (seconds), num_seeds, num_leechs (connected peers); tracker_hosts (list[str]), trackers_complete (bool). If trackers_complete is false, use trackers(). Values such as -1 retain the client's unknown/unlimited sentinel semantics.

transfer: connection_status, dht_nodes, dl_info_data, up_info_data (bytes), dl_info_speed, up_info_speed, dl_rate_limit, up_rate_limit (B/s).

properties: addition_date, completion_date, creation_date, last_seen (Unix seconds); total_size, total_uploaded, total_downloaded, total_uploaded_session, total_downloaded_session, total_wasted, piece_size (bytes); up_speed, dl_speed, up_speed_avg, dl_speed_avg, up_limit, dl_limit (B/s); share_ratio, seeding_time, time_elapsed, eta, reannounce (seconds except share_ratio); pieces_have, pieces_num, nb_connections, nb_connections_limit, peers, peers_total, seeds, seeds_total; name, hash, infohash_v1, infohash_v2, save_path, download_path, comment, created_by, is_private. Some fields depend on client version.

files: index, name (torrent-relative path), size (bytes), progress (0–1), priority, availability, piece_range, is_seed (optional). categories: {name: {name, savePath}}. Empty collections may be None when supplied by an older client; iterate with “or []” / “or {}”.

SDK errors raise `RuntimeError`. Read operations are scoped to selected active instances. Categories and tag definitions apply to the whole instance; deleting them affects all torrents using them. Preview reads real data but does not simulate writes in later reads.

## HDSKY built-in rule

The built-in script is read-only; copy it to customize its code. Create a task, select the intended clients and choose **Target trackers** from the searchable multi-select. Options combine active torrent trackers from the selected clients with global persisted tracker history from the dashboard, using QUI's site names and merged domains. Workflow tracker selection also includes this history. History is limited to domains previously recorded by QUI. Selecting a merged site includes all its domains. Saved domains remain visible when no torrents currently use them. An empty selection is not allowed for HDSKY.

The JSON parameters may remain `{}`. Existing HDSKY tasks automatically move their legacy `parameters.tracker_domains` selection into the task's dedicated field on startup, preserving other parameters, schedule and baseline state. Existing partial domain selections are not expanded automatically.

Custom scripts receive the selection through `ctx.tracker_domains`. They must apply that filter themselves; SDK instance scope and permissions still apply, but the SDK does not enforce tracker filtering.

The script matches any of the torrent's tracker hostnames. It needs limits, tags, and reannounce permissions. Suggested interval: 10 seconds. Increase the timeout if a large library takes longer to inspect.

| Torrent age / condition | Upload limit | Rule tag |
| --- | --- | --- |
| Under 50 minutes, incomplete | 2 MiB/s | HDSKY·50分钟内下载中·2MiB/s |
| Under 50 minutes, complete | 200 KiB/s | HDSKY·50分钟内已完成·200KiB/s |
| From 50 minutes, awaiting baseline | 450 MiB/s | HDSKY·等待汇报基准·450MiB/s |
| Baseline saved, not ratio-limited | 450 MiB/s | HDSKY·基准后增量刷流·450MiB/s |
| Before 72 hours, baseline-relative ratio strictly above 3.1 | 200 KiB/s | HDSKY·增量超3.1倍限速·200KiB/s |
| No baseline, total ratio below 4 | 450 MiB/s | HDSKY·无基准总量刷流·450MiB/s |
| Before 72 hours, no baseline, total ratio at least 4 | 200 KiB/s | HDSKY·总量达4倍限速·200KiB/s |
| At least 72 hours | 450 MiB/s | HDSKY·已满72小时·450MiB/s |

Each successful run leaves exactly one current HDSKY stage tag, even when a transition does not change the speed limit. All six legacy rule tags and other known HDSKY stage tags are removed from the torrent; unrelated tags are preserved. The speed in the tag is the configured upload cap, not the measured speed. 450 MiB/s is still a cap, not unlimited upload.

At age 60 minutes and 1 second, a previously managed torrent receives a reannounce request. Immediately after qBittorrent accepts it, the script reads fresh uploaded bytes and saves the baseline. This does not confirm that the tracker received or credited that upload amount. The ratio is `(uploaded - baseline) / total_size`. Once triggered, the ratio limit stays active until 72 hours. Each transition removes only the other rule tags, preserving unrelated tags.

**Torrents at or after 62 minutes without a saved baseline are managed using total uploaded bytes / total torrent size.** Below 72 hours, a ratio of at least 4 triggers 200 KiB/s and the tag `HDSKY·总量达4倍限速·200KiB/s`; below 4 the limit is 450 MiB/s. Once triggered, the limit stays until 72 hours, when 450 MiB/s and the release tag apply. No baseline is invented and no late reannounce is sent for this historical mode. Existing skip-log metadata does not prevent these torrents from being managed after the update. A missing baseline may be captured from age 60 minutes and 1 second up to, but not including, 62 minutes, including newly discovered torrents in that window. At 62 minutes, any torrent still missing a baseline switches to total-counter mode, even if it was managed earlier or a previous reannounce succeeded. Slow client calls crossing the deadline do not create a late baseline. Existing baselines remain valid and continue using the delta rule. The delay is logged; the counter at a missed instant cannot be reconstructed.

If an upload counter drops below its saved baseline, the script marks that torrent's state invalid and skips further changes. A re-added torrent with a new `added_on` gets separate state. A client that re-adds the same hash with the same timestamp and counters cannot be distinguished from the previous torrent.

A crash between a successful client reannounce and saving its completion may repeat that request on recovery. Cross-system exactly-once delivery is not guaranteed. The script saves the baseline before subsequent limit/tag operations so those failures do not normally repeat the reannounce.


### HDSKY run logs

Each run ends with scanned, matching, managed (including historical), historical total-counter mode, invalid and failed counts, plus duration. Detailed entries identify the instance name, torrent name and hash. The script logs initial management, stage transitions, drift corrections, accepted reannounce requests, baseline counters and timing, and the first ratio-limit trigger with uploaded bytes, baseline, delta, torrent size and calculated ratio. A successful reannounce request does not confirm tracker accounting.

Unchanged torrents do not produce per-torrent entries. Skip reasons are recorded once per torrent identity and reason in separate log metadata; this never creates a management baseline for historical torrents. Errors are recorded on every failed attempt. Stage success is logged only after all required limit/tag calls succeed; partial actions are not rolled back. Preview messages are marked as dry run and do not claim real client changes.
