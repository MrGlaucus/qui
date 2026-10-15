import copy
import importlib.util
from pathlib import Path
import sys
import types
import unittest


class FakeContext:
    def __init__(self):
        self.time = 1000000
        self.dry_run = False
        self.logs = []
        self.state = {}
        self.actions = []
        self.current = None
        self.params = {}
        self.tracker_domains = ["tracker.example.test"]

    def log(self, message):
        self.logs.append(message)

    def now(self):
        return self.time

    def state_get(self, key):
        return copy.deepcopy(self.state.get(key))

    def state_set(self, key, value):
        self.state[key] = copy.deepcopy(value)

    def torrent(self, instance_id, torrent_hash):
        return copy.deepcopy(self.current)

    def __getattr__(self, name):
        return lambda *args: self.actions.append((name, *args))


class HDSKYTests(unittest.TestCase):
    def setUp(self):
        self.ctx = FakeContext()
        sys.modules["qui"] = types.SimpleNamespace(ctx=self.ctx)
        spec = importlib.util.spec_from_file_location("hdsky_test", Path(__file__).with_name("hdsky.py"))
        self.script = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.script)

    def torrent(self, age, **values):
        result = dict(name="Synthetic torrent", hash="a" * 40, added_on=self.ctx.time - age, total_size=1000,
                      uploaded=100, progress=0.5, up_limit=0, tags="unrelated")
        result.update(values)
        self.ctx.current = result
        return result

    def run_torrent(self, torrent):
        self.ctx.current = torrent
        self.script.process(1, torrent)

    def limits(self):
        return [action[-1] for action in self.ctx.actions if action[0] == "upload_limit"]

    def test_first_fifty_minutes(self):
        for complete, expected in [(False, 2 * 1024**2), (True, 200 * 1024)]:
            with self.subTest(complete=complete):
                self.ctx.actions.clear()
                self.run_torrent(self.torrent(2999, progress=1 if complete else 0.5))
                self.assertEqual(self.limits(), [expected])

    def test_fifty_minute_boundary(self):
        self.run_torrent(self.torrent(3000))
        self.assertEqual(self.limits(), [450 * 1024**2])

    def test_historical_threshold_and_release(self):
        for age, uploaded, expected in [(3720, 3999, 450 * 1024**2),
                                         (4000, 4000, 200 * 1024),
                                         (4000, 4001, 200 * 1024),
                                         (72 * 3600, 5000, 450 * 1024**2),
                                         (90 * 3600, 5000, 450 * 1024**2)]:
            with self.subTest(age=age, uploaded=uploaded):
                self.ctx.state.clear()
                self.ctx.actions.clear()
                self.run_torrent(self.torrent(age, uploaded=uploaded))
                self.assertEqual(self.limits(), [expected])
                self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))
                self.assertNotIn("baseline", next(iter(self.ctx.state.values())))

    def test_baseline_once_and_strict_ratio_then_release(self):
        torrent = self.torrent(3600)
        self.run_torrent(torrent)
        self.ctx.time += 1
        self.run_torrent(torrent)
        self.assertEqual(sum(a[0] == "reannounce" for a in self.ctx.actions), 1)
        state = next(iter(self.ctx.state.values()))
        self.assertEqual(state["baseline"], 100)
        self.ctx.actions.clear()
        torrent["uploaded"] = 3200  # Exactly 3.1 must not trigger.
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [450 * 1024**2])
        self.ctx.actions.clear()
        torrent["uploaded"] = 3201
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [200 * 1024])
        self.ctx.actions.clear()
        torrent["total_size"] = 10000  # Sticky until 72h.
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [200 * 1024])
        self.ctx.actions.clear()
        self.ctx.time = torrent["added_on"] + 72 * 3600
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [450 * 1024**2])
        self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))

    def test_preserves_other_tags_and_avoids_redundant_writes(self):
        torrent = self.torrent(100, up_limit=2 * 1024**2,
                               tags="unrelated," + self.script.TAGS["downloading"] + ",50分钟内下载完成限速")
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [])
        self.assertIn(("remove_tags", 1, torrent["hash"], ["50分钟内下载完成限速"]), self.ctx.actions)
        self.assertFalse(any(a[0] == "add_tags" for a in self.ctx.actions))

    def test_delta_completion_marker_is_sticky_and_backfilled(self):
        torrent = self.torrent(4000, uploaded=3200)
        key = f"torrent:1:{torrent['hash']}:{torrent['added_on']}"
        self.ctx.state[key] = {"baseline": 100}
        marker = self.script.RATIO_MET_TAG
        action = ("add_tags", 1, torrent["hash"], [marker])
        self.run_torrent(torrent)
        self.assertNotIn(action, self.ctx.actions)  # Strictly greater than 3.1.
        self.ctx.actions.clear()
        torrent["uploaded"] = 3201
        self.run_torrent(torrent)
        self.assertIn(action, self.ctx.actions)
        self.assertEqual(self.limits(), [200 * 1024])

        self.ctx.actions.clear()
        self.ctx.time = torrent["added_on"] + self.script.RELEASE_AGE
        self.run_torrent(torrent)  # Backfill/retry even after release.
        self.assertIn(action, self.ctx.actions)
        self.assertEqual(self.limits(), [450 * 1024**2])

        self.ctx.actions.clear()
        torrent["tags"] += "," + marker
        self.run_torrent(torrent)
        self.assertNotIn(action, self.ctx.actions)
        self.assertFalse(any(a[0] == "remove_tags" and marker in a[-1] for a in self.ctx.actions))

    def test_historical_ratio_does_not_add_delta_completion_marker(self):
        torrent = self.torrent(4000, uploaded=4000)
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [200 * 1024])
        self.assertFalse(any(a[0] == "add_tags" and self.script.RATIO_MET_TAG in a[-1] for a in self.ctx.actions))

    def test_all_stages_replace_legacy_and_stale_labels(self):
        cases = [
            (100, 0.5, 100, {}, "downloading"),
            (100, 1, 100, {}, "complete"),
            (3000, 1, 100, {}, "waiting"),
            (3601, 1, 100, {}, "delta"),
            (4000, 1, 3201, {"baseline": 100}, "delta_limited"),
            (4000, 1, 3999, {}, "total"),
            (4000, 1, 4000, {}, "total_limited"),
            (72 * 3600, 1, 5000, {}, "released"),
        ]
        for age, progress, uploaded, state, stage in cases:
            with self.subTest(stage=stage):
                self.ctx.state.clear()
                self.ctx.actions.clear()
                old_tags = self.script.LEGACY_TAGS + list(self.script.TAGS.values())
                torrent = self.torrent(age, progress=progress, uploaded=uploaded,
                                       tags=",".join(["unrelated"] + old_tags))
                self.ctx.state[f"torrent:1:{torrent['hash']}:{torrent['added_on']}"] = state
                self.run_torrent(torrent)
                removed = next(a[-1] for a in self.ctx.actions if a[0] == "remove_tags")
                remaining = set(["unrelated"] + old_tags) - set(removed)
                self.assertEqual(remaining, {"unrelated", self.script.TAGS[stage]})

    def test_stage_changes_even_when_limit_is_unchanged(self):
        torrent = self.torrent(3600, up_limit=450 * 1024**2,
                               tags="unrelated," + self.script.TAGS["waiting"])
        self.run_torrent(torrent)
        self.ctx.time += 1
        for age, previous, expected in [
            (3601, "waiting", "delta"),
            (72 * 3600, "delta", "released"),
        ]:
            self.ctx.time = torrent["added_on"] + age
            torrent["tags"] = "unrelated," + self.script.TAGS[previous]
            self.ctx.actions.clear()
            self.run_torrent(torrent)
            self.assertEqual(self.limits(), [])
            self.assertIn(("remove_tags", 1, torrent["hash"], [self.script.TAGS[previous]]), self.ctx.actions)
            self.assertIn(("add_tags", 1, torrent["hash"], [self.script.TAGS[expected]]), self.ctx.actions)

    def test_counter_reset_invalidates_baseline(self):
        torrent = self.torrent(3600)
        self.run_torrent(torrent)
        self.ctx.time += 1
        self.run_torrent(torrent)
        self.ctx.actions.clear()
        torrent["uploaded"] = 0
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [])
        self.assertTrue(next(iter(self.ctx.state.values()))["invalid"])

    def test_readded_torrent_has_independent_state(self):
        torrent = self.torrent(100)
        self.run_torrent(torrent)
        self.ctx.time += 20
        self.run_torrent(self.torrent(0))
        self.assertEqual(len(self.ctx.state), 2)

    def test_resume_managed_torrent_after_downtime_records_actual_time(self):
        torrent = self.torrent(3500)
        self.run_torrent(torrent)
        self.ctx.time += 150
        self.run_torrent(torrent)
        state = next(iter(self.ctx.state.values()))
        self.assertEqual(state["baseline_at"], self.ctx.time)

    def test_baseline_window_boundaries(self):
        for age in [3601, 3719, 3720, 3721]:
            with self.subTest(age=age):
                self.ctx.state.clear()
                self.ctx.actions.clear()
                torrent = self.torrent(age, uploaded=4000)
                self.run_torrent(torrent)
                state = next(iter(self.ctx.state.values()))
                self.assertEqual("baseline" in state, age < 3720)
                self.assertEqual(any(a[0] == "reannounce" for a in self.ctx.actions), age < 3720)
                self.assertEqual(self.limits(), [450 * 1024**2 if age < 3720 else 200 * 1024])

    def test_managed_missing_baseline_expires_after_downtime(self):
        torrent = self.torrent(3500, uploaded=4000)
        self.run_torrent(torrent)
        self.ctx.time += 220
        self.ctx.actions.clear()
        self.run_torrent(torrent)
        state = next(iter(self.ctx.state.values()))
        self.assertNotIn("baseline", state)
        self.assertTrue(state["historical"])
        self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))
        self.assertEqual(self.limits(), [200 * 1024])

    def test_slow_baseline_read_cannot_capture_after_deadline(self):
        torrent = self.torrent(3719, uploaded=4000)
        def slow_read(*args):
            self.ctx.time += 2
            return copy.deepcopy(torrent)
        self.ctx.torrent = slow_read
        self.run_torrent(torrent)
        state = next(iter(self.ctx.state.values()))
        self.assertNotIn("baseline", state)
        self.assertTrue(state["historical"])
        self.assertEqual(self.limits(), [200 * 1024])
        self.ctx.actions.clear()
        self.run_torrent(torrent)
        self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))

    def test_failed_reannounce_does_not_create_baseline(self):
        torrent = self.torrent(3600)
        self.run_torrent(torrent)
        self.ctx.time += 1
        def fail(*args):
            raise RuntimeError("offline")
        self.ctx.reannounce = fail
        with self.assertRaises(RuntimeError):
            self.run_torrent(torrent)
        self.assertNotIn("baseline", next(iter(self.ctx.state.values())))


    def test_unchanged_stage_is_quiet_and_names_are_present(self):
        torrent = self.torrent(100, up_limit=2 * 1024**2,
                               tags="unrelated," + self.script.TAGS["downloading"])
        self.script.process(1, torrent, "Example client")
        self.assertTrue(any("Example client" in line and "Synthetic torrent" in line for line in self.ctx.logs))
        self.ctx.logs.clear()
        self.script.process(1, torrent, "Example client")
        self.assertEqual(self.ctx.logs, [])
        torrent["progress"] = 1
        self.script.process(1, torrent, "Example client")
        self.assertTrue(any(self.script.TAGS["downloading"] + " → " + self.script.TAGS["complete"] in line for line in self.ctx.logs))

    def test_historical_mode_persists_without_fabricating_baseline(self):
        torrent = self.torrent(4000, uploaded=4000, up_limit=200 * 1024,
                               tags="unrelated," + self.script.TAGS["total_limited"])
        # Old skip diagnostics must not prevent management under the new rule.
        self.ctx.state[f"hdsky-log:1:{torrent['hash']}:{torrent['added_on']}"] = "old skip"
        self.assertEqual(self.script.process(1, torrent), "historical")
        self.assertTrue(any("4.000000 >= 4" in line for line in self.ctx.logs))
        self.ctx.logs.clear()
        self.ctx.time += 10
        self.assertEqual(self.script.process(1, torrent), "historical")
        self.assertEqual(self.ctx.logs, [])
        self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))
        state = self.ctx.state[f"torrent:1:{torrent['hash']}:{torrent['added_on']}"]
        self.assertNotIn("baseline", state)
        torrent["uploaded"] = 3000
        self.run_torrent(torrent)
        self.assertTrue(state["ratio_limited"])
        self.ctx.time = torrent["added_on"] + 72 * 3600
        self.ctx.actions.clear()
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [450 * 1024**2])
        self.assertIn(("remove_tags", 1, torrent["hash"], [self.script.TAGS["total_limited"]]), self.ctx.actions)

    def test_historical_reaches_four_later_and_baseline_mode_stays_separate(self):
        torrent = self.torrent(4000, uploaded=3999)
        self.run_torrent(torrent)
        self.ctx.actions.clear()
        torrent["uploaded"] = 4000
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [200 * 1024])
        self.ctx.state.clear()
        self.ctx.actions.clear()
        torrent = self.torrent(3600, uploaded=1000)
        self.run_torrent(torrent)
        self.ctx.time += 1
        self.run_torrent(torrent)
        torrent["uploaded"] = 4000  # Total ratio 4, but baseline delta ratio only 3.
        self.ctx.actions.clear()
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [450 * 1024**2])

    def test_ratio_log_contains_evidence_and_only_triggers_once(self):
        torrent = self.torrent(3600)
        self.run_torrent(torrent)
        self.ctx.time += 1
        self.run_torrent(torrent)
        torrent["uploaded"] = 3201
        self.run_torrent(torrent)
        self.run_torrent(torrent)
        logs = [line for line in self.ctx.logs if "首次触发增量分享率" in line]
        self.assertEqual(len(logs), 1)
        self.assertIn("3.101000 > 3.1", logs[0])
        self.assertIn("100 字节", logs[0])
        self.assertIn("3101 字节", logs[0])

    def test_failed_write_does_not_report_transition_success(self):
        torrent = self.torrent(100)
        def fail(*args):
            raise RuntimeError("offline")
        self.ctx.add_tags = fail
        with self.assertRaisesRegex(RuntimeError, "添加规则标签失败"):
            self.run_torrent(torrent)
        self.assertFalse(any("规则状态已确认" in line for line in self.ctx.logs))
        self.assertNotIn("logged_stage", next(iter(self.ctx.state.values())))
        self.ctx.add_tags = lambda *args: None
        self.run_torrent(torrent)
        self.assertTrue(any("规则状态已确认" in line for line in self.ctx.logs))

    def test_preview_logs_do_not_claim_actual_success(self):
        self.ctx.dry_run = True
        torrent = self.torrent(3600)
        self.run_torrent(torrent)
        self.ctx.time += 1
        self.run_torrent(torrent)
        self.assertTrue(all(line.startswith("[试运行]") for line in self.ctx.logs))
        self.assertTrue(any("基准模拟记录" in line for line in self.ctx.logs))
        self.assertFalse(any("已确认" in line or "请求已提交" in line for line in self.ctx.logs))


if __name__ == "__main__":
    unittest.main()
