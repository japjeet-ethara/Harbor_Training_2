#!/usr/bin/env python3
"""Generate visible.jsonl and hidden.jsonl test cases for the figlet-go grader."""

import io
import json
import os
import random
import sys

# Locate pyfiglet repository
script_dir = os.path.dirname(os.path.abspath(__file__))
task_dir = os.path.dirname(script_dir)
sibling_pyfiglet = os.path.join(os.path.dirname(task_dir), 'pyfiglet')
if os.path.isdir(sibling_pyfiglet):
    sys.path.insert(0, sibling_pyfiglet)

import pyfiglet

FONTS = ['standard', 'slant', 'small', 'big', 'mini', 'banner', 'shadow', 'script']
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ['auto', 'left', 'center', 'right']
DIRECTIONS = ['auto', 'left-to-right', 'right-to-left']

TRICKY_TEXTS = [
    '|', '/', '\\', '_', '[', ']', '{', '}', '(', ')', '<', '>',
    '| / \\', '_|_', '[]', '{}', '()', '<>', '/\\', '\\/',
    'hello  world', 'double  space',
    'the quick brown fox jumps over the lazy dog',
    'A long sentence wrapping nicely at reasonable terminal widths',
    'é', 'ß', 'ñ', 'ÄÖÜäöü', 'café', '☃',
    '|_|', '[_]', '(/\\)', '{|}',
    '!@#$%^&*', '0123456789',
]

NORMAL_WORDS = [
    'hello', 'world', 'test', 'foo', 'bar', 'baz',
    'pyfiglet', 'golang', 'harbor', 'task',
    'quick', 'brown', 'fox', 'jump',
    'A', 'Z', '0', '42', 'OK', 'Go',
]

loaded_fonts = {f: pyfiglet.FigletFont(f) for f in FONTS}

class MockStdout:
    def __init__(self):
        self.buf = io.BytesIO()

    def detach(self):
        return self.buf

    def write(self, s):
        pass

    def flush(self):
        pass

def run_pyfiglet(args):
    old_stdout = sys.stdout
    old_argv = sys.argv
    try:
        mock = MockStdout()
        sys.stdout = mock
        sys.argv = ['pyfiglet'] + args
        pyfiglet.main()
        return mock.buf.getvalue().decode('utf-8', errors='replace')
    finally:
        sys.stdout = old_stdout
        sys.argv = old_argv

def gen_case(seen_args):
    while True:
        font = random.choice(FONTS)
        width = random.choice(WIDTHS)
        justify = random.choice(JUSTIFIES)
        direction = random.choice(DIRECTIONS)
        r = random.random() < 0.25
        F = random.random() < 0.25
        s = random.random() < 0.25
        n = random.random() < (1.0 / 3.0) if not s else False

        if random.random() < 0.5:
            text = random.choice(TRICKY_TEXTS)
        else:
            text = random.choice(NORMAL_WORDS)

        f_obj = loaded_fonts[font]
        max_w = max((f_obj.width.get(ord(c), 0) for c in text), default=0)
        if max_w >= width:
            continue

        args = ['-f', font, '-w', str(width), '-j', justify, '-D', direction]
        if r:
            args.append('-r')
        if F:
            args.append('-F')
        if s:
            args.append('-s')
        elif n:
            args.append('-n')
        args.append(text)

        args_key = tuple(args)
        if args_key in seen_args:
            continue

        try:
            out = run_pyfiglet(args)
            seen_args.add(args_key)
            return {'args': args, 'out': out}
        except Exception:
            continue

def main():
    data_dir = os.path.join(script_dir, 'data')
    os.makedirs(data_dir, exist_ok=True)

    random.seed(7)
    seen_args = set()

    visible_path = os.path.join(data_dir, 'visible.jsonl')
    hidden_path = os.path.join(data_dir, 'hidden.jsonl')

    print("Generating visible.jsonl (2000 cases)...")
    with open(visible_path, 'w', encoding='utf-8') as f:
        for _ in range(2000):
            case = gen_case(seen_args)
            f.write(json.dumps(case, ensure_ascii=False) + '\n')
    print("Visible cases generated.")

    print("Generating hidden.jsonl (2000 cases)...")
    with open(hidden_path, 'w', encoding='utf-8') as f:
        for _ in range(2000):
            case = gen_case(seen_args)
            f.write(json.dumps(case, ensure_ascii=False) + '\n')
    print("Hidden cases generated.")

if __name__ == '__main__':
    main()
