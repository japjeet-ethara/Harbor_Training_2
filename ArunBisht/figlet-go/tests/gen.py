import io
import json
from pathlib import Path
import random
import sys

# Seed random at top as required
random.seed(7)

# Sibling pyfiglet repository folder is next to figlet-go
sibling_dir = Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(sibling_dir))
import pyfiglet

FONTS = ['standard', 'slant', 'small', 'big', 'mini', 'banner', 'shadow', 'script']
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ['left', 'right', 'center']
DIRECTIONS = ['left-to-right', 'right-to-left']
FLAGS = ['-r', '-F', '-s', '-n']

# Preload fonts for checking glyph widths
FONT_OBJS = {name: pyfiglet.Figlet(font=name) for name in FONTS}

WORDS = [
    'hello', 'world', 'harbor', 'task', 'golang', 'terminal', 'ascii',
    'banner', 'system', 'code', 'programming', 'computer', 'challenge',
    'stream', 'buffer', 'matrix', 'vector', 'string', 'package', 'docker',
    'runner', 'output', 'render', 'font', 'smush', 'width', 'justify',
    'reverse', 'flip', 'check', 'value', 'option', 'format', 'quick',
    'brown', 'fox', 'jumps', 'lazy', 'dog', 'simple', 'build', 'test',
]

TRICKY_CHARS = ['|', '/', '\\', '_', '[', ']', '{', '}', '(', ')', '<', '>']

LATIN1_WORDS = [
    'café', 'résumé', 'Straße', 'schön', 'señor', 'mañana',
    'façade', 'naïve', 'über', 'piñata', 'voilà', 'hôtel',
    'crème', 'Noël', 'rôle', 'élégant', 'brûlée',
]

MISSING_CHARS = ['☺', '★', '語', '€', '©', '™', '∆', '✓', '🚀', '♥', '♦', '♣', '♠']

LONG_SENTENCES = [
    'The quick brown fox jumps over the lazy dog repeatedly until evening falls upon the quiet forest',
    'Writing a Go port of pyfiglet requires handling character fonts sub-character smushing justification and line wrapping correctly',
    'To be or not to be that is the question whether tis nobler in the mind to suffer the slings and arrows of outrageous fortune',
    'In computer science FIGlet is a computer program that generates text banners in which characters are formed from smaller ASCII characters',
    'Software engineering involves designing developing testing deploying and maintaining high quality applications across diverse platforms',
    'When in the course of human events it becomes necessary for one people to dissolve the political bands connecting them',
    'Harbor task environments provide isolated reproducible sandboxes for testing autonomous agents with deterministic evaluation benchmarks',
    'Standard FIGlet font headers contain magic numbers height baseline maximum length old smush mode comment lines and print direction parameters',
]


class CaptureStringIO(io.StringIO):
    """Subclass of io.StringIO that supports detach() and bytes decoding."""
    def detach(self):
        return self

    def write(self, s):
        if isinstance(s, (bytes, bytearray)):
            s = s.decode('utf-8', errors='replace')
        return super().write(s)


def render_figlet(args):
    orig_argv = sys.argv
    orig_stdout = sys.stdout
    buf = CaptureStringIO()
    try:
        sys.argv = ['pyfiglet'] + args
        sys.stdout = buf
        pyfiglet.main()
        return buf.getvalue()
    finally:
        sys.argv = orig_argv
        sys.stdout = orig_stdout


