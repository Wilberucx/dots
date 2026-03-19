import typer
from enum import Enum
from rich.tree import Tree
from dots.ui.output import console, print_header, print_warning
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules, get_active_variant
from dots.core.yaml_parser import detect_variants, parse_path_yaml


class OutputFormat(str, Enum):
    default = "default"
    table = "table"
    json = "json"


def status_cmd(
    module: list[str] | None = typer.Option(
        None, "--module", "-m",
        help="Show status only for specific modules (repeatable)"
    ),
    type: list[str] | None = typer.Option(
        None, "--type", "-t",
        help="Show status only for modules of this type (repeatable)"
    ),
    state: list[str] | None = typer.Option(
        None, "--state", "-s",
        help="Show only modules in this state: linked, unlinked, broken, missing, unsafe (repeatable)"
    ),
    format: OutputFormat = typer.Option(
        OutputFormat.default,
        "--format", "-f",
        help="Output format: default, table, json"
    ),
):
    """
    Show status of all dotfiles modules grouped by state.
    """
    config = DotsConfig.load()
    all_modules = resolve_modules(config, modules=module, types=type)

    if not all_modules:
        console.print("[yellow]⚠[/yellow] No modules found.")
        return

    state_filter = None
    if state:
        state_filter = set()
        for s in state:
            if s == "unlinked":
                state_filter.add("pending")
            elif s == "broken":
                state_filter.add("conflict")
            else:
                state_filter.add(s)

    if format == OutputFormat.table:
        _render_table(all_modules, state_filter, config)
    elif format == OutputFormat.json:
        _render_json(all_modules, state_filter, config)
    else:
        print_header("Dots Status")
        _render_default(all_modules, state_filter, config)


def _render_default(
    all_modules: dict,
    state_filter: set[str] | None,
    config,
) -> None:
    """Output actual agrupado por estado."""

    # Categorize modules
    linked = []
    broken = []
    missing_src = []
    unlinked = []
    not_linked = []

    for module_name, statuses in all_modules.items():
        module_linked = sum(1 for s in statuses if s.state == "linked")
        module_broken = sum(1 for s in statuses if s.state in ("conflict", "unsafe"))
        module_missing = sum(1 for s in statuses if s.state == "missing")
        module_pending = sum(1 for s in statuses if s.state == "pending")

        # Categorize module by worst state
        if module_broken > 0:
            if not state_filter or "conflict" in state_filter or "unsafe" in state_filter:
                reason = []
                conflicts = sum(1 for s in statuses if s.state == "conflict")
                unsafe = sum(1 for s in statuses if s.state == "unsafe")
                if conflicts > 0:
                    reason.append(f"{conflicts} conflict{'s' if conflicts > 1 else ''}")
                if unsafe > 0:
                    reason.append(f"{unsafe} unsafe path{'s' if unsafe > 1 else ''}")
                broken.append((module_name, ", ".join(reason)))
        elif module_missing > 0:
            if not state_filter or "missing" in state_filter:
                missing_src.append((module_name, f"{module_missing} missing source{'s' if module_missing > 1 else ''}"))
        elif module_pending > 0:
            if not state_filter or "pending" in state_filter:
                unlinked.append((module_name, f"{module_pending} unlinked"))
        elif module_linked > 0:
            if not state_filter or "linked" in state_filter:
                linked.append((module_name, f"{module_linked} linked"))
        else:
            not_linked.append((module_name, "no files to link"))

    # Display results
    if linked:
        linked_tree = Tree(f"[success]✔ Linked ({len(linked)} modules)[/success]")
        for name, info in linked:
            # Check if module has variants
            yaml_path = config.repo_root / name / "path.yaml"
            mappings = parse_path_yaml(yaml_path, config.current_os)
            variant_info = detect_variants(mappings)

            if variant_info.has_variants:
                # Expand with flavor indicators
                module_branch = linked_tree.add(f"[dim]{name}[/dim]")
                active = get_active_variant(config, name)
                for v in variant_info.variants:
                    if v == active:
                        module_branch.add(f"[success]●[/success] [bold]{v}[/bold] [dim]← active[/dim]")
                    else:
                        module_branch.add(f"[dim]○ {v}[/dim]")
            else:
                linked_tree.add(f"[dim]{name}[/dim] [success]({info})[/success]")
        console.print(linked_tree)
        console.print()

    if unlinked:
        unlinked_tree = Tree(f"[dim]ℹ Unlinked ({len(unlinked)} modules)[/dim]")
        for name, reason in unlinked:
            yaml_path = config.repo_root / name / "path.yaml"
            mappings = parse_path_yaml(yaml_path, config.current_os)
            variant_info = detect_variants(mappings)

            if variant_info.has_variants:
                module_branch = unlinked_tree.add(f"[dim]{name}[/dim]")
                for v in variant_info.variants:
                    module_branch.add(f"[dim]○ {v}[/dim]")
            else:
                unlinked_tree.add(f"[dim]{name}[/dim] [dim]({reason})[/dim]")
        console.print(unlinked_tree)
        console.print()

    if broken:
        broken_tree = Tree(f"[error]✖ Broken ({len(broken)} modules)[/error]")
        for name, reason in broken:
            broken_tree.add(f"[dim]{name}[/dim] [error]({reason})[/error]")
        console.print(broken_tree)
        console.print()

    if missing_src:
        missing_tree = Tree(f"[warning]⚠ Missing Source ({len(missing_src)} modules)[/warning]")
        for name, reason in missing_src:
            missing_tree.add(f"[dim]{name}[/dim] [warning]({reason})[/warning]")
        console.print(missing_tree)
        console.print()

    if not_linked:
        not_linked_tree = Tree(f"[dim]• Empty ({len(not_linked)} modules)[/dim]")
        for name, reason in not_linked:
            not_linked_tree.add(f"[dim]{name}[/dim] [dim]({reason})[/dim]")
        console.print(not_linked_tree)
        console.print()

    # Summary
    console.print("━" * console.width)
    summary_parts = []
    if linked:
        summary_parts.append(f"[success]{len(linked)} linked[/success]")
    if unlinked:
        summary_parts.append(f"[dim]{len(unlinked)} unlinked[/dim]")
    if broken:
        summary_parts.append(f"[error]{len(broken)} broken[/error]")
    if missing_src:
        summary_parts.append(f"[warning]{len(missing_src)} missing[/warning]")
    if not_linked:
        summary_parts.append(f"[dim]{len(not_linked)} empty[/dim]")

    console.print(f"[bold]Summary:[/bold] {' • '.join(summary_parts)}")


