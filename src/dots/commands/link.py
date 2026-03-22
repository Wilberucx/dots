import typer
import shutil
from pathlib import Path
from rich.tree import Tree
from dots.ui.output import console, print_header, print_warning, print_error, print_info
from dots.core.config import DotsConfig
from dots.core.resolver import (
    resolve_modules,
    get_module_variant_info,
    get_module_available_sources,
    get_active_variant,
)
from dots.core.yaml_parser import detect_variants, parse_path_yaml
from dots.core.transaction import TransactionLog
from dots.ui.selector import select_modules


def _get_effective_variant(config, module_name: str, variant: str | None) -> str | None:
    """Returns the variant that will be used — explicit or cascade default."""
    if variant:
        return variant
    yaml_path = config.repo_root / module_name / "path.yaml"
    if not yaml_path.exists():
        return None
    mappings = parse_path_yaml(yaml_path, config.current_os)
    vinfo = detect_variants(mappings)
    if vinfo.has_variants:
        return vinfo.default_variant
    return None


def link_cmd(
    module: list[str] | None = typer.Option(
        None,
        "--module",
        "-m",
        help="Link only specific modules (repeatable: -m Zsh -m Nvim)",
    ),
    type: list[str] | None = typer.Option(
        None,
        "--type",
        "-t",
        help="Link only modules of this type (repeatable: -t minimal -t work)",
    ),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would happen"),
    force: bool = typer.Option(False, "--force", help="Overwrite existing symlinks"),
    interactive: bool = typer.Option(
        False, "--interactive", "-i", help="Interactively select modules to link"
    ),
    variant: str | None = typer.Option(
        None,
        "--variant",
        help="Specific variant to use (when module has multiple variants sharing same destination)",
    ),
):
    """
    Create symlinks for dotfiles modules.

    By default, links all modules. Use --interactive to select specific modules.
    """
    print_header("Linking Dotfiles")

    config = DotsConfig.load()

    # Handle module selection
    if module:
        selected_modules = module
        print_info(f"Linking specified modules: {', '.join(selected_modules)}")
    elif interactive:
        selected_modules = select_modules(config, preselect_all=True)
        if not selected_modules:
            print_info("No modules selected.")
            return
        print_info(f"Selected {len(selected_modules)} module(s)")
        console.print()
    else:
        selected_modules = None  # Link all modules

    # If variant is specified, module must also be specified
    if variant and not selected_modules:
        print_error("When using --variant, you must specify the module name.")
        print_info("Example: dots link -m Nvim --variant notevim")
        raise typer.Exit(1)

    # When --variant is explicitly requested, auto-swap active variant conflicts.
    # If the destination already has a symlink to a different variant,
    # we treat it as a requested force-overwrite (variant switch).
    is_variant_swap = False
    if variant and not force:
        for module_name in selected_modules or []:
            active = get_active_variant(config, module_name)
            if active and active != variant:
                force = True
                is_variant_swap = True
                print_info(f"Auto-swap: {module_name} variant '{active}' → '{variant}'")
                break

    # Validate variant if specified with specific modules
    if variant and selected_modules:
        for module_name in selected_modules:
            variant_info = get_module_variant_info(config, module_name)
            available_sources = get_module_available_sources(config, module_name)

            if not variant_info or not variant_info.has_variants:
                # User specified a Variant but module has no variants
                print_error(
                    f"Module '{module_name}' has no variants. "
                    f"Available sources: {', '.join(available_sources)}"
                )
                raise typer.Exit(1)

            if variant not in variant_info.variants:
                print_error(
                    f"Variant '{variant}' not found in module '{module_name}'. "
                    f"Available variants: {', '.join(variant_info.variants)}"
                )
                raise typer.Exit(1)

    # Show warning when using cascade (variant auto-selected without explicit --variant)
    if not variant and selected_modules:
        for module_name in selected_modules:
            variant_info = get_module_variant_info(config, module_name)
            if variant_info and variant_info.has_variants:
                print_warning(
                    f"Module '{module_name}' has multiple variants. "
                    f"Using default: '{variant_info.default_variant}' (last in YAML). "
                    f"Use --variant to select a specific one."
                )

    # Pass variant to resolver (cascade handled inside if variant is None)
    type_filter = type if isinstance(type, list) else None
    modules = resolve_modules(
        config, modules=selected_modules, types=type_filter, variant=variant
    )

    if not modules:
        print_warning("No modules found.")
        return

    stats = {"linked": 0, "conflicts": 0, "pending": 0, "errors": 0}
    transaction = TransactionLog()

    try:
        for module_name, statuses in modules.items():
            # Create tree for this module
            module_tree = Tree(f"📦 [bold cyan]{module_name}[/bold cyan]")
            module_stats = {"linked": 0, "conflicts": 0, "pending": 0}

            # Calculate effective variant for this module
            effective_variant = _get_effective_variant(config, module_name, variant)
            variant_tag = (
                f" [dim][{effective_variant}][/dim]" if effective_variant else ""
            )

            for status in statuses:
                src = status.source
                final_dest = status.destination

                # Display based on state
                if status.state == "linked":
                    module_tree.add(f"[green]✔[/green] {src.name} → {final_dest}")
                    module_stats["linked"] += 1

                elif status.state == "unsafe":
                    module_tree.add(
                        f"[red]✗[/red] {src.name} → {final_dest} [red]({status.detail})[/red]"
                    )
                    module_stats["conflicts"] += 1

                elif status.state == "conflict":
                    if force:
                        if dry_run:
                            if is_variant_swap:
                                active = get_active_variant(config, module_name)
                                module_tree.add(
                                    f"[cyan]↔[/cyan] {src.name} → {final_dest} "
                                    f"[cyan](swapped: {active} → {variant})[/cyan]"
                                )
                            else:
                                module_tree.add(
                                    f"[yellow]⚠[/yellow] {src.name} → {final_dest} "
                                    f"[yellow](to be overwritten)[/yellow]"
                                )
                            module_stats["pending"] += 1
                        else:
                            if is_variant_swap:
                                active = get_active_variant(config, module_name)
                                module_tree.add(
                                    f"[cyan]↔[/cyan] {src.name} → {final_dest} "
                                    f"[cyan](swapped: {active} → {variant})[/cyan]"
                                )
                            else:
                                module_tree.add(
                                    f"[green]✔[/green] {src.name} → {final_dest} "
                                    f"[green](overwritten)[/green]"
                                )
                            module_stats["linked"] += 1
                            transaction.unlink(final_dest)
                            transaction.symlink(final_dest, src.resolve())
                    else:
                        module_tree.add(
                            f"[red]⚠[/red] {src.name} → {final_dest} [red](conflict: {status.detail})[/red]"
                        )
                        module_stats["conflicts"] += 1

                elif status.state == "pending":
                    if status.detail == "backup needed":
                        backup_path = Path(str(final_dest) + "-backup")
                        if backup_path.exists():
                            # Backup already exists -> require user attention and do not auto-create another backup
                            module_tree.add(
                                f"[yellow]⚠[/yellow] {src.name} → {final_dest} [yellow](backup exists at {backup_path}, review before creating new backups)[/yellow]"
                            )
                            module_stats["pending"] += 1
                        else:
                            if dry_run:
                                module_tree.add(
                                    f"[yellow]⚠[/yellow] {src.name} → {final_dest} [yellow](backup needed)[/yellow]"
                                )
                                module_stats["pending"] += 1
                            else:
                                module_tree.add(
                                    f"[green]✔[/green] {src.name} → {final_dest} [green](backed up and created)[/green]{variant_tag}"
                                )
                                module_stats["linked"] += 1
                                transaction.backup(final_dest, backup_path)
                                transaction.symlink(final_dest, src.resolve())
                    else:
                        if dry_run:
                            module_tree.add(
                                f"[blue]ℹ[/blue] {src.name} → {final_dest} [dim](to be created)[/dim]"
                            )
                            module_stats["pending"] += 1
                        else:
                            module_tree.add(
                                f"[green]✔[/green] {src.name} → {final_dest} [green](created)[/green]{variant_tag}"
                            )
                            module_stats["linked"] += 1
                            if not final_dest.parent.exists():
                                transaction.mkdir(final_dest.parent)
                            transaction.symlink(final_dest, src.resolve())

            # Add status line
            status_parts = []
            if module_stats["linked"] > 0:
                status_parts.append(f"[green]{module_stats['linked']} linked[/green]")
            if dry_run and module_stats["pending"] > 0:
                status_parts.append(
                    f"[yellow]{module_stats['pending']} to link[/yellow]"
                )
            if module_stats["conflicts"] > 0:
                status_parts.append(f"[red]{module_stats['conflicts']} conflicts[/red]")

            if status_parts:
                module_tree.add(f"[dim]Status: {' • '.join(status_parts)}[/dim]")

            # Update global stats
            for key in stats:
                stats[key] += module_stats.get(key, 0)

            console.print(module_tree)
            console.print()

        # Commit transaction if successful
        if not dry_run:
            transaction.commit()

    except Exception as e:
        # Rollback on any error
        if not dry_run:
            print_error(f"Error during linking: {e}")
            console.print("[yellow]Rolling back changes...[/yellow]")
            transaction.rollback()
            console.print("[green]Rollback complete.[/green]")
        stats["errors"] += 1
        raise typer.Exit(1)

    # Summary with divider
    console.print("━" * console.width)
    summary_parts = []
    if stats["linked"] > 0:
        summary_parts.append(f"[green]{stats['linked']} ✔ linked[/green]")
    if stats["conflicts"] > 0:
        summary_parts.append(f"[red]{stats['conflicts']} ⚠ conflicts[/red]")
    if dry_run and stats["pending"] > 0:
        summary_parts.append(f"[yellow]{stats['pending']} ℹ to link[/yellow]")

    console.print(f"[bold]Summary:[/bold] {' • '.join(summary_parts)}")

    if dry_run:
        console.print("\n[dim]This was a dry run. No changes were made.[/dim]")
