# Migrating from vendored Python `repoman`

A repository that still vendors the original Python `repoman` scripts
(`repoman/register.py`, `repoman/ed.py`, and so on) can migrate to
`gorepoman` without touching a single call site. `Makefile` targets,
`.repoman.json` release steps, and README-documented commands all
invoke `python3 repoman/<script>.py ...` today; after migration, every
one of those invocations still works, unchanged -- only what's *behind*
them changes, from the original Python implementation to a thin shim
that forwards to the `gorepoman` binary.

This is worth doing even for a repository that isn't hitting any
specific problem today: the vendored Python version has at least one
confirmed, deliberately-unpatched bug (atomic mode can silently write a
file that failed syntax validation when `gofmt` is absent), and staying
on it means re-paying a toolchain-install detour on every fresh session
that this repository's own `gorepoman` equivalent wouldn't need at all.

## The pattern

Every shim is the same template, varying only in which subcommand it
forwards to:

```python
#!/usr/bin/env python3
"""Thin shim -- forwards to the gorepoman binary's `register` subcommand.

The vendored Python implementation previously in this file has been
retired in favour of github.com/ha1tch/gorepoman (a static Go binary).
This shim exists only so every pre-existing invocation of
`python3 repoman/register.py ...` (Makefile targets, .repoman.json
release steps, README/MANUAL examples) keeps working unchanged.

Binary is located, in order:
  1. $REPOMAN_BIN environment variable
  2. `repoman` on $PATH

If neither resolves, install the binary -- see
https://github.com/ha1tch/gorepoman (mirror:
https://ha1tch.github.io/gorepoman/) -- then either export REPOMAN_BIN
to its path or put it on PATH.
"""
import os
import shutil
import subprocess
import sys

SUBCOMMAND = "register"


def find_binary():
    env = os.environ.get("REPOMAN_BIN")
    if env and os.path.isfile(env) and os.access(env, os.X_OK):
        return env
    return shutil.which("repoman")


def main():
    binary = find_binary()
    if binary is None:
        sys.stderr.write(
            "error: gorepoman binary not found for repoman/register.py shim.\n"
            "Set REPOMAN_BIN to its path, or put `repoman` on PATH.\n"
            "Install: https://github.com/ha1tch/gorepoman "
            "(mirror: https://ha1tch.github.io/gorepoman/)\n"
        )
        return 127
    return subprocess.call([binary, SUBCOMMAND] + sys.argv[1:])


if __name__ == "__main__":
    sys.exit(main())
```

Verified directly against a real binary before publishing here: both
resolution paths (`$REPOMAN_BIN` and `$PATH`), and a real subcommand
invocation with real, multi-line, quoted arguments (`register add
--body "..."`), correctly forward and produce the expected result.

## The mapping

Twelve of the thirteen vendored files map one-to-one to a `gorepoman`
subcommand. The thirteenth, `config.py`, was always a shared helper
module, never a direct CLI entry point -- once every script above it is
a shim, nothing imports `config.py` any more. Leave it in place; it's
dead code, not a broken one, and deleting it is a separate decision, not
part of the migration itself.

| Vendored file | `gorepoman` subcommand |
|---|---|
| `add_wave.py` | `addwave` |
| `doctor.py` | `doctor` |
| `ed.py` | `ed` |
| `gomod.py` | `gomod` |
| `guards.py` | `guards` |
| `register.py` | `register` |
| `relcore.py` | `relcore` |
| `roles.py` | `roles` |
| `selftest.py` | `selftest` |
| `str_replace_extended.py` | `strreplace` |
| `syncver.py` | `syncver` |
| `wave_progress.py` | `waveprogress` |
| `config.py` | *(none -- becomes dead code, left in place)* |

`badcode` has no vendored Python equivalent at all -- it's a
`gorepoman`-only addition, invoked automatically by `relcore` rather
than as a standalone daily-use command, so there's no corresponding
script to shim.

## Generating all twelve at once

