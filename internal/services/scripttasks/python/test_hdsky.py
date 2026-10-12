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
        self.params = {"tracker_domains": ["tracker.example.test"]}

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

    def test_historical_torrents_are_completely_untouched(self):
        for age in [3601, 3602, 72 * 3600, 90 * 3600]:
            with self.subTest(age=age):
                self.run_torrent(self.torrent(age))
                self.assertEqual(self.ctx.actions, [])
                self.assertFalse(any(key.startswith("torrent:") for key in self.ctx.state))

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
                               tags="unrelated,50分钟内下载中限速,50分钟内下载完成限速")
        self.run_torrent(torrent)
        self.assertEqual(self.limits(), [])
        self.assertIn(("remove_tags", 1, torrent["hash"], ["50分钟内下载完成限速"]), self.ctx.actions)
        self.assertFalse(any(a[0] == "add_tags" for a in self.ctx.actions))

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
        self.ctx.time += 1000
        self.run_torrent(torrent)
        state = next(iter(self.ctx.state.values()))
        self.assertEqual(state["baseline_at"], self.ctx.time)

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
                               tags="unrelated,50分钟内下载中限速")
        self.script.process(1, torrent, "Example client")
        self.assertTrue(any("Example client" in line and "Synthetic torrent" in line for line in self.ctx.logs))
        self.ctx.logs.clear()
        self.script.process(1, torrent, "Example client")
        self.assertEqual(self.ctx.logs, [])
        torrent["progress"] = 1
        self.script.process(1, torrent, "Example client")
        self.assertTrue(any("50分钟内下载中限速 → 50分钟内下载完成限速" in line for line in self.ctx.logs))

    def test_historical_skip_logged_once_without_becoming_managed(self):
        torrent = self.torrent(4000)
        self.assertEqual(self.script.process(1, torrent), "historical")
        self.assertIn("缺少上传基准", self.ctx.logs[-1])
        self.ctx.logs.clear()
        self.ctx.time += 10
        self.assertEqual(self.script.process(1, torrent), "historical")
        self.assertEqual(self.ctx.logs, [])
        self.assertFalse(any(a[0] == "reannounce" for a in self.ctx.actions))
        self.assertFalse(any(key.startswith("torrent:") for key in self.ctx.state))

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
