"""Normalize native Go coverage paths to the scanner checkout without losing counters."""
from pathlib import Path
import re


def prepare(root, report, output):
    root = Path(root).resolve()
    module_match = re.search(r'^module\s+(\S+)', (root / 'go.mod').read_text(encoding='utf-8'), re.MULTILINE)
    if not module_match:
        raise ValueError('go.mod must declare a module')
    module = module_match[1] + '/'
    rows = Path(report).read_text(encoding='utf-8').splitlines()
    if len(rows) < 2 or rows[0] not in {'mode: atomic', 'mode: count', 'mode: set'}:
        raise ValueError('Coverage must contain a Go mode and source records')
    normalized = [rows[0]]
    for row in rows[1:]:
        match = re.fullmatch(r'(.+):(\d+\.\d+,\d+\.\d+\s+\d+\s+\d+)', row)
        if not match:
            raise ValueError('Malformed Go coverage record')
        name = match[1].replace('\\', '/')
        if name.startswith(module):
            name = name[len(module):]
        source = (root / name).resolve()
        try:
            relative = source.relative_to(root)
        except ValueError:
            raise ValueError('Coverage references a path outside the checkout') from None
        if source.suffix != '.go' or not source.is_file():
            raise ValueError('Coverage references a missing Go source file')
        normalized.append(relative.as_posix() + ':' + match[2])
    with Path(output).open('w', encoding='utf-8', newline='\n') as destination:
        destination.write('\n'.join(normalized) + '\n')


if __name__ == '__main__':
    prepare(Path.cwd(), Path('coverage.out'), Path('coverage-sonar.out'))
