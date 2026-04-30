import typer
from pathlib import Path
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules, get_module_variant_info
from dots.ui.output import console

def list_cmd(
    linked: bool = typer.Option(False, "--linked", help="Show linked modules"),
    unlinked: bool = typer.Option(False, "--unlinked", help="Show unlinked modules"),
    broken: bool = typer.Option(False, "--broken", help="Show broken modules"),
    variant: bool = typer.Option(False, "--variant", help="Show variants"),
    bak: bool = typer.Option(False, "--bak", help="Show backup paths"),
):
    """
    List modules or backups with optional filters.
    """
    config = DotsConfig.load()
    all_modules = resolve_modules(config)

    results = set()

    # If no flags are provided, show all module names
    if not any([linked, unlinked, broken, variant, bak]):
        for module_name in all_modules:
            results.add(module_name)

    # Single pass over modules to evaluate all flags
    for module_name, statuses in all_modules.items():
        if linked and any(s.state == "linked" for s in statuses):
            results.add(module_name)

        if unlinked and any(s.state == "pending" for s in statuses):
            results.add(module_name)

        if broken and any(s.state in ("conflict", "unsafe") for s in statuses):
            results.add(module_name)

        if variant:
            vinfo = get_module_variant_info(config, module_name)
            if vinfo and vinfo.has_variants:
                for v in vinfo.variants:
                    results.add(f"{module_name}:{v}")

        if bak:
            for s in statuses:
                bak_path = Path(str(s.destination) + "-backup")
                if bak_path.exists():
                    results.add(str(bak_path))

    for item in sorted(results):
        console.print(item)
