"""
dots show — tabla inline de módulos, archivos y estado de symlinks.
Filtrable por --module/-m, --type/-t y --state/-s.
"""
import typer
from typing import Optional, List
from rich.table import Table
from rich.text import Text
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules, LinkState
from dots.core.yaml_parser import parse_module_meta
from dots.ui.output import console, print_header, print_warning


# Mapeo de estados a label + color (consistente con status.py)
STATE_STYLE: dict[str, tuple[str, str]] = {
    "linked":   ("linked",   "green"),
    "pending":  ("unlinked", "dim"),
    "conflict": ("broken",   "red"),
    "missing":  ("missing",  "yellow"),
    "unsafe":   ("unsafe",   "red"),
}


def show_cmd(
    module: Optional[List[str]] = typer.Option(
        None, "--module", "-m",
        help="Filter by module name (repeatable)"
    ),
    type: Optional[List[str]] = typer.Option(
        None, "--type", "-t",
        help="Filter by module type declared in path.yaml (repeatable)"
    ),
    state: Optional[List[str]] = typer.Option(
        None, "--state", "-s",
        help="Filter by link state: linked, unlinked, broken, missing, unsafe (repeatable)"
    ),
):
    """
    Show all managed dotfiles in a table with their link state.
    Filterable by module, type, and state.
    """
    print_header("Dotfiles")

    config = DotsConfig.load()
    all_modules = resolve_modules(
        config,
        modules=list(module) if module else None,
        types=list(type) if type else None,
    )

    if not all_modules:
        print_warning("No modules found.")
        raise typer.Exit(0)

    # Normalizar --state a LinkState internos
    # "unlinked" es alias de "pending" en la UI
    state_filter: set[str] | None = None
    if state:
        state_filter = set()
        for s in state:
            if s == "unlinked":
                state_filter.add("pending")
            elif s == "broken":
                state_filter.add("conflict")
            else:
                state_filter.add(s)

    table = Table(
        show_header=True,
        header_style="bold",
        box=None,
        padding=(0, 1),
    )
    table.add_column("Module", style="bold", no_wrap=True)
    table.add_column("Source", no_wrap=True)
    table.add_column("Destination", no_wrap=True)
    table.add_column("State", no_wrap=True)
    table.add_column("Type", style="dim", no_wrap=True)

    total_shown = 0

    for module_name, statuses in sorted(all_modules.items()):
        # Leer type del módulo
        yaml_path = config.repo_root / module_name / "path.yaml"
        meta = parse_module_meta(yaml_path)
        module_type = meta.get("type", "")

        first_row = True
        for st in statuses:
            # Aplicar filtro de estado
            if state_filter and st.state not in state_filter:
                continue

            label, color = STATE_STYLE.get(st.state, (st.state, "white"))
            state_text = Text(f"● {label}", style=color)

            # Nombre del módulo solo en la primera fila del grupo
            mod_cell = module_name if first_row else ""
            type_cell = module_type if first_row else ""
            first_row = False

            table.add_row(
                mod_cell,
                st.source.name,
                str(st.destination).replace(str(config.home_dir), "~"),
                state_text,
                type_cell,
            )
            total_shown += 1

    if total_shown == 0:
        print_warning("No dotfiles match the given filters.")
        raise typer.Exit(0)

    console.print(table)
    console.print(f"\n[dim]Total: {total_shown} files[/dim]")
