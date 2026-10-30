#!/usr/bin/env python3
"""Rename magpie -> pigeon across the tree, protecting what must not change.

Protected, because renaming breaks something outside this tree:
  * usemagpie.ai, magpie-community, magpie-releases, ghcr.io/yetone/magpie
    -- the author's servers and repos; a rename would 404.
  * MAGPIE_* environment variables -- user scripts and CI set them.
  * magpie:// -- the URL scheme registered with the OS.
  * the plugin JS namespace (magpie.call, magpie.actions, ...) and the
    localStorage keys (magpie.lastView, ...) -- plugin authors' code.
  * magpie.* in a config path, and providers.magpie -- the provider id
    written into other agents' config files.
  * "magpie" as a whole quoted literal -- the provider id itself.

Renamed:
  * every other mention -- prose, window titles, file names, the module path.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.abspath(__file__))

SKIP_DIRS = {'.git', 'node_modules', 'dist'}
SKIP_EXT = {'.exe', '.png', '.jpg', '.jpeg', '.gif', '.ico', '.icns', '.woff',
            '.woff2', '.ttf', '.otf', '.zip', '.gz', '.bin', '.pdf', '.mp4',
            '.webp', '.syso', '.car'}
SKIP_NAMES = {'rename-to-pigeon.py'}

# Identity written to disk or to other apps: renamed by hand, if at all.
KEEP_FILES = {
    'internal/appdir/appdir.go',
    'internal/appdir/env.go',
    'internal/agent/route.go',
}

PROTECT_LITERALS = [
    'usemagpie',
    'magpie-community',
    'magpie-releases',
    'magpie.magpie-backup',
    'magpie-windows-amd64',
    'magpie-x-search',
    'magpie-fake-agent',
    'magpie.example.com',
    'magpie.test',
    'magpie.localhost',
    'magpie://',
]

PROTECT_REGEX = [
    r'MAGPIE_[A-Z0-9_]+',                 # environment variables
    r'ghcr\.io/yetone/magpie[\w:./-]*',   # published docker image
    r'(?<=@)magpie(?=-community)',        # npm scope, only where it is one
    # the provider id: an identity other agents and the usage log hold.
    # Only where it is one -- the window's title and tray label are prose
    # and take the new name.
    r'(?<="ID": )"magpie"',
    r'(?<=ID: )"magpie"',
    r'(?<== )"magpie"(?=[,)]|\s*\{)',
    # plugin JS namespace and localStorage keys
    r'magpie\.(?:actions|agentModelsSeen|call|context|ctxDays|ctxLook|ctxShut|'
    r'deprecatedHidden|discoveryIgnored|fillSlots|icon|lanAddress|lastView|maskEmails|'
    r'modelFavorites|navLayout|offProvidersOpen|onAction|onTheme|options|page|pages|'
    r'panelOptsFolded|panelQuotas|panelTab|panelUseMetric|panelUsePeriod|panels|'
    r'permissions|pluginTab|quotaLeft|quotaRange|ready|read|replacing|settingsTab|'
    r'sideWidth|slot|sort|sseView|start|strings|theme|usageAccts\w*|usageAccounts|'
    r'usageKeysAll|usageOpen|usageTab|libTab|sessRange|warmTab|maskEmails)\b',
    # the config folder: settings, cache, the portable data dir all live
    # under a "magpie" path that keeps its name. A struct field (rung.magpie)
    # is not one.
    r'(?<=[~/\\])magpie(?=[/\\])',
    r'(?<=\{)magpie(?=[/\\])',
    r'(?<=")magpie(?=[/\\])',
    # providers.magpie in a JSON path
    r'(?<=[\w.])\.magpie\b[\w.]*',
]

REPLACEMENTS = [
    ('github.com/yetone/magpie', 'github.com/hoki2020/pigeon'),
    ('Magpie', 'Pigeon'),
    ('magpie', 'pigeon'),
]

SENT = '\x00%d\x00'


def eligible(rel, path):
    if rel in KEEP_FILES or os.path.basename(path) in SKIP_NAMES:
        return False
    return os.path.splitext(path)[1].lower() not in SKIP_EXT


def walk():
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for name in filenames:
            p = os.path.join(dirpath, name)
            rel = os.path.relpath(p, ROOT).replace(os.sep, '/')
            if eligible(rel, p):
                yield p, rel


def transform(text):
    held = []

    def hold(m):
        held.append(m.group(0))
        return SENT % (len(held) - 1)

    # the module path first: it is unambiguous, and the path rules below
    # would otherwise see its slashes as a config folder
    text = text.replace('github.com/yetone/magpie', 'github.com/hoki2020/pigeon')

    lits = sorted(PROTECT_LITERALS, key=len, reverse=True)
    if lits:
        text = re.compile('|'.join(re.escape(x) for x in lits)).sub(hold, text)
    for pr in PROTECT_REGEX:
        text = re.sub(pr, hold, text)
    for old, new in REPLACEMENTS:
        text = text.replace(old, new)
    return re.sub('\x00(\\d+)\x00', lambda m: held[int(m.group(1))], text)


def main():
    dry = '--apply' not in sys.argv
    changed, report = 0, []
    for p, rel in walk():
        try:
            with open(p, 'r', encoding='utf-8', newline='') as f:
                src = f.read()
        except (UnicodeDecodeError, OSError):
            continue
        if 'magpie' not in src.lower():
            continue
        dst = transform(src)
        if dst == src:
            continue
        changed += 1
        report.append(rel)
        if not dry:
            with open(p, 'w', encoding='utf-8', newline='') as f:
                f.write(dst)
    print(f"{'DRY RUN' if dry else 'APPLIED'}: {changed} files")
    if dry:
        for rel in report[:30]:
            print('  ', rel)
        if len(report) > 30:
            print(f'   ... and {len(report) - 30} more')


if __name__ == '__main__':
    main()
