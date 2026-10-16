// Reaggregate the recorded official numeric observations; no experiment execution.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const assert = require('assert/strict');
const dir = path.resolve(__dirname, '../paper-data');
const source = 'runs/2026-10-16-run01/measurements.json';
const bytes = fs.readFileSync(path.join(dir, source));
const m = JSON.parse(bytes);
const manifest = JSON.parse(fs.readFileSync(path.join(dir, 'runs/2026-10-16-run01/source-manifest.json')));
assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), manifest.normalizedMeasurementSHA256);
const names = ['Entry','Transfer','Proceed','Recall','Merge','Split','Process','Exit','Issue'];
function stats(xs, n) {
  assert.equal(xs.length, n); assert(xs.every(Number.isFinite));
  const a = [...xs].sort((x,y)=>x-y), k=Math.floor(n/2);
  return {median:n%2?a[k]:(a[k-1]+a[k])/2, min:a[0], max:a[n-1], n};
}
function fixed(xs) { assert(xs.length && xs.every(x=>x===xs[0])); return xs[0]; }
function official(rows) { assert.deepEqual(rows.map(x=>x.run).sort((a,b)=>a-b), [1,2,3,4,5]); return rows; }
const quote = x => /[,"\n]/.test(String(x)) ? '"'+String(x).replaceAll('"','""')+'"' : String(x);
function csv(name, header, rows) {
  fs.writeFileSync(path.join(dir,name+'.csv'), [header,...rows].map(r=>r.map(x=>quote(x??'')).join(',')).join('\n')+'\n');
}
const events = names.map(event=>{
  const on=official(m.events.filter(x=>x.event===event&&x.variant==='audit'));
  const off=official(m.events.filter(x=>x.event===event&&x.variant==='plain'));
  return {event, constraints:fixed(on.map(x=>x.constraints)), plainConstraints:fixed(off.map(x=>x.constraints)), gas:stats(on.map(x=>x.gas),5), plainGas:stats(off.map(x=>x.gas),5), e2eMS:stats(on.map(x=>x.e2eMS),5)};
});
const by=Object.fromEntries(events.map(x=>[x.event,x]));
csv('events',['event','constraints','gas_median','gas_min','gas_max','e2e_ms_median','e2e_ms_min','e2e_ms_max'],events.map(x=>[x.event,x.constraints,x.gas.median,x.gas.min,x.gas.max,x.e2eMS.median,x.e2eMS.min,x.e2eMS.max]));
csv('audit-overhead',['event','constraints_off','constraints_on','gas_off','gas_on'],events.map(x=>[x.event,x.plainConstraints,x.constraints,x.plainGas.median,x.gas.median]));
const processRows=[[2,1],[3,1],[4,1],[2,2],[2,3]].map(([inputs,outputs])=>{
  const get=variant=>{const rows=m.process.filter(x=>x.inputs===inputs&&x.outputs===outputs&&x.variant===variant);assert.equal(rows.length,1);assert.equal(rows[0].run,1);return rows[0];};
  const a=get('audit'), b=get('plain');return {inputs,outputs,off:b,on:a};
});
csv('process',['inputs_outputs','constraints_off','constraints_on','gas_off','gas_on'],processRows.map(x=>[`${x.inputs}-${x.outputs}`,x.off.constraints,x.on.constraints,x.off.gas,x.on.gas]));
const direct=stats(official(m.routes.filter(x=>x.route==='direct')).map(x=>x.gas),5);
assert.equal(m.shared.length,1);
const routes=[
  {route:'direct',audit:'no',transactions:1,gas:direct.median,min:direct.min,max:direct.max,aggregation:'median of five direct transactions',samples:'5'},
  {route:'shared',audit:'yes',transactions:1,gas:m.shared[0].gas,min:m.shared[0].gas,max:m.shared[0].gas,aggregation:'one official observation',samples:'1'},
  {route:'proceed',audit:'yes',transactions:2,gas:by.Transfer.gas.median+by.Proceed.gas.median,aggregation:'sum of audited Transfer and Proceed medians',samples:'5+5'},
  {route:'recall',audit:'yes',transactions:2,gas:by.Transfer.gas.median+by.Recall.gas.median,aggregation:'sum of audited Transfer and Recall medians',samples:'5+5'}
];
csv('transfer-routes',['route','transactions','gas_median','gas_min','gas_max','aggregation','samples'],routes.map(x=>[x.route,x.transactions,x.gas,x.min,x.max,x.aggregation,x.samples]));
const labels={direct:'zkTransfer',shared:'SharedSecret',proceed:'Transfer->Proceed',recall:'Transfer->Recall'};
csv('transfer-comparison',['route','audit_record','gas','aggregation'],routes.map(x=>[labels[x.route],x.audit,x.gas,x.aggregation]));
function merge(part, arms) {
  const b=m[part];
  return arms.map(arm=>{
    const rows=b.native.filter(x=>x.Arm===arm);assert.equal(rows.length,30);
    for(const run of [1,2,3]) assert.equal(rows.filter(x=>x.Run===run).length,10);
    const evmArm=arm.startsWith('D-')?'D':arm.startsWith('A-')?'A':arm;
    const er=b.evm.filter(x=>x.arm===evmArm);assert.equal(er.length,30);
    return {arm,prove:stats(rows.map(x=>x.ProveMS),30),verify:stats(rows.map(x=>x.VerifyMS),30),abi:fixed(er.map(x=>x.canonicalABIBytes)),gas:stats(er.map(x=>x.verificationExecutionGas),30),keyBytes:b.keyBytes[evmArm],nativeProofBytes:fixed(rows.map(x=>x.NativeProofBytes)),solidityProofBytes:fixed(rows.map(x=>x.SolidityProofBytes)),publicInputBytes:fixed(rows.map(x=>x.PublicInputBytes)),canonicalNativeBytes:fixed(rows.map(x=>x.CanonicalNativeBytes))};
  });
}
const same=merge('same',['U','D-sequential','D-parallel-2']);
const three=merge('three',['A-sequential','A-parallel-2','B','C']);
const mergeHeader=['arm','samples','prove_ms_median','prove_ms_min','prove_ms_max','verify_ms_median','verify_ms_min','verify_ms_max','canonical_abi_bytes','verification_execution_gas','unique_proving_key_bytes','native_proof_bytes','solidity_proof_bytes','public_input_bytes','canonical_native_bytes'];
for(const [name,rows] of [['same-merge',same],['three-merge',three]]) csv(name,mergeHeader,rows.map(x=>[x.arm,30,x.prove.median,x.prove.min,x.prove.max,x.verify.median,x.verify.min,x.verify.max,x.abi,x.gas.median,x.keyBytes,x.nativeProofBytes,x.solidityProofBytes,x.publicInputBytes,x.canonicalNativeBytes]));
const aggregate={edition:m.edition,setting:m.setting,events,process:processRows,routes,same,three};
fs.writeFileSync(path.join(dir,'current-summary.json'),JSON.stringify(aggregate,null,2)+'\n');
const f=(x,d=0)=>x.toLocaleString('en-US',{minimumFractionDigits:d,maximumFractionDigits:d});
let md='# 현재 원고의 수치표: 2026-10-16 run01\n\n';
md+='공식 측정행과 재집계 방법은 [README](README.md)에 연결한다. 값은 실제 재실행의 기록이며 새 실험을 수행한 결과가 아니다.\n\n';
function table(title,head,rows){md+='## '+title+'\n\n| '+head.join(' | ')+' |\n| '+head.map(()=> '---').join(' | ')+' |\n'+rows.map(x=>'| '+x.join(' | ')+' |').join('\n')+'\n\n';}
table('거래별 비용',['거래','제약 수','gas','E2E 중앙값(ms)','최소–최대(ms)'],events.map(x=>[x.event,f(x.constraints),f(x.gas.median),f(x.e2eMS.median,3),f(x.e2eMS.min,3)+'–'+f(x.e2eMS.max,3)]));
table('감사 추가 비용',['거래','제약 증가','gas 증가','gas 증가율'],events.map(x=>[x.event,f(x.constraints-x.plainConstraints),f(x.gas.median-x.plainGas.median),f(100*(x.gas.median-x.plainGas.median)/x.plainGas.median,2)+'%']));
table('가공 형상',['입력','출력','제약 수','gas'],processRows.map(x=>[x.inputs,x.outputs,f(x.on.constraints),f(x.on.gas)]));
table('전달 방식',['방식','gas','집계'],routes.map(x=>[labels[x.route],f(x.gas),x.aggregation]));
md+='두 단계는 각각의 감사 포함 거래 중앙값을 더한 값이다. 별도 전달 경로 실험의 중앙값과 섞지 않는다.\n\n';
for(const [title,rows] of [['동일 기능 Merge',same],['A/B/C 보조 비교',three]]) table(title,['구성','생성(ms)','로컬 검증(ms)','ABI(B)','무상태 검증 실행 gas'],rows.map(x=>[x.arm,f(x.prove.median,3),f(x.verify.median,3),f(x.abi),f(x.gas.median)]));
fs.writeFileSync(path.join(dir,'current-results.md'),md);
const prior=JSON.parse(fs.readFileSync(path.join(dir,'archive/b11b6e7/provenance.json')));
fs.writeFileSync(path.join(dir,'provenance.json'),JSON.stringify({edition:m.edition,setting:m.setting,environment:m.environment,numericObservations:source,numericObservationSHA256:manifest.normalizedMeasurementSHA256,originalRawHashes:'runs/2026-10-16-run01/source-manifest.json',aggregationCode:'../src/build-current-paper-data.cjs',preservedLegacy:{description:'Unchanged key-recovery and proof-free tracing/freezing records retained by the current manuscript',original:prior}},null,2)+'\n');
console.log(JSON.stringify({edition:m.edition,events:events.length,process:processRows.length,transfer:routes.map(x=>[x.route,x.gas]),same: same.map(x=>[x.arm,x.prove.median]),three:three.map(x=>[x.arm,x.prove.median])}));
