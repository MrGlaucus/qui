"""HDSKY 刷流规则。通过 QUI 任务的作用 Tracker 选择站点。"""
from qui import ctx

LEGACY_TAGS = ["50分钟内下载中限速", "50分钟内下载完成限速", "50分钟后解除限速",
        "72小时内3倍分享率限速", "72小时后解除限速", "72小时内历史种子4倍分享率限速"]
TAGS = {
    "downloading": "HDSKY·50分钟内下载中·2MiB/s",
    "complete": "HDSKY·50分钟内已完成·200KiB/s",
    "waiting": "HDSKY·等待汇报基准·450MiB/s",
    "delta": "HDSKY·基准后增量刷流·450MiB/s",
    "delta_limited": "HDSKY·增量超3.1倍限速·200KiB/s",
    "total": "HDSKY·无基准总量刷流·450MiB/s",
    "total_limited": "HDSKY·总量达4倍限速·200KiB/s",
    "released": "HDSKY·已满72小时·450MiB/s",
}
RULE_TAGS = set(LEGACY_TAGS) | set(TAGS.values())
# Persistent completion marker, independent of the current speed-limit stage.
RATIO_MET_TAG = "HDSKY·分享率已满足"
BASELINE_AGE = 3601
BASELINE_DEADLINE = 62 * 60
RELEASE_AGE = 72 * 3600


def identity(instance_id, torrent, instance_name):
    name = str(torrent.get("name", torrent["hash"])).replace("\n", " ").replace("\r", " ")
    return f"实例「{instance_name or instance_id}」 / 种子「{name}」 [{torrent['hash']}]"


def log(message):
    prefix = "[试运行] " if ctx.dry_run else ""
    ctx.log(prefix + message)


def size(value):
    return f"{value / 1024**3:.3f} GiB（{value} 字节）"


def skip(instance_id, torrent, label, reason):
    # Skip diagnostics are separate from the torrent management state.
    key = f"hdsky-log:{instance_id}:{torrent['hash']}:{torrent['added_on']}"
    if ctx.state_get(key) != reason:
        ctx.state_set(key, reason)
        log(f"跳过：{label}；原因：{reason}")


def operation(label, description, function, *args):
    try:
        function(*args)
    except Exception as error:
        raise RuntimeError(f"{description}失败：{label}；{error}；此前成功的操作不会回滚") from error


def use_total_ratio(state, key, label):
    if not state.get("historical"):
        state["historical"] = True
        ctx.state_set(key, state)
        log(f"转入历史总量模式：{label}；已满 62 分钟仍无基准，"
            "使用当前上传量 / 种子总体积 >= 4；不补录基准、不再补汇报")


