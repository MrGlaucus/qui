#!/usr/bin/env python3
import json
import subprocess
import sys
import os

cultures = ["de", "fr", "it", "cs", "pt-BR", "ko", "uk"]

def get_upstream_json(culture):
    """Get upstream JSON content for a culture"""
    try:
        result = subprocess.run(
            ["git", "show", f"upstream/develop:web/src/i18n/locales/{culture}/torrents.json"],
            capture_output=True, check=True, cwd=r"F:\qui", encoding='utf-8'
        )
        return json.loads(result.stdout)
    except Exception as e:
        print(f"Error getting upstream for {culture}: {e}")
        return None

def fix_culture(culture):
    our_path = f"F:\\qui\\web\\src\\i18n\\locales\\{culture}\\torrents.json"
    
    # Read our version
    try:
        with open(our_path, 'r', encoding='utf-8') as f:
            our_json = json.load(f)
    except Exception as e:
        print(f"Error reading our {culture}: {e}")
        return False
    
    # Get upstream version
    upstream_json = get_upstream_json(culture)
    if not upstream_json:
        return False
    
    # Get manualCrossSeed from upstream (it's at top level, not under torrents)
    manual_cross_seed = upstream_json.get("manualCrossSeed")
    if not manual_cross_seed:
        print(f"No manualCrossSeed in upstream for {culture}")
        return False
    
    # Insert manualCrossSeed into our JSON at top level after reportDialog
    # Our JSON has: page, addRoute, managementBar, ..., torrents, reportDialog, manualCrossSeed?, columnSync
    keys = list(our_json.keys())
    
    # Find position of reportDialog
    report_dialog_index = -1
    for i, key in enumerate(keys):
        if key == "reportDialog":
            report_dialog_index = i
            break
    
    if report_dialog_index == -1:
        print(f"No reportDialog in {culture}")
        return False
    
    # Build new ordered dict at top level
    new_json = {}
    for i, key in enumerate(keys):
        new_json[key] = our_json[key]
        if i == report_dialog_index:
            # Insert manualCrossSeed after reportDialog
            new_json["manualCrossSeed"] = manual_cross_seed
    
    # Write back with proper formatting
    try:
        with open(our_path, 'w', encoding='utf-8') as f:
            json.dump(new_json, f, ensure_ascii=False, indent=2)
        print(f"Fixed: {culture}")
        return True
    except Exception as e:
        print(f"Error writing {culture}: {e}")
        return False

if __name__ == "__main__":
    cultures = ["de", "fr", "it", "cs", "pt-BR", "ko", "uk"]
    for culture in cultures:
        try:
            if fix_culture(culture):
                print(f"OK {culture}")
            else:
                print(f"FAIL {culture}")
        except Exception as e:
            print(f"FAIL {culture}: {e}")