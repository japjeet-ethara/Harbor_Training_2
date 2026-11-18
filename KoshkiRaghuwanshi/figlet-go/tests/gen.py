import io, json, os, random, sys

sys.path.insert(0, os.path.abspath(os.path.join("..", "pyfiglet")))
import pyfiglet

FONTS = ['standard', 'slant', 'small', 'big', 'mini', 'banner', 'shadow', 'script']
WIDTHS = [20, 40, 60, 80, 120]
JUSTIFIES = ['auto', 'left', 'center', 'right']
DIRECTIONS = ['auto', 'left-to-right', 'right-to-left']

TRICKY = [
    '||', '_/', '|/', '[{', '[]', '()', '{}', '/\\', '\\/', '><',
    '|_|', '///\\\\\\', '___', '[({})]', '<><>', '/|\\_[]{}()<>',
    'hello  world', '  leading and trailing  ',
    'The quick brown fox jumps over the lazy dog 1234567890!',
    'Supercalifragilisticexpialidocious',
    'A very long sentence that definitely needs to wrap across multiple lines in narrow widths!',
    'café résumé naïve über straße niño',
    'Hello \u2603 \u2764 unknown', 'a\nb\nc', '   ', 'W M W M W',
    '1 + 2 = 3; (a * b) / c [0]', '===***===', 'H e l l o', '~`!@#$%^&*()_+-=',
]
NORMAL = ['hello', 'world', 'FIGlet', 'Go', 'Harbor', 'test', 'code', 'Testing 123', 'Docker', 'Alpine', 'Linux', 'Terminal']

def run_pyfiglet(args):
    raw_buf = io.BytesIO()
    wrapper = io.TextIOWrapper(raw_buf, encoding='utf-8')
    old_argv, old_stdout = sys.argv, sys.stdout
    sys.argv = ['pyfiglet'] + args
    sys.stdout = wrapper
    try:
        pyfiglet.main()
    except SystemExit:
        pass
    finally:
        if sys.stdout is wrapper:
            sys.stdout.flush()
        sys.argv, sys.stdout = old_argv, old_stdout
    return raw_buf.getvalue().decode('utf-8', errors='replace')

def make_case(rng, seen):
    while True:
        args = ['-f', rng.choice(FONTS), '-w', str(rng.choice(WIDTHS + [20, 30, 40]))]
        j = rng.choice(JUSTIFIES)
        if j != 'auto' or rng.random() < 0.5: args.extend(['-j', j])
        d = rng.choice(DIRECTIONS + ['right-to-left'])
        if d != 'auto' or rng.random() < 0.5: args.extend(['-D', d])
        for flag in ['-r', '-F', '-s', '-n']:
            if rng.random() < 0.25: args.append(flag)
        text = rng.choice(TRICKY) if rng.random() < 0.65 else ' '.join(rng.choice(NORMAL) for _ in range(rng.randint(1, 5)))
        args.append(text)
        key = tuple(args)
        if key in seen:
            continue
        try:
            out = run_pyfiglet(args)
            seen.add(key)
            return {'args': args, 'out': out}
        except pyfiglet.CharNotPrinted:
            continue

rng = random.Random(7)
seen = set()
os.makedirs('tests/data', exist_ok=True)
for fn in ['visible.jsonl', 'hidden.jsonl']:
    p = os.path.join('tests/data', fn)
    with open(p, 'w', encoding='utf-8', newline='\n') as f:
        for _ in range(2000): f.write(json.dumps(make_case(rng, seen), ensure_ascii=False) + '\n')
    print(f'Done {p}')