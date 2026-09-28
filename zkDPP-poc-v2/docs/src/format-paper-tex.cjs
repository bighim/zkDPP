// Same formatter and indentation as LaTeX Workshop. No line-reflow (-m).
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const {execFileSync} = require('node:child_process');
const docs = path.resolve(__dirname, '..');
function paperFiles() {
  return [path.join(docs, 'paper-evaluation-results.tex'), ...fs.readdirSync(path.join(docs, 'paper-modules')).filter(n => n.endsWith('.tex')).sort().map(n => path.join(docs, 'paper-modules', n))];
}
function formatFiles(files, check = false) {
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'zkdpp-latexindent-'));
  let changed = 0;
  for (const file of files) {
    const before = fs.readFileSync(file, 'utf8');
    const after = execFileSync('latexindent', ['-c', scratch + '/', file, "-y=defaultIndent: '    '"], {cwd: path.dirname(file), encoding: 'utf8', maxBuffer: 16 * 1024 * 1024});
    if (before === after) continue;
    if (fs.readFileSync(file, 'utf8') !== before) throw new Error('File changed during formatting: ' + file);
    if (!check) fs.writeFileSync(file, after);
    console.log((check ? 'Needs format: ' : 'Formatted: ') + path.relative(docs, file));
    changed++;
  }
  return changed;
}
module.exports = {formatFiles, paperFiles};
if (require.main === module) {
  const check = process.argv.includes('--check');
  const changed = formatFiles(paperFiles(), check);
  if (check && changed) process.exitCode = 1;
  else console.log(check ? 'All paper TeX files are formatted.' : 'Formatting complete.');
}