def generate_text_for_font_width(font_name, width):
    f_obj = FONT_OBJS[font_name]
    while True:
        style = random.randint(0, 6)
        if style == 0:
            # Normal English words
            k = random.randint(1, 4)
            text = ' '.join(random.choices(WORDS, k=k))
        elif style == 1:
            # Tricky characters: | / \ _ [ ] { } ( ) < >
            pattern = random.randint(0, 5)
            if pattern == 0:
                c1 = random.choice(TRICKY_CHARS)
                c2 = random.choice(TRICKY_CHARS)
                w = random.choice(WORDS)
                text = f'{c1} {w} {c2}'
            elif pattern == 1:
                w1 = random.choice(WORDS)
                w2 = random.choice(WORDS)
                text = f'[{w1}] <{w2}>'
            elif pattern == 2:
                w1 = random.choice(WORDS)
                w2 = random.choice(WORDS)
                w3 = random.choice(WORDS)
                text = f'{w1}/{w2}\\{w3}'
            elif pattern == 3:
                w1 = random.choice(WORDS)
                w2 = random.choice(WORDS)
                text = f'{{{w1}}} ({w2})'
            elif pattern == 4:
                w1 = random.choice(WORDS)
                w2 = random.choice(WORDS)
                text = f'{w1} | {w2} _ ok'
            else:
                # All tricky characters together
                text = '| / \\ _ [ ] { } ( ) < >'
        elif style == 2:
            # Double spaces
            w1 = random.choice(WORDS)
            w2 = random.choice(WORDS)
            w3 = random.choice(WORDS)
            pattern = random.randint(0, 2)
            if pattern == 0:
                text = f'{w1}  {w2}'
            elif pattern == 1:
                text = f'{w1}  {w2}  {w3}'
            else:
                text = f'  {w1}  {w2}  '
        elif style == 3:
            # Long sentences that must wrap
            if random.random() < 0.5:
                text = random.choice(LONG_SENTENCES)
            else:
                text = ' '.join(random.choices(WORDS, k=random.randint(12, 22)))
        elif style == 4:
            # Latin-1 letters like é ß ñ
            lat = random.choice(LATIN1_WORDS)
            w = random.choice(WORDS)
            if random.random() < 0.5:
                text = f'{lat} {w}'
            else:
                lat2 = random.choice(LATIN1_WORDS)
                text = f'{lat} {lat2}'
        elif style == 5:
            # Characters the font does not have
            m = random.choice(MISSING_CHARS)
            w = random.choice(WORDS)
            if random.random() < 0.5:
                text = f'{w} {m}'
            else:
                m2 = random.choice(MISSING_CHARS)
                text = f'{m} {w} {m2}'
        else:
            # Mixed: words, tricky chars, double spaces, Latin-1, missing chars
            w = random.choice(WORDS)
            c = random.choice(TRICKY_CHARS)
            lat = random.choice(LATIN1_WORDS)
            m = random.choice(MISSING_CHARS)
            text = f'{c}{w}  {lat} {m}{c}'

        # Ensure all characters fit within width in this font
        if all(f_obj.Font.width.get(ord(ch), 0) < width for ch in text):
            return text


def generate_case():
    font_name = random.choice(FONTS)
    width = random.choice(WIDTHS)
    justify = random.choice(JUSTIFIES)
    direction = random.choice(DIRECTIONS)

    text = generate_text_for_font_width(font_name, width)

    args = ['-f', font_name, '-w', str(width), '-j', justify, '-D', direction]
    for flag in FLAGS:
        if random.random() < 0.25:
            args.append(flag)
    args.append(text)

    out = render_figlet(args)
    return {'args': args, 'out': out}


def main():
    data_dir = Path(__file__).resolve().parent / 'data'
    data_dir.mkdir(parents=True, exist_ok=True)

    visible_path = data_dir / 'visible.jsonl'
    hidden_path = data_dir / 'hidden.jsonl'

    # Generate all cases into memory before writing to disk
    # This prevents truncating files while parallel readers (e.g. bash <(cat ...)) access them
    visible_lines = [json.dumps(generate_case(), ensure_ascii=False) + '\n' for _ in range(2000)]
    hidden_lines = [json.dumps(generate_case(), ensure_ascii=False) + '\n' for _ in range(2000)]

    with open(visible_path, 'w', encoding='utf-8') as f:
        f.writelines(visible_lines)

    with open(hidden_path, 'w', encoding='utf-8') as f:
        f.writelines(hidden_lines)


if __name__ == '__main__':
    main()
