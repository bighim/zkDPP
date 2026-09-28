const fs = require('node:fs');
const path = require('node:path');
const {execFileSync} = require('node:child_process');
const root = __dirname;
const workspace = path.resolve(root, '../../../..');
const build = path.join(workspace, '.latex-build', path.relative(workspace, root));
fs.mkdirSync(build, {recursive: true});
for (const name of fs.readdirSync(root).filter(n => n.endsWith('.tex'))) {
  execFileSync('latexindent', ['-wd', '-s', '-c', build + '/', "-y=defaultIndent: '    '", name], {cwd: root, stdio: 'inherit'});
}
for (let i = 0; i < 2; i++) {
  execFileSync('xelatex', ['-synctex=1', '-interaction=batchmode', '-halt-on-error', '-output-directory=' + build, 'main.tex'], {cwd: root, stdio: 'inherit'});
}
const log = fs.readFileSync(path.join(build, 'main.log'), 'utf8');
if (/Missing character:|Overfull \\[hv]box/.test(log)) throw new Error('Check rendering warnings in ' + build);
fs.copyFileSync(path.join(build, 'main.pdf'), path.join(root, 'main.pdf'));
console.log(path.join(root, 'main.pdf'));
