# Sphere SCP Lint (Go Action)

Lint SphereServer .scp scripts locally or in CI. Reports errors with file/line annotations in GitHub Actions.

## What It Checks

- Missing [EOF] at the end of a file
- Duplicate ITEMDEF, CHARDEF, EVENTS, FUNCTION, REGIONTYPE, AREADEF, DIALOG, MENU, ROOMDEF, SKILL, SKILLCLASS, SKILLMENU, SPAWN, SPELL, and TYPEDEF
- Unbalanced blocks (IF/ELSE/ENDIF, FOR/ENDFOR, WHILE/ENDWHILE, BEGIN/END, DO*/ENDDO)
- Common typos and bracket errors (including < >, while allowing <...> tokens in expressions)
- FOR, WHILE, and DORAND rules without arguments
- Undeclared references for common prefixes: i_ (ITEMDEF), c_ (CHARDEF), spawn_ (SPAWN), t_ (TYPEDEF), s_ (SPELL), r_ (REGIONTYPE/AREADEF), e_ (EVENTS), m_ (MENU), d_ (DIALOG), f_ (FUNCTION)
- Any ID defined as ITEMDEF, CHARDEF, SPAWN, etc is considered declared for undeclared checks
- DEFNAME values declared inside sections (including MULTIDEF), plus RESDEFNAME/RES_RESDEFNAME alias keys, are treated as declared IDs
- RESDEFNAME/RES_RESDEFNAME sections are treated as compatibility alias tables, so their mapped values are not validated as existing resources

## Quick Start (GitHub Actions)

```yaml
name: Lint Sphere scripts
on:
  push:
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: raydienull/sphere-lint@v1
```

  ## Run Locally

  From the repository root:

  ```bash
  go run .
  ```

  To build a binary:

  ```bash
  go build -o sphere-lint ./
  ./sphere-lint
  ```


## Behavior

- Scans the repository for .scp files
- Ignores .git, .github, backups, backup, and trash directories
- Emits error annotations with file and line numbers
- Exits with code 1 if it finds errors
