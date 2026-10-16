// Current numeric edition; the fallback below preserves the historical extractor.
if (require('fs').existsSync(require('path').join(__dirname, '../paper-data/runs/2026-10-16-run01/measurements.json'))) {
  require('./build-current-paper-data.cjs');
} else {
// Read-only extraction from the published experiment Raw; no benchmark execution.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const root = path.resolve(__dirname, '../..');
require('./build-paper-tables.cjs');
const docs = path.join(root, 'docs');
const figdir = path.join(docs, 'paper-figures');
const datadir = path.join(docs, 'paper-data');
fs.mkdirSync(figdir, { recursive: true });
fs.mkdirSync(datadir, { recursive: true });
const inputs = {};
function load(n) {
  const p = path.join(root, `output/experiments/experiment-${n}.json`);
  const bytes = fs.readFileSync(p);
  inputs[`output/experiments/experiment-${n}.json`] = crypto.createHash('sha256').update(bytes).digest('hex');
  const e = JSON.parse(bytes);
  if (e.status !== 'complete' || e.repetitions !== 5 || e.warmups !== 1) throw Error(`Incomplete experiment ${n}`);
  return e;
}
const e1 = load(1), e2 = load(2), e3 = load(3), e4 = load(4), e8 = load(8);
function runs(rows, test) {
  const r = rows.filter(x => x.Run > 0 && test(x)).sort((a,b) => a.Run-b.Run);
  if (r.length !== 5 || r.some((x,i) => x.Run !== i+1 || x.Matched !== true)) throw Error('Expected five matched official runs');
  return r;
}
function stat(rows, fn) {
  const a = rows.map(fn).sort((a,b) => a-b);
  if (a.some(v => !Number.isFinite(v))) throw Error('Invalid metric');
  return { value:a[2], min:a[0], max:a[4], low:a[2]-a[0], high:a[4]-a[2] };
}
const format = (v,d=0) => v.toLocaleString('en-US', {minimumFractionDigits:d, maximumFractionDigits:d});
const eventNames = ['Entry','Transfer','Proceed','Recall','Merge','Split','Process','Exit','Issue'];
const positions = [1,3,6,4,9,12,11,14,13];
const audit = eventNames.map((name,i) => {
  const a = runs(e1.rows, r=>r.Case===`exp1-lifecycle-${String(positions[i]).padStart(2,'0')}`);
  const b = runs(e3.rows, r=>r.Case===`exp3-lifecycle-${String(positions[i]).padStart(2,'0')}`);
  return {name, constraints:a[0].Constraints, plainConstraints:b[0].Constraints,
    gas:stat(a,r=>r.Tx.Gas), plainGas:stat(b,r=>r.Tx.Gas), e2e:stat(a,r=>r.E2EMS), plainE2E:stat(b,r=>r.E2EMS)};
});
const routes = Object.fromEntries(['direct','proceed','recall'].map(name => {
  const a = runs(e2.routes, r=>r.Route===name);
  return [name,{gas:stat(a,r=>r.Gas),time:stat(a,r=>r.TotalMS),tx:a[0].Transactions}];
}));
const graphNames = {chain:'연속 Split',balanced:'균형 Split',diamond:'Split → Merge'};
const graphs = [];
for (const kind of Object.keys(graphNames)) for (let count of [10,100,1000]) {
  if (kind==='diamond') count++;
  const row = {kind,count};
  for (const op of ['forward','backward','freeze']) {
    const a = runs(e4.rows, r=>r.Case===`${kind}-${count}` && r.Operation===op);
    row[op] = {time:stat(a,r=>r.TotalMS/1000),gas:stat(a,r=>r.Gas),visits:a[0].UniqueRecords,
      decryptions:a[0].Decryptions,targets:a[0].Targets,transactions:a[0].Transactions};
  }
  graphs.push(row);
}
const supplementPath = 'output/paper-supplement/process-independent.json';
const supplementBytes = fs.readFileSync(path.join(root,supplementPath));
const supplement = JSON.parse(supplementBytes);
const verificationPath = 'output/paper-supplement/verification-after.json';
const verificationBytes = fs.readFileSync(path.join(root,verificationPath));
const verification = JSON.parse(verificationBytes);
inputs[supplementPath] = crypto.createHash('sha256').update(supplementBytes).digest('hex');
inputs[verificationPath] = crypto.createHash('sha256').update(verificationBytes).digest('hex');
if(supplement.status!=='complete' || supplement.repetitions!==1 || supplement.warmups!==0 || supplement.officialCases!==10 || supplement.rows.length!==10) throw Error('Supplement incomplete');
if(verification.status!=='complete' || !verification.sourceUnchangedSinceMeasurement || verification.officialProofs!==10 || verification.pairs.length!==5 || !verification.pairs.every(p=>p.inputsOutputsRootsPolicyEqual) || verification.files[supplementPath]!==inputs[supplementPath]) throw Error('Supplement verification mismatch');
const arities = ['2-1','3-1','4-1','2-2','2-3'];
const processes = arities.map(arity=>{
 const item={arity};
 for(const variant of ['plain','audit']) {
  const matches=supplement.rows.filter(r=>r.Profile===`process-${arity}-${variant}`);
  if(matches.length!==1)throw Error('Supplement duplicate/missing');
  const r=matches[0];
  if(r.Run!==1||r.Warmup||!r.Matched||r.Tx.Status!==1)throw Error('Invalid official row');
  item[variant]={constraints:r.Constraints,gas:{value:r.Tx.Gas}};
 }
 return item;
});
const keys = Object.fromEntries(['2-of-3','2-of-5','2-of-10','2-of-20','3-of-10','5-of-10','7-of-10','10-of-10'].map(id=>{
  const a = runs(e8.rows,r=>r.Case===id);
  return [id,{k:a[0].K,n:a[0].N,time:stat(a,r=>r.TotalMS/r.InnerIterations),
    validation:stat(a,r=>r.Recovery.ValidationMS/r.InnerIterations),
    interpolation:stat(a,r=>r.Recovery.InterpolationMS/r.InnerIterations)}];
}));
function csv(name, header, rows) {
  const quote = s => /[,"\n]/.test(String(s)) ? '"'+String(s).replaceAll('"','""')+'"' : String(s);
  fs.writeFileSync(path.join(datadir,name+'.csv'),[header,...rows].map(r=>r.map(quote).join(',')).join('\n')+'\n');
}
csv('events',['event','constraints','gas_median','gas_min','gas_max','e2e_ms_median','e2e_ms_min','e2e_ms_max'],audit.map(r=>[r.name,r.constraints,r.gas.value,r.gas.min,r.gas.max,r.e2e.value,r.e2e.min,r.e2e.max]));
csv('audit-overhead',['event','constraints_off','constraints_on','gas_off','gas_on'],audit.map(r=>[r.name,r.plainConstraints,r.constraints,r.plainGas.value,r.gas.value]));
csv('transfer-routes',['route','transactions','gas_median','gas_min','gas_max'],Object.entries(routes).map(([k,v])=>[k,v.tx,v.gas.value,v.gas.min,v.gas.max]));
csv('tracing',['graph','total_records','direction','visited_records','decryptions','seconds_median','seconds_min','seconds_max'],graphs.flatMap(g=>['forward','backward'].map(d=>[g.kind,g.count,d,g[d].visits,g[d].decryptions,g[d].time.value,g[d].time.min,g[d].time.max])));
csv('freeze',['graph','total_records','targets','transactions','seconds_median','seconds_min','seconds_max','total_gas'],graphs.map(g=>[g.kind,g.count,g.freeze.targets,g.freeze.transactions,g.freeze.time.value,g.freeze.time.min,g.freeze.time.max,g.freeze.gas.value]));
csv('process',['inputs_outputs','constraints_off','constraints_on','gas_off','gas_on'],processes.map(r=>[r.arity,r.plain.constraints,r.audit.constraints,r.plain.gas.value,r.audit.gas.value]));
csv('key-recovery',['k','n','ms_median','ms_min','ms_max','validation_ms_median','interpolation_ms_median'],Object.values(keys).map(r=>[r.k,r.n,r.time.value,r.time.min,r.time.max,r.validation.value,r.interpolation.value]));

const axis = 'width=7.8cm,height=6.0cm,grid=major,major grid style={gray!18},axis line style={gray!60},tick label style={font=\\small},label style={font=\\small},title style={font=\\small\\bfseries},legend style={font=\\footnotesize,draw=none},scaled ticks=false';
const colors = ['blue!65!black','orange!85!black','teal!70!black'];
function group(plots) {return '\\begin{tikzpicture}\n\\begin{groupplot}[group style={group size=2 by 1,horizontal sep=1.8cm},'+axis+']\n'+plots.join('\n')+'\n\\end{groupplot}\n\\end{tikzpicture}\n';}
const barPlot = (data,color,shift) => `\\addplot+[ybar,bar width=4pt,bar shift=${shift}pt,draw=${color},fill=${color}!65,mark=none] coordinates {${data.map(([x,y])=>`(${x},${y})`).join(' ')}};\n`;
function bars(labels, left, right, ylabel, title) {
  return `\\nextgroupplot[ybar,single ybar legend,ymin=0,ylabel={${ylabel}},title={${title}},symbolic x coords={${labels.join(',')}},xtick=data,x tick label style={rotate=45,anchor=east,font=\\footnotesize},enlarge x limits=0.08,legend style={at={(0.5,-0.40)},anchor=north,legend columns=2}]\n`+
    barPlot(labels.map((x,i)=>[x,left[i]]),colors[0],-2.6)+barPlot(labels.map((x,i)=>[x,right[i]]),colors[1],2.6)+'\\legend{Audit off,Audit on}\n';
}
function points(values, color, mark, only=false) {
 return `\\addplot+[${only?'only marks,':''}color=${color},mark=${mark},mark size=${mark==='*'?2.8:2}pt,mark options={solid,fill=white,draw=${color}},thick] coordinates {`+
  values.map(([x,s])=>`(${x},${s.value})`).join(' ')+'};\n';
}
const figures = {};
figures['audit-overhead'] = group([
  bars(eventNames,audit.map(r=>r.plainConstraints/1000),audit.map(r=>r.constraints/1000),'Constraints (thousands)','(a) Circuit cost'),
  bars(eventNames,audit.map(r=>r.plainGas.value/1e6),audit.map(r=>r.gas.value/1e6),'Gas (millions)','(b) Transaction cost')
]);
const legend='';
const marks=['*','square*','triangle*'];
figures['tracing-time']='\\begin{tikzpicture}\n\\begin{axis}[width=9cm,height=6.5cm,grid=major,major grid style={gray!18},axis line style={gray!60},tick label style={font=\\small},label style={font=\\small},scaled ticks=false,xmode=log,ymode=log,log basis x=10,log basis y=10,xlabel={Unique visited AuditRecords},ylabel={Tracing time (s)},legend style={at={(0.5,-0.25)},anchor=north,legend columns=2,font=\\small,draw=none}]\n'+
 ['forward','backward'].map((d,i)=>points(graphs.filter(g=>g.kind==='chain').map(g=>[g[d].visits,g[d].time]),colors[i],i?'square*':'*')).join('')+'\\legend{Forward,Backward}\n\\end{axis}\n\\end{tikzpicture}\n';
figures['freeze-cost']='\\begin{tikzpicture}\n\\begin{axis}[width=9cm,height=6.5cm,grid=major,major grid style={gray!18},axis line style={gray!60},tick label style={font=\\small},label style={font=\\small},scaled ticks=false,xmode=log,ymode=log,log basis x=10,log basis y=10,xlabel={Freeze targets},ylabel={Freeze time (s)}]\n'+
 points(graphs.filter(g=>g.kind==='chain').map(g=>[g.freeze.targets,g.freeze.time]),colors[0],'*')+'\\end{axis}\n\\end{tikzpicture}\n';
for(const direction of ['input','output']) {
 const input=direction==='input';
 const cases=input?['2-1','3-1','4-1']:['2-1','2-2','2-3'];
 const xs=input?[2,3,4]:[1,2,3];
 const xlabel=input?'Input commitments ($M$)':'Output commitments ($N$)';
 figures['process-'+direction+'-sweep']=group(['constraints','gas'].map((metric,i)=>{
  const ylabel=i?'Gas (millions)':'Constraints (thousands)';
  const divisor=i?1e6:1000;
  const title=i?'(b) Gas':'(a) Constraints';
  const coords=cases.map((c,j)=>{const p=processes.find(p=>p.arity===c).audit;return [xs[j],{value:(i?p.gas.value:p.constraints)/divisor}];});
  return `\\nextgroupplot[xlabel={${xlabel}},ylabel={${ylabel}},title={${title}},xtick={${xs.join(',')}},ymin=0,ymax=${i?4.2:100}]\n`+points(coords,colors[0],'*');
 }));
}
figures['key-recovery']=group([
 `\\nextgroupplot[xlabel={Shares used ($k$)},ylabel={Local recovery time (ms)},title={(a) Committee $n=10$},xtick={2,3,5,7,10},ymin=0]\n`+points([2,3,5,7,10].map(k=>[k,keys[`${k}-of-10`].time]),colors[0],'*'),
 `\\nextgroupplot[xlabel={Committee members ($n$)},ylabel={Local recovery time (ms)},title={(b) Shares $k=2$},xtick={3,5,10,20},ymin=0]\n`+points([3,5,10,20].map(n=>[n,keys[`2-of-${n}`].time]),colors[1],'square*')
]);
for (const [name,content] of Object.entries(figures)) {
  fs.writeFileSync(path.join(figdir,name+'.tex'),content);
  fs.writeFileSync(path.join(figdir,name+'-standalone.tex'),'\\documentclass[border=6pt]{standalone}\n\\usepackage{pgfplots}\n\\pgfplotsset{compat=1.18}\n\\usepgfplotslibrary{groupplots}\n\\begin{document}\n'+content+'\\end{document}\n');
}

const texEscape = s => s.replace(/[\\%&_#]/g,c=>({'\\':'\\textbackslash{}','%':'\\%','&':'\\&','_':'\\_','#':'\\#'}[c])).replaceAll('→','$\\rightarrow$');
function rich(s) {
 return s.split('**').map((x,i)=>i%2?'\\textbf{'+texEscape(x)+'}':texEscape(x)).join('').replaceAll('\n','\\newline\n');
}
let md='# zkDPP 논문용 실험 결과 초안\n\n',tex=String.raw`\documentclass[11pt,a4paper]{article}
\usepackage{kotex}
\usepackage[margin=18mm]{geometry}
\usepackage{amsmath,booktabs,array,graphicx,needspace}
\usepackage{pgfplots}
\pgfplotsset{compat=1.18}
\usepgfplotslibrary{groupplots}
\usepackage[hidelinks]{hyperref}
\usepackage{xurl}
\setlength{\parindent}{0pt}
\setlength{\parskip}{6pt}
\setlength{\emergencystretch}{1em}
\renewcommand{\arraystretch}{1.18}
\begin{document}
{\LARGE\bfseries zkDPP 논문용 실험 결과 초안}\par\medskip
`;
function p(s){md+=s.replaceAll('\n','  \n')+'\n\n';tex+=rich(s)+'\n\n';}
function heading(s){md+='## '+s+'\n\n';if(/^[3-5]\./.test(s))tex+='\\clearpage\n';tex+='\\needspace{8\\baselineskip}\\section*{'+texEscape(s)+'}\n';}
function sub(s){md+='### '+s+'\n\n';if(/^3\.[23]/.test(s))tex+='\\clearpage\n';tex+='\\needspace{6\\baselineskip}\\subsection*{'+texEscape(s)+'}\n';}
function table(head,rows,textCols=1){
 md+='| '+head.join(' | ')+' |\n|'+head.map((_,i)=>i<textCols?'---':'---:').join('|')+'|\n';
 for(const r of rows)md+='| '+r.join(' | ')+' |\n';md+='\n';
 tex+='\\begin{center}\\small\\begin{tabular}{@{}'+head.map((_,i)=>i<textCols?'l':'r').join('')+'@{}}\\toprule\n'+head.map(h=>'\\textbf{'+texEscape(h)+'}').join(' & ')+' \\\\\n\\midrule\n';
 for(const r of rows)tex+=r.map(texEscape).join(' & ')+' \\\\\n';
 tex+='\\bottomrule\\end{tabular}\\end{center}\n';
}
function figure(name,caption){
 md+=`![${caption}](paper-figures/${name}.png)\n\n**${caption}**\n\n`;
 md+=fs.readFileSync(path.join(docs,'paper-tables',name+'.md'),'utf8')+'\n';
  tex+='\\par\\noindent\\begin{minipage}{\\linewidth}\n\\centering'+((name==='freeze-cost'||name==='tracing-time')?'\\input{paper-figures/'+name+'.tex}':'\\resizebox{\\linewidth}{!}{\\input{paper-figures/'+name+'.tex}}')+'\n\\par\\smallskip{\\small '+rich(caption)+'}\\par\\medskip\n\\input{paper-tables/'+name+'.tex}\n\\end{minipage}\\par\\medskip\n';
}
heading('실험 환경');
table(['항목','설정'],[["언어","Go 1.25.7"],["Circuit 라이브러리","gnark 0.15.0 / gnark-crypto 0.20.1"],["Proof system","PLONK-KZG / BLS12-381"],["감사 암호화","Jubjub / Poseidon2"],["병렬 실행 설정","GOMAXPROCS=8"],["블록체인 노드","Anvil 1.7.1"],["Contract 컴파일러","Solidity 0.8.30"],["EVM","Prague"],["Chain ID","31337"],["Transaction gas limit","30,000,000"]],2);
heading('1. 공급망 Event별 실행 비용');
p('**고정 조건**\nState는 3개이며 AuditRecord를 기록합니다.\nProcess의 입출력은 2→1입니다.');
p('**측정 기준**\nE2E는 witness 준비부터 같은 transaction의 성공 receipt 확인까지입니다.\nGas는 해당 transaction의 실제 gasUsed입니다.');
table(['Event','Constraints','Gas','E2E (ms)'],audit.map(r=>[r.name,format(r.constraints),format(r.gas.value),format(r.e2e.value,3)]));
heading('2. zkTransfer와 Voucher 전달 Tradeoff 비용');
p('**비교 조건**\nzkTransfer는 AuditRecord를 기록하지 않습니다.\nTransfer → Recall은 AuditRecord를 기록합니다.');
const auditedRecallGas=audit.find(r=>r.name==='Transfer').gas.value+audit.find(r=>r.name==='Recall').gas.value;
table(['전달 방식','AuditRecord','총 Gas'],[['zkTransfer','없음',format(routes.direct.gas.value)],['Transfer → Recall','있음',format(auditedRecallGas)]],2);
p('Transfer → Recall의 총 Gas는 Event별 대표 Gas를 합산한 값입니다.');
csv('transfer-comparison',['route','audit_record','gas','aggregation'],[['zkTransfer','no',routes.direct.gas.value,'median of direct transaction'],['Transfer->Recall','yes',auditedRecallGas,'sum of Event medians']]);
heading('3. Audit 추가 비용과 Auditor 수행 비용');
sub('3.1 Event별 Audit 추가 비용');
p('**고정 조건**\n각 Event의 입력·출력 State와 시작 Tree를 맞춥니다.\nowner·membership·소비 방지·Status·Policy 검사는 유지합니다.\n**변경 조건**\nAudit 암호화·기록·Tracing mapping을 추가한 경우와 추가하지 않은 경우를 비교합니다.');
figure('audit-overhead','그림 1. 동일 Event의 Audit 없음·있음 비교. 왼쪽은 Constraints, 오른쪽은 실제 transaction Gas입니다.');
const cd=audit.map(r=>r.constraints-r.plainConstraints),gd=audit.map(r=>r.gas.value-r.plainGas.value);
p(`**해석**\n9개 Event 모두 Constraints와 Gas가 증가했습니다.\nConstraints 증가량은 ${format(Math.min(...cd))}–${format(Math.max(...cd))}입니다.\nGas 증가량은 ${format(Math.min(...gd))}–${format(Math.max(...gd))}입니다.\nGas 차이는 Proof 검증·상태 변경을 포함하며 순수 SSTORE 비용만의 차이는 아닙니다.`);
const proc=audit.find(r=>r.name==='Process');
p(`**시간 해석의 한계**\nProcess E2E median은 Audit 없음 ${format(proc.plainE2E.value,3)} ms, 있음 ${format(proc.e2e.value,3)} ms였습니다.\n공식 5회의 범위가 겹치므로 이 감소를 성능 개선으로 결론 내리지 않습니다.\nConstraints·Gas는 추가 연산과 온체인 비용을 보여줍니다.\n실제 Prove 시간·메모리·RPC 대기는 별도의 지표입니다.`);
sub('3.2 Tracing 비용');
p('**기록 수와 측정 범위**\n가로축은 실제로 방문한 고유 AuditRecord 수입니다.\n같은 기록을 다시 만나도 한 건으로 셉니다.\n기록 조회·복호화·연결 탐색을 측정합니다.\nMaster Key 복구·Snapshot 확보·Freeze는 제외합니다.\n각 실행은 빈 cache에서 시작합니다.');
figure('tracing-time','그림 2. Unique visited AuditRecords에 따른 Tracing 시간.');
p('**구조와 시작점**\n고유 AuditRecord 10·100·1,000개를 방문했을 때의 시간을 비교합니다.\nForward는 최초 cm에서 시작합니다.\nBackward는 가장 깊은 최종 Note에서 조상을 찾습니다.');
p('기록 수 증가에 따른 비용을 비교합니다.\n다른 연결 구조와의 비교는 이 그림에 포함하지 않습니다.');
sub('3.3 Freeze 비용');
p('**동결 시간**\nTracing으로 대상 목록을 먼저 확보한 뒤 측정합니다.\n제출 전 상태 검사부터 동결 transaction 실행·최종 상태 확인까지 포함합니다.\n현재 구현은 대상 한 개마다 transaction 한 개를 제출합니다.');
const singleFreeze = graphs.find(g=>g.kind==='diamond' && g.count===11).freeze;
p('**한 개를 동결하는 Gas**\nNote 한 개를 Active에서 Frozen으로 변경하는 실제 transaction은 **'+format(singleFreeze.gas.value)+' Gas**를 사용했습니다.');
figure('freeze-cost','그림 3. 동결 대상 수에 따른 Freeze 시간.');
heading('4. Process 입출력 cm 수에 따른 비용');
p('**고정 조건**\nState는 3개이며 입력 합계는 (320,30,231), 출력 합계는 (300,30,261)입니다.\n질량 손실률 6.25%, 탄소 추가 30, WASTE 생성률 0%를 유지합니다.');
p('**입력 수 증가**\n출력은 1개로 고정하고 입력을 2·3·4개로 늘립니다.\n**출력 수 증가**\n입력은 2개로 고정하고 출력을 1·2·3개로 늘립니다.');
p('**측정 조건**\n입출력 허용 범위를 확장한 보충 원장에서 각 조건을 공식 1회 측정했습니다.\nAuditRecord를 기록하는 zkDPP Process 결과입니다.');
figure('process-input-sweep','그림 4. 출력 N=1을 고정하고 입력 수를 늘렸을 때의 비용.');
figure('process-output-sweep','그림 5. 입력 M=2를 고정하고 출력 수를 늘렸을 때의 비용.');
heading('5. k-of-n Master Key 복구 비용');
p('**두 비교축**\n첫 번째 그래프는 Committee n=10을 고정하고 사용하는 share k를 늘립니다.\n두 번째 그래프는 share k=2를 고정하고 Committee n을 늘립니다.');
p('**측정 범위**\npackage·share 검사부터 보간·공개키 대조까지의 로컬 복구 시간입니다.\npackage 생성·DKG·위원 통신·승인 대기는 제외합니다.\n온체인 transaction이 없으므로 Gas는 측정하지 않습니다.');
figure('key-recovery','그림 6. k와 n을 각각 변경한 로컬 Master Key 복구 시간.');
p('**반복 기준**\n각 공식 run에서 복구를 1,000회 실행합니다.\n그 시간을 1,000으로 나누어 복구 한 번의 시간을 구합니다.\n공식 run 5회의 median을 표시합니다.\n두 그래프의 2-of-10은 동일한 측정값입니다.');
p('**해석**\n현재 구현에서는 package·share 검사 시간이 큰 비중을 차지합니다.\nn 증가 그래프는 보간에 사용하는 share가 많아지는 실험이 아닙니다.\nshare는 두 개로 유지하고 검사할 전체 package 규모만 늘렸습니다.\n구간별 median의 합은 전체 median과 같지 않을 수 있습니다.');
heading('재현성과 범위');
p('기존 원자료는 output/experiments의 실험 1·2·3·4·8입니다.\nProcess 보충 원자료는 output/paper-supplement/process-independent.json입니다.\n집계 CSV와 원자료 SHA-256은 docs/paper-data에 저장했습니다.\nProcess 보충 실험은 각 조건 1회입니다.\n다른 항목의 기존 5회 median과 통계를 합치지 않습니다.\nAnvil 관측으로 Production finality·동시 처리량·Privacy 보안을 입증하지 않습니다.');
tex+='\\end{document}\n';
if(!process.argv.includes('--assets-only')) {
fs.writeFileSync(path.join(docs,'paper-evaluation-results.md'),md);
fs.writeFileSync(path.join(docs,'paper-evaluation-results.tex'),tex);
}
fs.writeFileSync(path.join(datadir,'provenance.json'),JSON.stringify({source:'published experiments and verified Process supplement',warmups:{legacy:1,process:0},officialRuns:{legacy:5,process:1},statistic:{legacy:'median',process:'single observation'},errorBars:'not displayed; min-max retained in CSV',displayedGraphSeries:['chain'],newSupplementIncluded:true,inputs},null,2)+'\n');
console.log(JSON.stringify({figures:Object.keys(figures),inputs:Object.keys(inputs),officialRuns:{legacy:5,process:1},supplementIncluded:true}));

}
