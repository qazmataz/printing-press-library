# macctx Printed CLI Agent Guide

This directory is a generated `macctx-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
macctx-pp-cli doctor --json
macctx-pp-cli dump --json
```

Use runtime discovery instead of relying on a copied command list:

```bash
macctx-pp-cli <command> --help
```

Observation commands are safe defaults, local-first, and call Peekaboo with `--no-remote`. Clipboard output is preview-only unless `--full` is explicitly passed.

Before running any UI action, inspect the dry run first:

```bash
macctx-pp-cli act click --on B3 --json
macctx-pp-cli act click --on B3 --execute
```

Use `--execute` only after the target, arguments, and side effects are clear.

For install, examples, and longer product guidance, read `README.md`, `SKILL.md`, and `docs/COMPUTER_USE.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Local Customizations

If you modify this CLI beyond what the generator produced, record each customization so it isn't lost on the next regen and is visible to the next reader.

1. **Mark every changed site** in source with a comment summarizing the deviation:

    ```
    // PATCH: <one-line summary>
    ```

2. **Catalog the change** in `.printing-press-patches.json` at this CLI's root.

This file is an index of customizations, not a second copy of the diff. Diffs live in `git`; code lives in the source files; the inline `// PATCH:` comment carries the local semantics.
