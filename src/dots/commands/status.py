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
    broken = []
    missing_src = []
    unlinked = []
    not_linked = []
    
    for module_name, statuses in modules.items():
        module_linked = sum(1 for s in statuses if s.state == "linked")
        module_broken = sum(1 for s in statuses if s.state in ("conflict", "unsafe"))
        module_missing = sum(1 for s in statuses if s.state == "missing")
        module_pending = sum(1 for s in statuses if s.state == "pending")
        
        # Categorize module by worst state
        if module_broken > 0:
            reason = []
            conflicts = sum(1 for s in statuses if s.state == "conflict")
            unsafe = sum(1 for s in statuses if s.state == "unsafe")
            if conflicts > 0:
                reason.append(f"{conflicts} conflict{'s' if conflicts > 1 else ''}")
            if unsafe > 0:
                reason.append(f"{unsafe} unsafe path{'s' if unsafe > 1 else ''}")
            broken.append((module_name, ", ".join(reason)))
        elif module_missing > 0:
            missing_src.append((module_name, f"{module_missing} missing source{'s' if module_missing > 1 else ''}"))
        elif module_pending > 0:
            unlinked.append((module_name, f"{module_pending} unlinked"))
        elif module_linked > 0:
            linked.append((module_name, f"{module_linked} linked"))
        else:
            not_linked.append((module_name, "no files to link"))
    
    # Display results
    if linked:
        linked_tree = Tree(f"[success]✔ Linked ({len(linked)} modules)[/success]")
        for name, info in linked:
            linked_tree.add(f"[dim]{name}[/dim] [success]({info})[/success]")
        console.print(linked_tree)
        console.print()
    
    if unlinked:
        unlinked_tree = Tree(f"[dim]ℹ Unlinked ({len(unlinked)} modules)[/dim]")
        for name, reason in unlinked:
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