def process(instance_id, torrent, instance_name=""):
    label = identity(instance_id, torrent, instance_name)
    now = ctx.now()
    added = torrent["added_on"]
    if added <= 0 or added > now or torrent["total_size"] <= 0:
        skip(instance_id, torrent, label, f"添加时间或体积无效：added_on={added}，total_size={torrent['total_size']}")
        return "invalid"
    torrent_hash = torrent["hash"]
    key = f"torrent:{instance_id}:{torrent_hash}:{added}"
    state = ctx.state_get(key)
    age = now - added
    if state is None:
        state = {"managed_since": now}
        ctx.state_set(key, state)
        log(f"首次接管：{label}；添加后 {int(age)} 秒，进度 {torrent['progress']:.1%}，体积 {size(torrent['total_size'])}")
    if state.get("invalid"):
        skip(instance_id, torrent, label, "上传计数曾重置，原基准失效")
        return "invalid"
    if age >= BASELINE_DEADLINE and "baseline" not in state:
        use_total_ratio(state, key, label)
    if age >= BASELINE_AGE and "baseline" not in state and not state.get("historical"):
        # Capture fresh counters immediately after qBittorrent accepts reannounce.
        # Persist each step so a later limit/tag error does not repeat the request.
        if "announced_at" not in state:
            operation(label, "重新汇报", ctx.reannounce, instance_id, torrent_hash)
            state["announced_at"] = ctx.now()
            ctx.state_set(key, state)
            log(f"重新汇报请求{'将提交' if ctx.dry_run else '已提交'}：{label}；添加后 {int(age)} 秒；不代表站点已入账")
        if ctx.now() - added < BASELINE_DEADLINE:
            torrent = ctx.torrent(instance_id, torrent_hash)
            if torrent["added_on"] != added:
                skip(instance_id, {**torrent, "added_on": added}, label, "汇报后添加时间变化，种子可能已删除并重新添加")
                return "invalid"
        observed_at = ctx.now()
        age = observed_at - added
        if age >= BASELINE_DEADLINE:
            use_total_ratio(state, key, label)
        else:
            state["baseline"] = torrent["uploaded"]
            state["baseline_at"] = observed_at
            ctx.state_set(key, state)
            log(f"基准{'模拟记录' if ctx.dry_run else '已记录'}：{label}；上传 {size(state['baseline'])}；"
                f"记录时间 Unix {int(state['baseline_at'])}；"
                f"距 60 分 1 秒延迟 {max(0, int(state['baseline_at'] - added - BASELINE_AGE))} 秒")
    if "baseline" in state and torrent["uploaded"] < state["baseline"]:
        state["invalid"] = True
        ctx.state_set(key, state)
        log(f"上传计数重置：{label}；当前 {size(torrent['uploaded'])} < 基准 {size(state['baseline'])}")
        skip(instance_id, torrent, label, "上传计数曾重置，原基准失效")
        return "invalid"
    if age >= RELEASE_AGE:
        limit, tag = 450 * 1024**2, TAGS["released"]
    elif age < 50 * 60:
        complete = torrent["progress"] >= 1
        limit, tag = (200 * 1024, TAGS["complete"]) if complete else (2 * 1024**2, TAGS["downloading"])
        ctx.wake_at(added + 50 * 60)
    else:
        if "baseline" in state and (torrent["uploaded"] - state["baseline"]) / torrent["total_size"] > 3.1:
            if not state.get("ratio_limited"):
                state["ratio_limited"] = True
                ctx.state_set(key, state)
                delta = torrent["uploaded"] - state["baseline"]
                log(f"首次触发增量分享率限速：{label}；当前上传 {size(torrent['uploaded'])}；"
                    f"基准 {size(state['baseline'])}；增量 {size(delta)}；"
                    f"种子体积 {size(torrent['total_size'])}；"
                    f"增量分享率 {delta / torrent['total_size']:.6f} > 3.1；保持限速直到 72 小时")
        if state.get("historical") and not state.get("ratio_limited") and torrent["uploaded"] / torrent["total_size"] >= 4:
            state["ratio_limited"] = True
            ctx.state_set(key, state)
            log(f"首次触发历史总量分享率限速：{label}；当前上传 {size(torrent['uploaded'])}；"
                f"种子体积 {size(torrent['total_size'])}；"
                f"总量分享率 {torrent['uploaded'] / torrent['total_size']:.6f} >= 4；保持限速直到 72 小时")
        if state.get("ratio_limited"):
            limit = 200 * 1024
            tag = TAGS["total_limited"] if state.get("historical") else TAGS["delta_limited"]
        else:
            limit = 450 * 1024**2
            stage = "total" if state.get("historical") else "delta" if "baseline" in state else "waiting"
            tag = TAGS[stage]
        ctx.wake_at(added + RELEASE_AGE)
    if age < BASELINE_AGE:
        ctx.wake_at(added + BASELINE_AGE)
    changed = torrent["up_limit"] != limit
    if changed:
        operation(label, "设置上传限速", ctx.upload_limit, instance_id, torrent_hash, limit)
    tags = [t.strip() for t in torrent["tags"].split(",") if t.strip()]
    obsolete = [t for t in tags if t in RULE_TAGS and t != tag]
    if obsolete:
        operation(label, "移除旧规则标签", ctx.remove_tags, instance_id, torrent_hash, obsolete)
    if tag not in tags:
        operation(label, "添加规则标签", ctx.add_tags, instance_id, torrent_hash, [tag])
    if "baseline" in state and state.get("ratio_limited") and RATIO_MET_TAG not in tags:
        operation(label, "添加分享率达标标签", ctx.add_tags, instance_id, torrent_hash, [RATIO_MET_TAG])
        log(f"增量分享率达标标记{'计划添加' if ctx.dry_run else '已添加'}：{label}；「{RATIO_MET_TAG}」；72 小时解除限速后保留")
    if state.get("logged_stage") != tag or changed or obsolete or tag not in tags:
        previous = state.get("logged_stage", "未设置")
        speed = "200 KiB/s" if limit == 200 * 1024 else f"{limit // 1024**2} MiB/s"
        result = "计划应用" if ctx.dry_run else "已确认"
        log(f"规则状态{result}：{label}；{previous} → {tag}；添加后 {int(age)} 秒；"
            f"进度 {torrent['progress']:.1%}；上传限速 {speed}；"
            f"移除标签 {obsolete}；目标标签「{tag}」"
            + ("；修正与规则不一致的限速或标签" if previous == tag else ""))
        # Only mark the transition logged after all client operations succeed.
        state["logged_stage"] = tag
        ctx.state_set(key, state)
    return "historical" if state.get("historical") else "managed"


def main():
    domains = {host.strip().lower().rstrip(".") for host in ctx.tracker_domains}
    errors = 0
    counts = {"managed": 0, "historical": 0, "invalid": 0}
    scanned = matched = 0
    started = ctx.now()
    for instance in ctx.instances():
        try:
            torrents = ctx.torrents(instance["id"])
        except Exception as error:
            errors += 1
            log(f"实例「{instance['name']}」 [{instance['id']}] 读取失败：{error}")
            continue
        for torrent in torrents:
            scanned += 1
            try:
                hosts = set(torrent["tracker_hosts"])
                if not hosts.intersection(domains) and not torrent.get("trackers_complete", False):
                    hosts.update(ctx.trackers(instance["id"], torrent["hash"]))
                if hosts.intersection(domains):
                    matched += 1
                    counts[process(instance["id"], torrent, instance["name"])] += 1
            except Exception as error:
                errors += 1
                log(f"处理失败：{identity(instance['id'], torrent, instance['name'])}；{error}")
    log(f"本轮完成：扫描 {scanned}，站点匹配 {matched}，接管 {counts['managed'] + counts['historical']}，"
        f"其中历史总量模式 {counts['historical']}，跳过无效状态 {counts['invalid']}，"
        f"失败 {errors}；耗时 {ctx.now() - started:.2f} 秒")
    if errors:
        raise RuntimeError(f"{errors} 项处理失败，请检查上方日志")


if __name__ == "__main__":
    main()