Writing twelve near-identical files by hand is exactly the kind of
mechanical work worth generating instead -- both faster and removes the
chance of a slip partway through (a hand-authored shim missing the
`SUBCOMMAND` update, or a stray copy-paste artifact, is a real risk
across twelve near-identical files that a generator doesn't have).
Save this alongside the vendored scripts and run it once:

```python
#!/usr/bin/env python3
"""Generate gorepoman migration shims for a vendored Python repoman
install. Run from the repository root, with the vendored scripts still
in place (this only writes shims; it never deletes anything).

    python3 generate_shims.py repoman/

Safe to re-run; each shim is regenerated from the same template.
"""
import os
import sys
import stat

TEMPLATE = '''#!/usr/bin/env python3
"""Thin shim -- forwards to the gorepoman binary's `{subcommand}` subcommand.

The vendored Python implementation previously in this file has been
retired in favour of github.com/ha1tch/gorepoman (a static Go binary).
This shim exists only so every pre-existing invocation of
`python3 {vendor_dir}/{filename} ...` (Makefile targets, .repoman.json
release steps, README/MANUAL examples) keeps working unchanged.

Binary is located, in order:
  1. $REPOMAN_BIN environment variable
  2. `repoman` on $PATH

If neither resolves, install the binary -- see
https://github.com/ha1tch/gorepoman (mirror:
https://ha1tch.github.io/gorepoman/) -- then either export REPOMAN_BIN
to its path or put it on PATH.
"""
import os
import shutil
import subprocess
import sys

SUBCOMMAND = "{subcommand}"


def find_binary():
    env = os.environ.get("REPOMAN_BIN")
    if env and os.path.isfile(env) and os.access(env, os.X_OK):
        return env
    return shutil.which("repoman")


def main():
    binary = find_binary()
    if binary is None:
        sys.stderr.write(
            "error: gorepoman binary not found for {vendor_dir}/{filename} shim.\\n"
            "Set REPOMAN_BIN to its path, or put `repoman` on PATH.\\n"
            "Install: https://github.com/ha1tch/gorepoman "
            "(mirror: https://ha1tch.github.io/gorepoman/)\\n"
        )
        return 127
    return subprocess.call([binary, SUBCOMMAND] + sys.argv[1:])


if __name__ == "__main__":
    sys.exit(main())
'''

SHIMS = [
    ("add_wave.py", "addwave"),
    ("doctor.py", "doctor"),
    ("ed.py", "ed"),
    ("gomod.py", "gomod"),
    ("guards.py", "guards"),
    ("register.py", "register"),
    ("relcore.py", "relcore"),
    ("roles.py", "roles"),
    ("selftest.py", "selftest"),
    ("str_replace_extended.py", "strreplace"),
    ("syncver.py", "syncver"),
    ("wave_progress.py", "waveprogress"),
]


def main():
    if len(sys.argv) != 2:
        sys.stderr.write("usage: python3 generate_shims.py <vendor_dir>\n")
        return 1
    vendor_dir = sys.argv[1].rstrip("/")
    if not os.path.isdir(vendor_dir):
        sys.stderr.write(f"error: {vendor_dir!r} is not a directory\n")
        return 1

    for filename, subcommand in SHIMS:
        path = os.path.join(vendor_dir, filename)
        content = TEMPLATE.format(
            subcommand=subcommand, vendor_dir=vendor_dir, filename=filename
        )
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        os.chmod(path, os.stat(path).st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
        print(f"wrote {path} -> {subcommand}")

    print(f"\n{len(SHIMS)} shims written. Next: run `repoman selftest` to confirm the "
          f"binary itself is trustworthy, then spot-check a few shims against their real "
          f"original invocations before committing.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

Verified end to end before publishing: generates all twelve files,
every one compiles cleanly (`python3 -m py_compile`), and a generated
shim was run for real (`register list`, `doctor --quiet`) with correct
output.

## After generating

The generator writes files; it doesn't verify the result is trustworthy
on its own. Before committing:

1. **Install `gorepoman` and run `repoman selftest`** — do not trust
   the binary itself until this is green (or green with deferred checks
   naming a missing optional toolchain component, which is fine; see
   `repoman-030-getting-started.md`).
2. **Spot-check a handful of shims against their real original
   invocation** — not every one; the ones actually exercised in this
   repository's own `Makefile` or `.repoman.json` are the ones that
   matter. `register check`, `guards stale`, `syncver show`, `gomod
   check` are typical candidates.
3. **Run the completion sweep** (`repoman-020-failure-modes.md` #4
   describes why this is step one of completion, not a post-failure
   remedy): confirm nothing still references the old vendored
   implementations directly, and that `config.py` (now dead code) isn't
   imported from anywhere unexpected.
4. **Leave `config.py` in place** unless there's a separate reason to
   remove it -- deleting dead code is real, distinct work, not a
   required part of this migration.

See `repoman-030-getting-started.md` for installing the binary itself,
and `repoman-070-releases.md` for how `relcore`, `syncver`, and `gomod`
fit together once the shims are calling into it.
