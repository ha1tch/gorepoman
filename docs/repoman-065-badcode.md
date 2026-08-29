# The forbidden-string gate: `badcode`

`badcode` is a release-blocking scan for text that must never reach a
release under any circumstances -- names, internal codenames,
credentials-shaped tokens, "DO NOT SHIP" markers. `relcore` (see
`repoman-070-releases.md`) runs it automatically and unconditionally as
the very first thing it does, on every release, including on `--resume`
-- not a step that can be skipped or configured away.

## The design constraint that shapes everything here

The pattern list is deliberately **never stored in any repository**. It
lives in a local, per-machine config directory instead, because a
blocklist that ships inside the repo it's protecting can be edited by
whoever has commit access -- including an overly-permissive or
compromised agent -- in the same change that would have been caught by
it. Keeping it local and out-of-band is the entire point: the check
exists outside the reach of "just edit the config to stop being
blocked."

This has a direct consequence worth stating plainly: **a fresh session
in a fresh sandbox has no badcode config until one is recreated.**
Nothing is wrong if `badcode check` reports zero patterns configured --
that's the expected starting state, not a broken installation. See
"No config configured" below for what that actually looks like and why
it's a soft pass, not a silent one.

## Config location and format

The OS-appropriate user config directory (via `os.UserConfigDir()` --
`~/.config` on Linux, `~/Library/Application Support` on macOS,
`%AppData%` on Windows) under a `repoman` subdirectory, overridable via
the `REPOMAN_BADCODE_DIR` environment variable. Two file formats are
read, both optional, both additive if both exist:

```
badcode.txt    one pattern per line; blank lines and lines starting
                with '#' are ignored.
badcode.json   [{"pattern": "...", "reason": "..."}, ...] -- "reason"
                is optional, and included in the refusal message when
                a pattern matches, so a match is actionable rather
                than just a bare string.
```

## Matching: literal substring, not regex -- deliberately

Case-insensitive, literal substring search. Not regex. Regex adds
failure modes (a pattern that silently doesn't compile, ReDoS, an
off-by-one in an anchor) to a check whose entire value is that it never
has a false "clean" result -- a plain substring search can't fail in a
way that looks like success.

This does mean a pattern split across an ordinary line wrap is checked
for too, not just a pattern sitting entirely on one line -- an
accidental line wrap is enough to split a forbidden name across two
lines by complete accident, not just deliberate evasion, and a match
found only that way is reported with a `[spans lines N-M]` marker so
the message stays precise about what actually happened.

Binary files are skipped (the same NUL-byte heuristic git itself uses),
so a forbidden byte sequence that happens to occur coincidentally
inside a compiled artifact is never flagged.

## Usage

```
$ repoman badcode check .
BADCODE CHECK OK (4 pattern(s) checked)
```

A real match refuses clearly, with file, line, and (if configured) the
reason:

```
$ repoman badcode check .
BADCODE CHECK FAIL: 1 match(es)
ERROR badcode-match: pattern "INTERNAL_CODENAME" (project codename --
must never appear in anything meant for external publication) found
in README.md:12: still using INTERNAL_CODENAME in this draft
```

## No config configured

```
$ repoman badcode check .
WARN no badcode patterns configured in /home/user/.config/repoman -- nothing checked
BADCODE CHECK OK (0 patterns configured)
```

Exit 0 -- optional, local, per-operator tooling shouldn't block every
release for an operator who hasn't set it up yet. But the `WARN` line
is always there, on every invocation with no config, so a check that
never actually ran can't be mistaken for a check that ran clean. This
is the same guard principle `repoman-020-failure-modes.md` describes
for dormant guards generally: a check that never actually ran proves
nothing, and this one says so plainly rather than looking identical to
a real pass.

## Recreating the config in a fresh session

Because the config is deliberately never committed, a fresh session
starts with none. If the project has one configured (check prior
session notes, or ask), recreate it at the start of any session doing
release-related work, before that work begins -- not discovered as
missing partway through a release:

```bash
mkdir -p ~/.config/repoman
cat > ~/.config/repoman/badcode.json << 'EOF'
[
  {"pattern": "...", "reason": "..."}
]
EOF
```

## Integration with `relcore`

`relcore <version>` runs `badcode check` against the whole tree as the
first thing it does, before reading `release.steps`, before any step
runs, including on `--resume` -- it is not part of the resumable-steps
journal at all, so there is nothing for `--resume` to bypass. A real
match blocks the entire release; fixing the match (removing the
matched content) and re-running proceeds normally. See
`repoman-070-releases.md` for the full release workflow this sits
inside.