def _render_table(
    all_modules: dict,
    state_filter: set[str] | None,
    config,
) -> None:
    """Tabla Rich con columnas Module/Source/Destination/State/Type."""
    from rich.table import Table
    from rich.text import Text
    from dots.core.yaml_parser import parse_module_meta

    STATE_STYLE = {
        "linked":   ("linked",   "green"),
        "pending":  ("unlinked", "dim"),
        "conflict": ("broken",   "red"),
        "missing":  ("missing",  "yellow"),
        "unsafe":   ("unsafe",   "red"),
    }

    table = Table(
        show_header=True,
        header_style="bold",
        box=None,
        padding=(0, 1),
    )
    table.add_column("Module",      style="bold", no_wrap=True)
    table.add_column("Source",      no_wrap=True)
    table.add_column("Destination", no_wrap=True)
    table.add_column("State",       no_wrap=True)
    table.add_column("Type",        style="dim", no_wrap=True, min_width=10)

    total = 0

    for module_name, statuses in sorted(all_modules.items()):
        yaml_path = config.repo_root / module_name / "path.yaml"
        meta = parse_module_meta(yaml_path)
        module_type = meta.get("type", "")

        first_row = True
        for st in statuses:
            if state_filter and st.state not in state_filter:
                continue

            label, color = STATE_STYLE.get(st.state, (st.state, "white"))
            state_text = Text(f"● {label}", style=color)

            mod_cell  = module_name  if first_row else ""
            type_cell = module_type  if first_row else ""
            first_row = False

            table.add_row(
                mod_cell,
                st.source.name,
                str(st.destination).replace(str(config.home_dir), "~"),
                state_text,
                type_cell,
            )
            total += 1

    if total == 0:
        print_warning("No dotfiles match the given filters.")
        return

    console.print(table)
    console.print(f"\n[dim]Total: {total} files[/dim]")


def _render_json(
    all_modules: dict,
    state_filter: set[str] | None,
    config,
) -> None:
    """JSON estructurado para scripting."""
    import json
    from dots.core.yaml_parser import parse_module_meta

    STATE_LABEL = {
        "linked":   "linked",
        "pending":  "unlinked",
        "conflict": "broken",
        "missing":  "missing",
        "unsafe":   "unsafe",
    }

    summary = {
        "linked": 0, "unlinked": 0, "broken": 0,
        "missing": 0, "unsafe": 0,
    }

    modules_data = {}

    for module_name, statuses in sorted(all_modules.items()):
        yaml_path = config.repo_root / module_name / "path.yaml"
        meta = parse_module_meta(yaml_path)

        files = []
        for st in statuses:
            if state_filter and st.state not in state_filter:
                continue

            label = STATE_LABEL.get(st.state, str(st.state))
            summary[label] = summary.get(label, 0) + 1

            files.append({
                "source":      st.source.name,
                "destination": str(st.destination).replace(
                    str(config.home_dir), "~"
                ),
                "state": label,
            })

        if files:
            modules_data[module_name] = {
                "type":  meta.get("type", None),
                "files": files,
            }

    output = {
        "modules": modules_data,
        "summary": summary,
    }

    console.print(json.dumps(output, indent=2))
