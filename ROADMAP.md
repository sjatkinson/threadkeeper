# ThreadKeeper Roadmap

Ideas under consideration. Not scheduled; not committed. Captured here so the rationale survives between sessions.

---

## Per-project workspaces

Allow a `.threadkeeper/` workspace at a project root, used automatically when running `tk` from anywhere inside that project.

### Model

- Two scopes: **project** and **global**. Never mixed.
- **Walk-up discovery, single match**: starting from `cwd`, walk upward looking for a `.threadkeeper/` directory. The first one found wins. Walking stops there — there is no merging, no `--all` view, no traversal past the first hit.
- If no `.threadkeeper/` is found in any ancestor, the **global** workspace is used (resolved the same way as today: `default_workspace` in config, else XDG default).
- `--path` continues to override everything.
- `--global` flag forces the global workspace regardless of `cwd`.

### Why this shape

- Matches muscle memory from `git` (walk up to nearest marker).
- Every `tk` invocation operates on exactly one workspace, so:
  - Short IDs stay simple (no cross-workspace collisions).
  - No ambiguity about which workspace a write targets.
  - No need for workspace keys or qualified IDs like `web:3`.
- If you want a merged view across workspaces, write a shell script — the CLI shouldn't grow that complexity.

### What was considered and rejected

- **Merged `--all` listings across workspaces.** Rejected: forces short-ID disambiguation, introduces scope-labeling in output, complicates writes. Easy to add later if needed; hard to remove.
- **Centralized storage with path-keyed registry.** Rejected: breaks when the project dir is renamed/moved, symlink and case-sensitivity issues on macOS, cloning on another machine loses bindings. Pollution of `.threadkeeper/` at project root is the same kind nobody complains about with `.git/`, `.envrc`, `.nvmrc`.
- **Hybrid: central storage + tiny marker file.** Rejected: two-place coupling for marginal benefit; deleting either side orphans the other.

### Open questions when revisiting

- `tk init` default: does it create local (`.threadkeeper/` in `cwd`) or global? Probably local with `--global` to target global explicitly.
- Should `.threadkeeper/` be git-ignored by default, or is that the user's call?
- What happens if a user runs `tk` in `$HOME` and `$HOME` has a `.threadkeeper/`? Works fine, but worth noting.

---

## Console GUI (TUI)

A terminal UI for browsing threads, reading bodies and notes, and navigating between tasks. The CLI is the primary interface and will remain so; the TUI is for the times when you want to move through several threads rather than act on a specific one.

Design and interaction are not yet specified. Open questions for later:

- Scope: read/navigate only, or also edit (status changes, adding notes)?
- Launch model: a separate subcommand (`tk ui`), or a flag on existing commands?
- Library: Bubble Tea, tview, or something else?
- Relationship to per-project workspaces: single-workspace view, or a workspace picker?

Not blocked by anything; can be built whenever the CLI settles.
