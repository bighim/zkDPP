// Format the current hand-edited report, then compile without regenerating content.
const fs = require('node:fs');
const path = require('node:path');
const {execFileSync} = require('node:child_process');
const {formatFiles, paperFiles} = require('./format-paper-tex.cjs');
const docs = path.resolve(__dirname, '..');
formatFiles(paperFiles());
const build = path.resolve(docs, '../.codex-build/paper-report');
fs.mkdirSync(build, {recursive: true});
for (let i = 0; i < 2; i++) execFileSync('xelatex', ['-interaction=batchmode', '-halt-on-error', '-output-directory=' + build, 'paper-evaluation-results.tex'], {cwd: docs, stdio: 'inherit'});
const output = path.join(docs, 'paper-evaluation-results.pdf');
fs.copyFileSync(path.join(build, 'paper-evaluation-results.pdf'), output);
console.log(output);
