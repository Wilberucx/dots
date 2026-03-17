import typer
from pathlib import Path
from rich.tree import Tree
from dots.ui.output import console, print_header, print_warning, print_error, print_info, print_success
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules
from dots.core.transaction import TransactionLog
from dots.ui.selector import select_modules

def unlink_cmd(
    module: list[str] | None = typer.Option(
        None, "--module", "-m",
        help="Unlink only specific modules (repeatable: -m Zsh -m Nvim)"
    ),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would happen"),
    interactive: bool = typer.Option(False, "--interactive", "-i", help="Interactively select modules to unlink"),
):
    """
    Remove symlinks for dotfiles modules.
    
    By default, unlinks all modules. Use --interactive to select specific modules or pass them as arguments.
    """
    print_header("Unlinking Dotfiles")
    
    config = DotsConfig.load()
    
    if module:
        selected_modules = module
        print_info(f"Unlinking specified modules: {', '.join(selected_modules)}")
    elif interactive:
        selected_modules = select_modules(config, preselect_all=False)
        if not selected_modules:
            print_info("No modules selected.")
            return
        print_info(f"Selected {len(selected_modules)} module(s)")
        console.print()
    else:
        selected_modules = None  # Unlink all modules
    
    modules = resolve_modules(config, modules=selected_modules)
    
    if not modules:
        print_warning("No modules found.")
        return
    
    stats = {"unlinked": 0, "not_linked": 0, "errors": 0}
    transaction = TransactionLog()
    
    try:
        for module_name, statuses in modules.items():
            module_tree = Tree(f"📦 [bold cyan]{module_name}[/bold cyan]")
            module_stats = {"unlinked": 0, "not_linked": 0}
            
            for status in statuses:
                src = status.source
                final_dest = status.destination
                
                if status.state == "linked":
                    if dry_run:
                        module_tree.add(f"[red]✗[/red] {src.name} → {final_dest} [red](to be removed)[/red]")
                        module_stats["unlinked"] += 1
                    else:
                        module_tree.add(f"[green]✔[/green] {src.name} → {final_dest} [green](removed)[/green]")
                        module_stats["unlinked"] += 1
                        transaction.unlink(final_dest)
                elif status.state == "conflict":
                    module_tree.add(f"[yellow]⚠[/yellow] {src.name} → {final_dest} [yellow](conflict, skipping)[/yellow]")
                    module_stats["not_linked"] += 1
                elif status.state in ["pending", "missing"]:
                    module_tree.add(f"[blue]ℹ[/blue] {src.name} → {final_dest} [dim](not linked)[/dim]")
                    module_stats["not_linked"] += 1
                elif status.state == "unsafe":
                    module_tree.add(f"[yellow]⚠[/yellow] {src.name} → {final_dest} [yellow](unsafe, skipping)[/yellow]")
                    module_stats["not_linked"] += 1
            
            status_parts = []
            if module_stats["unlinked"] > 0:
                label = "to remove" if dry_run else "removed"
                status_parts.append(f"[red]{module_stats['unlinked']} {label}[/red]")
            if module_stats["not_linked"] > 0:
                status_parts.append(f"[dim]{module_stats['not_linked']} not linked[/dim]")
            
            if status_parts:
                module_tree.add(f"[dim]Status: {' • '.join(status_parts)}[/dim]")
            
            for key in stats:
                stats[key] += module_stats.get(key, 0)
            
            console.print(module_tree)
            console.print()
        
        if not dry_run:
            transaction.commit()
    
    except Exception as e:
        if not dry_run:
            print_error(f"Error during unlinking: {e}")
            console.print("[yellow]Rolling back changes...[/yellow]")
            transaction.rollback()
            console.print("[green]Rollback complete.[/green]")
        stats["errors"] += 1
        raise typer.Exit(1)
    
    console.print("━" * console.width)
    summary_parts = []
    if stats["unlinked"] > 0:
        label = "to remove" if dry_run else "removed"
        summary_parts.append(f"[green]{stats['unlinked']} ✔ {label}[/green]")
    if stats["not_linked"] > 0:
        summary_parts.append(f"[dim]{stats['not_linked']} not linked[/dim]")
    
    console.print(f"[bold]Summary:[/bold] {' • '.join(summary_parts)}")
    
    if dry_run:
        console.print("\n[dim]This was a dry run. No changes were made.[/dim]")
