---
sidebar_position: 10
title: Tracker Customizations
description: Give trackers friendly display names and merge multiple announce domains into one entry.
---

# Tracker customizations

qui identifies trackers by their announce domain (`tracker.example.com`). A tracker customization maps one or more domains to a friendly display name (`MyTracker`). qui then shows the display name in place of the raw domain.

Display names apply across all your instances. You configure them once.

## Where to find it

Tracker customizations live on the **Dashboard**, in the **Tracker Breakdown** section. There is no Settings page for them.

Expand **Tracker Breakdown** to see the table of trackers. The rename, merge, edit, and delete actions are the row actions in that table.

## Rename a tracker

1. Open the **Dashboard** and expand **Tracker Breakdown**.
2. Hover the tracker row. If you are on mobile, tap the row to open its drawer.
3. Click the pencil icon (**Rename**).
4. Enter a **Display Name** and save.

## Merge trackers

If a tracker announces on several domains, you can combine the domains into a single entry:

1. Tick the checkbox on each tracker row that you want to combine.
2. Click the link icon on one of the selected rows (**Add to merge**).
3. Enter the **Display Name** for the merged entry and save.

The merge dialog marks the first domain as the **Display domain**. qui uses it for presentation, including the group icon. It does not control which torrents count.

Dashboard statistics use the union of the torrents announced on every domain in the group. If the same torrent appears on several domains, qui counts it once. Torrents that appear on only one of the domains still count. Tracker announce status does not change group membership, so a tracker returning `-1` does not make the totals jump between domains.

Traffic history remains associated with each announce domain and follows the current grouping. Renaming or regrouping domains keeps their history. Newly observed torrents establish a counter baseline first, so traffic from before collection began is not added to the current day.

If you want to add another domain to an existing group later, select the domain and click the link icon on the group's row.

## Edit or delete

Rows that already have a customization show a pencil (**Edit**) and a trash icon (**Delete**) on hover. If you delete a customization, those trackers revert to their raw domains.

## Import and export

The **Tracker Breakdown** header has import and export buttons.

**Export** copies all customizations to the clipboard as JSON. **Import** accepts the same JSON and reports new entries, conflicts, and unchanged entries before it applies them. For each conflict, you choose **Skip** or **Overwrite**.

```json
{
  "comment": "qui tracker customizations for Dashboard",
  "trackerCustomizations": [
    {
      "displayName": "MyTracker",
      "domains": ["tracker.example.com", "tracker2.example.com"]
    }
  ]
}
```

The first entry in `domains` is the display domain. All domains participate in the deduplicated Dashboard statistics.

## Where display names are used

- **Dashboard** statistics and tracker breakdown.
- **[Automations](./automations.md)**: the **Tracker** condition matches your display name as well as the raw URL or domain, tag actions can tag torrents with it, and move paths can use it with `{{.Tracker}}`. See [Move](./automations.md#move) for when `{{.Tracker}}` uses the display name.
- **[Cross-seed link directories](./cross-seed/link-directories.md)**: the `by-tracker` preset uses the display name for folder names.
