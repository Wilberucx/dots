import json
import threading
import time
from pathlib import Path
from importlib.metadata import version as get_current_version
import requests
from dots.ui.output import console

CACHE_FILE = Path.home() / ".cache" / "dots" / "update.json"
GITHUB_API_URL = "https://api.github.com/repos/cantoarch/dots/releases/latest"

def is_newer(latest: str, current: str) -> bool:
    """Simple semantic version comparison."""
    try:
        l = tuple(map(int, latest.lstrip('v').split('.')))
        c = tuple(map(int, current.lstrip('v').split('.')))
        return l > c
    except (ValueError, AttributeError):
        return False

def _fetch_and_cache():
    """Fetch latest version from GitHub and save to cache."""
    try:
        # Ensure cache directory exists
        CACHE_FILE.parent.mkdir(parents=True, exist_ok=True)

        # Timeout 1.5s to be as non-intrusive as possible
        response = requests.get(GITHUB_API_URL, timeout=1.5)
        if response.status_code == 200:
            data = response.json()
            latest_version = data.get("tag_name", "").lstrip("v")
            
            if latest_version:
                cache_data = {
                    "latest_version": latest_version,
                    "last_check": time.time()
                }
                with open(CACHE_FILE, "w") as f:
                    json.dump(cache_data, f)
    except Exception:
        # Silently fail on network issues or API errors
        pass

def check_for_updates_async():
    """Starts a background thread to check for updates if cache is expired (>24h)."""
    should_check = True
    if CACHE_FILE.exists():
        try:
            with open(CACHE_FILE, "r") as f:
                cache = json.load(f)
                last_check = cache.get("last_check", 0)
                # Only check once every 24 hours
                if time.time() - last_check < 86400:
                    should_check = False
        except (json.JSONDecodeError, KeyError):
            pass
    
    if should_check:
        thread = threading.Thread(target=_fetch_and_cache, daemon=True)
        thread.start()

def notify_if_needed():
    """Prints a notification if a newer version is found in cache."""
    if not CACHE_FILE.exists():
        return

    try:
        with open(CACHE_FILE, "r") as f:
            cache = json.load(f)
            latest = cache.get("latest_version")
            current = get_current_version("dots")

            if latest and is_newer(latest, current):
                console.print()
                console.print(
                    f"[dim]✨ A new version of dots is available: "
                    f"[bold green]v{latest}[/] (current: v{current})[/]"
                )
                console.print(f"[dim]   Update with: [italic]pipx upgrade dots[/][/]")
    except Exception:
        # Fail silently to avoid interrupting the user's flow
        pass
