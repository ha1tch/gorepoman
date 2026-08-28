#!/usr/bin/env python3
"""
gorepoman site generator -- builds the static GitHub Pages mirror.

This exists for one reason: an agent stuck in a sandbox that can't
reach github.com/releases for whatever reason (network policy,
transient issue, tool permissions) may still be able to reach a plain
github.io static file. Same content, a genuinely different request
path -- no release-asset redirect flow through api.github.com or
objects.githubusercontent.com, just a static GET. This script does
not replace GitHub Releases; it mirrors it, so there are two
independent ways to get the same verified thing.

CI (.github/workflows/pages.yml) runs `make cross` first, producing
dist/repoman-<os>-<arch>[.exe] and dist/checksums.txt -- this script
copies those into the site tree alongside a rendered copy of every
docs/*.md file, then writes an index linking both. Style matches
zsp/tools/build_site.py (same markdown extensions, same
VERSION-file-driven footer) since it's the same kind of tool for the
same kind of project, not a reason to invent a different convention.
"""
import os
import shutil
import markdown as mdlib

TOOLS_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(TOOLS_DIR)
SITE = os.path.join(REPO_ROOT, "gorepoman_site")
DIST = os.path.join(REPO_ROOT, "dist")
DOCS = os.path.join(REPO_ROOT, "docs")

MD_EXT = ["fenced_code", "tables", "sane_lists", "toc", "attr_list"]

PLATFORMS = [
    ("linux", "amd64", ""), ("linux", "arm64", ""),
    ("darwin", "amd64", ""), ("darwin", "arm64", ""),
    ("windows", "amd64", ".exe"), ("windows", "arm64", ".exe"),
    ("freebsd", "amd64", ""), ("freebsd", "arm64", ""),
    ("openbsd", "amd64", ""), ("openbsd", "arm64", ""),
    ("netbsd", "amd64", ""), ("netbsd", "arm64", ""),
    ("dragonfly", "amd64", ""),
]


def read_version():
    p = os.path.join(REPO_ROOT, "VERSION")
    if not os.path.isfile(p):
        raise SystemExit(f"build_site: VERSION file not found at {p}")
    v = open(p, encoding="utf-8").read().strip()
    if not v:
        raise SystemExit("build_site: VERSION file is empty")
    return v


PAGE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{title} - gorepoman</title>
<style>
  body {{ font-family: -apple-system, sans-serif; max-width: 760px; margin: 2rem auto;
         padding: 0 1rem; line-height: 1.55; color: #1a1a1a; }}
  a {{ color: #0550ae; }}
  nav {{ margin-bottom: 2rem; font-size: 0.9rem; }}
  nav a {{ margin-right: 1rem; }}
  table {{ border-collapse: collapse; width: 100%; margin: 1rem 0; }}
  th, td {{ text-align: left; padding: 0.4rem 0.8rem; border-bottom: 1px solid #ddd; }}
  code {{ background: #f2f2f2; padding: 0.1rem 0.3rem; border-radius: 3px; }}
  pre code {{ display: block; padding: 0.8rem; overflow-x: auto; }}
  footer {{ margin-top: 3rem; font-size: 0.85rem; color: #666; }}
</style>
</head>
<body>
<nav><a href="index.html">gorepoman</a> {navextra}</nav>
{body}
<footer>gorepoman {version} &middot;
<a href="https://github.com/ha1tch/gorepoman">github.com/ha1tch/gorepoman</a> &middot;
this page mirrors the GitHub release; if one is unreachable, try the other</footer>
</body>
</html>
"""


def render_page(title, body_html, navextra="", version=""):
    return PAGE.format(title=title, body=body_html, navextra=navextra, version=version)


def build_index(version):
    rows = []
    for os_, arch, ext in PLATFORMS:
        name = f"repoman-{os_}-{arch}{ext}"
        rows.append(
            f"<tr><td>{os_}</td><td>{arch}</td>"
            f'<td><a href="bin/{name}">{name}</a></td></tr>'
        )
    doc_links = "\n".join(
        f'<li><a href="docs/{os.path.splitext(f)[0]}.html">{f}</a></li>'
        for f in sorted(os.listdir(DOCS)) if f.endswith(".md")
    )
    body = f"""
<h1>gorepoman {version}</h1>
<p>Repository-discipline tooling: journaled editing, syntactic-role-aware
substitution, a mandatory forbidden-string release gate, and more --
as a single static binary, no toolchain required to bootstrap it.</p>

<h2>Binaries</h2>
<table>
<tr><th>OS</th><th>Arch</th><th>Download</th></tr>
{"".join(rows)}
</table>
<p><a href="bin/checksums.txt">checksums.txt</a> -- verify with <code>sha256sum</code>
(or <code>sha256</code> on the BSDs) before trusting a download from either mirror.</p>

<h2>Documentation</h2>
<ul>
{doc_links}
</ul>
"""
    return render_page("gorepoman", body, version=version)


def build_doc_pages(version):
    for f in sorted(os.listdir(DOCS)):
        if not f.endswith(".md"):
            continue
        src = os.path.join(DOCS, f)
        text = open(src, encoding="utf-8").read()
        html = mdlib.markdown(text, extensions=MD_EXT)
        out_name = os.path.splitext(f)[0] + ".html"
        out_path = os.path.join(SITE, "docs", out_name)
        os.makedirs(os.path.dirname(out_path), exist_ok=True)
        page = render_page(f, html, navextra='&middot; <a href="../index.html">index</a>', version=version)
        with open(out_path, "w", encoding="utf-8") as fh:
            fh.write(page)


def copy_binaries():
    if not os.path.isdir(DIST):
        raise SystemExit(f"build_site: {DIST} not found -- run `make cross` first")
    bin_dir = os.path.join(SITE, "bin")
    os.makedirs(bin_dir, exist_ok=True)
    copied = 0
    for name in os.listdir(DIST):
        shutil.copy2(os.path.join(DIST, name), os.path.join(bin_dir, name))
        copied += 1
    if copied == 0:
        raise SystemExit(f"build_site: {DIST} exists but is empty -- `make cross` did not produce binaries")
    return copied


def main():
    version = read_version()
    if os.path.isdir(SITE):
        shutil.rmtree(SITE)
    os.makedirs(SITE, exist_ok=True)

    n = copy_binaries()
    build_doc_pages(version)
    with open(os.path.join(SITE, "index.html"), "w", encoding="utf-8") as fh:
        fh.write(build_index(version))

    print(f"gorepoman site built: {n} binaries, "
          f"{len([f for f in os.listdir(DOCS) if f.endswith('.md')])} docs -> {SITE}")


if __name__ == "__main__":
    main()
