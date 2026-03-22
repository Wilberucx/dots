from __future__ import annotations
import subprocess
from pathlib import Path
from typing import Optional

from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules, LinkStatus, get_module_variant_info


class DotsService:
    def __init__(self, config: DotsConfig):
        self.config = config
        self.module_statuses: dict[str, list[LinkStatus]] = {}
        self.module_names: list[str] = []
        self.module_link_state: dict[str, str] = {}
        self.module_destinations: dict[str, str] = {}
        self.module_last_backup: dict[str, str] = {}
        self.module_has_variants: dict[str, bool] = {}
        self.module_variants: dict[str, list[str]] = {}
        self.module_active_variant: dict[str, str] = {}
        self.backup_entries: list[str] = []

    def refresh_modules(self):
        mods = resolve_modules(self.config)
        self.module_statuses = mods
        self.module_names = sorted(mods.keys())

        # Also include module dirs without statuses
        for d in self.config.get_module_dirs():
            if d.name not in self.module_names:
                self.module_names.append(d.name)
                self.module_statuses[d.name] = []
        self.module_names.sort()

        home = str(Path.home())
        sh = lambda p: "~" + str(p)[len(home) :] if str(p).startswith(home) else str(p)

        for name in self.module_names:
            sts = self.module_statuses.get(name, [])

            # Aggregate link state
            if not sts:
                self.module_link_state[name] = "unlinked"
            elif any(s.state in ("conflict", "unsafe") for s in sts):
                self.module_link_state[name] = "broken"
            elif all(s.state == "linked" for s in sts):
                self.module_link_state[name] = "linked"
            elif any(s.state == "linked" for s in sts):
                self.module_link_state[name] = "linked"
            else:
                self.module_link_state[name] = "unlinked"

            # Primary destination
            if sts:
                self.module_destinations[name] = sh(sts[0].destination)
            else:
                self.module_destinations[name] = ""

            # Last backup (git commit message)
            try:
                r = subprocess.run(
                    ["git", "log", "-1", "--format=%s", "--", name],
                    cwd=self.config.repo_root,
                    capture_output=True,
                    text=True,
                    timeout=2,
                )
                msg = r.stdout.strip() if r.returncode == 0 else ""
                self.module_last_backup[name] = msg if msg else ""
            except Exception:
                self.module_last_backup[name] = ""

            # Variants
            try:
                vinfo = get_module_variant_info(self.config, name)
                if vinfo and vinfo.has_variants:
                    self.module_has_variants[name] = True
                    self.module_variants[name] = vinfo.variants

                    # Deduce active variant
                    active_vars = set()
                    mod_dir = self.config.repo_root / name
                    for s in sts:
                        try:
                            rel = s.source.relative_to(mod_dir)
                            part = rel.parts[0] if rel.parts else ""
                            if part in vinfo.variants:
                                active_vars.add(part)
                        except ValueError:
                            pass
                    if active_vars:
                        self.module_active_variant[name] = list(active_vars)[0]
                    else:
                        self.module_active_variant[name] = vinfo.default_variant
                else:
                    self.module_has_variants[name] = False
                    self.module_variants[name] = []
                    self.module_active_variant[name] = ""
            except Exception:
                self.module_has_variants[name] = False
                self.module_variants[name] = []
                self.module_active_variant[name] = ""

    def refresh_backups(self):
        try:
            r = subprocess.run(
                ["git", "log", "--oneline", "-30"],
                cwd=self.config.repo_root,
                capture_output=True,
                text=True,
                timeout=5,
            )
            if r.returncode == 0:
                self.backup_entries = [
                    l.strip() for l in r.stdout.strip().splitlines() if l.strip()
                ]
            else:
                self.backup_entries = ["(no git history)"]
        except Exception:
            self.backup_entries = ["(git unavailable)"]
