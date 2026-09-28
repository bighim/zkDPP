// Numeric companions to the paper figures. Reads Raw; never runs experiments.
const fs = require('fs');
const path = require('path');
const root = path.resolve(__dirname, '../..');
const out = path.join(root, 'docs/paper-tables');
fs.mkdirSync(out, {recursive:true});
const load=n=>JSON.parse(fs.readFileSync(path.join(root,`output/experiments/experiment-${n}.json`)));
const median=a=>[...a].sort((x,y)=>x-y)[Math.floor(a.length/2)];
const fmt=(v,d=0)=>v.toLocaleString('en-US',{minimumFractionDigits:d,maximumFractionDigits:d});
const rows=(e,p)=>{const a=e.rows.filter(r=>r.Run>0&&p(r));if(a.length!==5||a.some(r=>!r.Matched))throw Error('Expected five validated observations');return a;};
const e1=load(1),e3=load(3),e4=load(4),e8=load(8);
const names=['Entry','Transfer','Proceed','Recall','Merge','Split','Process','Exit','Issue'];
const positions=[1,3,6,4,9,12,11,14,13];
const audit=names.map((name,i)=>{
 const a=rows(e1,r=>r.Case===`exp1-lifecycle-${String(positions[i]).padStart(2,'0')}`);
 const b=rows(e3,r=>r.Case===`exp3-lifecycle-${String(positions[i]).padStart(2,'0')}`);
 const off=b[0].Constraints,on=a[0].Constraints;
 const goff=median(b.map(r=>r.Tx.Gas)),gon=median(a.map(r=>r.Tx.Gas));
 return {name,off,on,goff,gon};
});
const groups={};
groups['audit-overhead']=[
 {title:'(a) Constraints',head:['Event','Audit X','Audit O','증가량'],data:audit.map(r=>[r.name,fmt(r.off),fmt(r.on),fmt(r.on-r.off)])},
 {title:'(b) Gas',head:['Event','Audit X','Audit O','증가량'],data:audit.map(r=>[r.name,fmt(r.goff),fmt(r.gon),fmt(r.gon-r.goff)])}
];
groups['tracing-time']=[{
 title:'Tracing time',head:['Visited AuditRecords','Forward (s)','Backward (s)'],
 data:[10,100,1000].map(n=>{
  const f=rows(e4,r=>r.Case===`chain-${n}`&&r.Operation==='forward');
  const b=rows(e4,r=>r.Case===`chain-${n}`&&r.Operation==='backward');
  if(f[0].UniqueRecords!==n||b[0].UniqueRecords!==n)throw Error('Different visited-count axes');
  return [fmt(n),fmt(median(f.map(r=>r.TotalMS))/1000,3),fmt(median(b.map(r=>r.TotalMS))/1000,3)];
 })
}];
groups['freeze-cost']=[{title:'Freeze',head:['동결 대상 수','시간 (s)'],data:[10,100,1000].map(n=>{
 const a=rows(e4,r=>r.Case===`chain-${n}`&&r.Operation==='freeze');return [fmt(a[0].Targets),fmt(median(a.map(r=>r.TotalMS))/1000,3)];
})}];
const supplement=JSON.parse(fs.readFileSync(path.join(root,'output/paper-supplement/process-independent.json')));
if(supplement.status!=='complete'||supplement.repetitions!==1||supplement.rows.length!==10)throw Error('Supplement incomplete');
for(const direction of ['input','output']) {
 const input=direction==='input';
 groups['process-'+direction+'-sweep']=[{
  title:input?'Outputs N=1':'Inputs M=2',head:[input?'Input M':'Output N','Constraints','Gas'],
  data:(input?['2-1','3-1','4-1']:['2-1','2-2','2-3']).map(arity=>{
   const a=supplement.rows.filter(r=>r.Profile===`process-${arity}-audit`);
   if(a.length!==1||!a[0].Matched||a[0].Tx.Status!==1)throw Error('Invalid audit case');
   return [arity.split('-')[input?0:1],fmt(a[0].Constraints),fmt(a[0].Tx.Gas)];
  })
 }];
}
groups['key-recovery']=[
 {title:'(a) Committee n=10',head:['share k','복구 시간 (ms)'],data:[2,3,5,7,10].map(k=>{
  const a=rows(e8,r=>r.Case===`${k}-of-10`);return [String(k),fmt(median(a.map(r=>r.TotalMS/r.InnerIterations)),3)];})},
 {title:'(b) share k=2',head:['Committee n','복구 시간 (ms)'],data:[3,5,10,20].map(n=>{
  const a=rows(e8,r=>r.Case===`2-of-${n}`);return [String(n),fmt(median(a.map(r=>r.TotalMS/r.InnerIterations)),3)];})}
];
function texTable(t){
 const header=t.auditHeader
  ? '\\textbf{Event} & \\multicolumn{2}{c}{\\textbf{Audit}} & \\textbf{Overhead} \\\\\n\\cmidrule(lr){2-3}\n & \\textbf{Disabled} & \\textbf{Enabled} & \\\\\n'
  : t.head.map(h=>'\\textbf{'+h+'}').join(' & ')+' \\\\\n';
 return '{\\small\\setlength{\\tabcolsep}{7pt}\\renewcommand{\\arraystretch}{1.12}\n\\textbf{'+t.title+'}\\par\\smallskip\n\\begin{tabular}{@{}l'+'r'.repeat(t.head.length-1)+'@{}}\\toprule\n'+header+'\\midrule\n'+t.data.map(r=>r.map((v,i)=>['증가량','Overhead'].includes(t.head[i])?'\\textbf{'+v+'}':v).join(' & ')+' \\\\').join('\n')+'\n\\bottomrule\\end{tabular}}\n';
}
for(const [name,tables]of Object.entries(groups)){
 const tex='\\par\\medskip\\noindent\n'+(tables.length===2?tables.map(t=>'\\begin{minipage}[t]{.49\\linewidth}\\centering\n'+texTable(t)+'\\end{minipage}').join('\\hfill\n'):'\\begin{center}\n'+texTable(tables[0])+'\\end{center}')+'\n\\par\\medskip\n';
 fs.writeFileSync(path.join(out,name+'.tex'),tex);
 const md=tables.map(t=>'**'+t.title+'**\n\n| '+t.head.join(' | ')+' |\n|---|'+('---:|'.repeat(t.head.length-1))+'\n'+t.data.map(r=>'| '+r.join(' | ')+' |').join('\n')+'\n').join('\n');
 fs.writeFileSync(path.join(out,name+'.md'),md);
}
fs.writeFileSync(path.join(out,'values.json'),JSON.stringify(groups,null,2)+'\n');
