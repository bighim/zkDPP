#!/usr/bin/env python3
"""Build the existing manuscript locally without modifying archived PDFs."""
from pathlib import Path
import shutil
import subprocess

root = Path(__file__).resolve().parents[1]
paper = root / 'paper'
out = paper / 'build' / 'portable'
out.mkdir(parents=True, exist_ok=True)

def executable(name):
    path = shutil.which(name)
    if path:
        return path
    fallback = Path('/Library/TeX/texbin') / name
    if fallback.is_file():
        return str(fallback)
    raise SystemExit(f'{name} is required. Install a TeX distribution with XeLaTeX/BibTeX.')

xelatex, bibtex = executable('xelatex'), executable('bibtex')
tex_command = [xelatex, '-no-shell-escape', '-interaction=nonstopmode',
               '-halt-on-error', '-output-directory=' + str(out), 'main.tex']
commands = [tex_command, [bibtex, str((out / 'main').relative_to(paper))], tex_command, tex_command]
for number, command in enumerate(commands, 1):
    result = subprocess.run(command, cwd=paper, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, text=True)
    (out / f'pass-{number}.log').write_text(result.stdout)
    if result.returncode:
        print(result.stdout[-5000:])
        raise SystemExit(result.returncode)
print(out / 'main.pdf')
