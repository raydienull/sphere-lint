# Sphere SCP Lint (Go Action)

Linter for SphereServer .scp scripts. Runs as a GitHub Action and reports errors with file annotations.

## What It Checks

- Missing [EOF] at the end of a file
- Duplicate ITEMDEF, CHARDEF, EVENTS, FUNCTION, REGIONTYPE, AREADEF, DIALOG, MENU, ROOMDEF, SKILL, SKILLCLASS, SKILLMENU, SPAWN, SPELL, and TYPEDEF
- Unbalanced blocks (IF/ELSE/ENDIF, FOR/ENDFOR, WHILE/ENDWHILE, BEGIN/END, DO*/ENDDO)
- Common typos and bracket errors (including < >)
- FOR, WHILE, and DORAND rules without arguments
- Undeclared references for common prefixes: i_ (ITEMDEF), c_ (CHARDEF), t_ (TYPEDEF), s_ (SPELL), r_ (REGIONTYPE/AREADEF), e_ (EVENTS), m_ (MENU), d_ (DIALOG), f_ (FUNCTION)
- DEFNAME values declared inside ITEMDEF/CHARDEF are treated as declared IDs

## Use

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

## Behavior

- Scans the repository for .scp files
- Ignores .git, .github, backups, backup, and trash directories
- Emits error annotations with file and line numbers
- Fails the job if it finds errors
