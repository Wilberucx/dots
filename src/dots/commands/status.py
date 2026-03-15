import typer
from rich.tree import Tree
from dots.ui.output import console, print_header
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules

def status_cmd():
    """
    Show status of all dotfiles modules grouped by state.
    """
    print_header("Dots Status")
    
    config = DotsConfig.load()
    modules = resolve_modules(config)
    
    if not modules:
        console.print("[yellow]⚠[/yellow] No modules found.")
        return
    
    # Categorize modules
    linked = []
    needs_review = []
    not_linked = []
    
    for module_name, statuses in modules.items():
        module_linked = sum(1 for s in statuses if s.state == "linked")
        module_conflicts = sum(1 for s in statuses if s.state in ("conflict", "unsafe"))
        module_pending = sum(1 for s in statuses if s.state == "pending")
        
        # Categorize module
        if module_conflicts > 0 or module_pending > 0:
            reason = []
            if module_conflicts > 0:
                reason.append(f"{module_conflicts} conflict{'s' if module_conflicts > 1 else ''}")
            if module_pending > 0:
                reason.append(f"{module_pending} pending")
            needs_review.append((module_name, ", ".join(reason)))
        elif module_linked > 0:
            linked.append((module_name, f"{module_linked} file{'s' if module_linked > 1 else ''}"))
        else:
            not_linked.append((module_name, "no files to link"))
    
    # Display results
    if linked:
        linked_tree = Tree(f"[green]✔ Linked ({len(linked)} modules)[/green]")
        for name, info in linked:
            linked_tree.add(f"[dim]{name}[/dim] [green]({info})[/green]")
        console.print(linked_tree)
        console.print()
    
    if needs_review:
        review_tree = Tree(f"[yellow]⚠ Needs Review ({len(needs_review)} modules)[/yellow]")
        for name, reason in needs_review:
            review_tree.add(f"[dim]{name}[/dim] [yellow]({reason})[/yellow]")
        console.print(review_tree)
        console.print()
    
    if not_linked:
        not_linked_tree = Tree(f"[blue]ℹ Not Linked ({len(not_linked)} modules)[/blue]")
        for name, reason in not_linked:
            not_linked_tree.add(f"[dim]{name}[/dim] [blue]({reason})[/blue]")
        console.print(not_linked_tree)
        console.print()
    
    # Summary
    console.print("━" * console.width)
    summary_parts = []
    if linked:
        summary_parts.append(f"[green]{len(linked)} linked[/green]")
    if needs_review:
        summary_parts.append(f"[yellow]{len(needs_review)} need review[/yellow]")
    if not_linked:
        summary_parts.append(f"[blue]{len(not_linked)} not linked[/blue]")
    
    console.print(f"[bold]Summary:[/bold] {' • '.join(summary_parts)}")

