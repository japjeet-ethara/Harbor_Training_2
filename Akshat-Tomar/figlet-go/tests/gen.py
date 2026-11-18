#!/usr/bin/env python3
"""
tests/gen.py — generate visible.jsonl and hidden.jsonl
Run from the workspace root (next to both figlet-go/ and pyfiglet/).
"""
import json
import os
import random
import sys
from io import StringIO
from optparse import OptionParser

# Point Python at the cloned pyfiglet repo (sibling of figlet-go/)
# __file__ is  .../akshat/figlet-go/tests/gen.py
# pyfiglet is  .../akshat/pyfiglet/
PYFIGLET_DIR = os.path.abspath(
    os.path.join(os.path.dirname(__file__), '..', '..', 'pyfiglet')
)
sys.path.insert(0, PYFIGLET_DIR)

import pyfiglet as pf

FONTS     = ['standard', 'slant', 'small', 'big', 'mini', 'banner', 'shadow', 'script']
WIDTHS    = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ['auto', 'left', 'center', 'right']
DIRS      = ['auto', 'left-to-right', 'right-to-left']

NORMAL_WORDS = [
    'hello', 'world', 'foo', 'bar', 'test', 'go', 'figlet', 'harbor',
    'python', 'port', 'ASCII', 'art', 'agent', 'task', 'build',
]
TRICKY = [
    # smushing edge cases
    '||', '><', '/\\', '\\/+', '_|_', '[]', '{}', '()', '[[]]',
    # special chars
    '|', '/', '\\', '_', '[', ']', '{', '}', '(', ')', '<', '>',
    # Latin-1
    'é', 'ß', 'ñ', 'ö', 'ü', 'ä',
    # double spaces
    'hello  world',
    # long wrap sentences
    'the quick brown fox jumps over the lazy dog',
    'this is a very long sentence that should wrap at narrow widths',
    # chars likely not in font
    '★', '€', '→',
    # empty-ish
    ' ',
    # mixed
    'hello | world',
    'foo/bar\\baz',
    '< angle >',
]

# Cache loaded FigletFont objects to avoid parsing .flf thousands of times
_FONT_CACHE = {}

def get_font(name):
    if name not in _FONT_CACHE:
        _FONT_CACHE[name] = pf.FigletFont(name)
    return _FONT_CACHE[name]

def run_pyfiglet(args):
    """Run pyfiglet.main() with the given args list and return captured stdout."""
    parser = OptionParser()
    parser.add_option('-f', '--font',      default='standard')
    parser.add_option('-D', '--direction', default='auto',
                      choices=['auto','left-to-right','right-to-left'])
    parser.add_option('-j', '--justify',   default='auto',
                      choices=['auto','left','center','right'])
    parser.add_option('-w', '--width',     type='int', default=80)
    parser.add_option('-r', '--reverse',   action='store_true', default=False)
    parser.add_option('-n', '--normalize-surrounding-newlines',
                      action='store_true', default=False)
    parser.add_option('-s', '--strip-surrounding-newlines',
                      action='store_true', default=False)
    parser.add_option('-F', '--flip',      action='store_true', default=False)
    opts, rest_args = parser.parse_args(args)

    if not rest_args:
        return None

    text = ' '.join(rest_args)
    try:
        font_obj = get_font(opts.font)
        # Guard against pyfiglet bug: if any character is >= width,
        # pyfiglet's line-wrapper enters an infinite loop.
        if any(font_obj.width.get(ord(c), 0) >= opts.width for c in text):
            return None

        fig = pf.Figlet(
            font=opts.font,
            direction=opts.direction,
            justify=opts.justify,
            width=opts.width,
        )
        r = fig.renderText(text)
    except Exception:
        return None

    if opts.reverse:
        r = r.reverse()
    if opts.flip:
        r = r.flip()
    if opts.strip_surrounding_newlines:
        r = r.strip_surrounding_newlines()
    elif opts.normalize_surrounding_newlines:
        r = r.normalize_surrounding_newlines()

    # pyfiglet writes r.encode('UTF-8') + b'\n' to stdout
    return r + '\n'


def build_args(rng):
    args = []
    font = rng.choice(FONTS)
    args += ['-f', font]
    width = rng.choice(WIDTHS)
    args += ['-w', str(width)]
    args += ['-j', rng.choice(JUSTIFIES)]
    args += ['-D', rng.choice(DIRS)]
    if rng.random() < 0.25:
        args.append('-r')
    if rng.random() < 0.25:
        args.append('-F')
    if rng.random() < 0.25:
        args.append('-s')
    elif rng.random() < 0.25:
        args.append('-n')

    # Pick text: 50% normal word, 50% tricky / combo
    if rng.random() < 0.5:
        text = rng.choice(NORMAL_WORDS)
    else:
        text = rng.choice(TRICKY)
        # Sometimes append a normal word
        if rng.random() < 0.3:
            text = text + ' ' + rng.choice(NORMAL_WORDS)

    args.append(text)
    return args


def generate(n, rng):
    cases = []
    attempts = 0
    while len(cases) < n:
        attempts += 1
        if attempts > n * 20:
            break
        args = build_args(rng)
        out = run_pyfiglet(args)
        if out is None:
            continue
        cases.append({'args': args, 'out': out})
        if len(cases) % 500 == 0:
            print(f'  progress: {len(cases)}/{n} cases generated...', flush=True)
    return cases


def write_jsonl(path, cases):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=False) + '\n')


if __name__ == '__main__':
    rng = random.Random(7)

    print('Generating 2000 visible cases...')
    visible = generate(2000, rng)

    print('Generating 2000 hidden cases...')
    hidden = generate(2000, rng)

    out_dir = os.path.join(os.path.dirname(__file__), 'data')
    write_jsonl(os.path.join(out_dir, 'visible.jsonl'), visible)
    write_jsonl(os.path.join(out_dir, 'hidden.jsonl'),  hidden)

    print(f'Done. Visible: {len(visible)}, Hidden: {len(hidden)}')
