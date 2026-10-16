import json, statistics, csv
from pathlib import Path
ROOT = Path(__file__).resolve().parent
def stats(xs):
    return {'n':len(xs),'median':statistics.median(xs),'min':min(xs),'max':max(xs),'mean':statistics.mean(xs)}
runs=[json.loads((ROOT/f'raw/native-run-{i:02d}.json').read_text()) for i in range(1,4)]
rows=[r for j in runs for r in j['rows'] if not r['Warmup']]
assert len(rows)==90
arms={}
for arm in ['U','D-sequential','D-parallel-2']:
    rr=[r for r in rows if r['Arm']==arm];assert len(rr)==30
    arms[arm]={'proveMS':stats([r['ProveMS'] for r in rr]),'verifyMS':stats([r['VerifyMS'] for r in rr]),'runs':{}}
    for i in range(1,4):
        rs=[r for r in rr if r['Run']==i];assert len(rs)==10
        arms[arm]['runs'][i]={'proveMS':stats([r['ProveMS'] for r in rs]),'verifyMS':stats([r['VerifyMS'] for r in rs])}
    for k in ['NativeProofBytes','SolidityProofBytes','PublicInputBytes','CanonicalNativeBytes']:
        ss={r[k] for r in rr};assert len(ss)==1;arms[arm][k]=ss.pop()
setup=json.loads((ROOT/'setup.json').read_text());keys={r['Name']:r for r in setup['rows']};metadata={}
for arm,names in {'U':['U'],'D':['T','A']}.items():
    metadata[arm]={'uniqueProvingKeyBytes':sum(keys[n]['Bytes']['proving.key'] for n in names),'constraintsSum':sum(keys[n]['Constraints'] for n in names),'paddedDomainSum':sum(keys[n]['Domain'] for n in names),'uniqueCircuits':[keys[n] for n in names]}
def reduction(old,new):return 100*(old-new)/old
cmp={
 'Dseq_to_U_prove_reduction_percent':reduction(arms['D-sequential']['proveMS']['median'],arms['U']['proveMS']['median']),
 'Dpar_to_U_prove_reduction_percent':reduction(arms['D-parallel-2']['proveMS']['median'],arms['U']['proveMS']['median']),
 'Dseq_to_U_verify_reduction_percent':reduction(arms['D-sequential']['verifyMS']['median'],arms['U']['verifyMS']['median']),
 'Dpar_to_U_verify_reduction_percent':reduction(arms['D-parallel-2']['verifyMS']['median'],arms['U']['verifyMS']['median']),
 'D_to_U_native_proof_reduction_percent':reduction(arms['D-sequential']['NativeProofBytes'],arms['U']['NativeProofBytes']),
 'D_to_U_solidity_proof_reduction_percent':reduction(arms['D-sequential']['SolidityProofBytes'],arms['U']['SolidityProofBytes']),
 'D_to_U_canonical_native_reduction_percent':reduction(arms['D-sequential']['CanonicalNativeBytes'],arms['U']['CanonicalNativeBytes']),
 'D_to_U_unique_PK_reduction_percent':reduction(metadata['D']['uniqueProvingKeyBytes'],metadata['U']['uniqueProvingKeyBytes']),
 'D_to_U_constraints_reduction_percent':reduction(metadata['D']['constraintsSum'],metadata['U']['constraintsSum'])
}
evm=None
if (ROOT/'raw/evm.json').exists():
    er=json.loads((ROOT/'raw/evm.json').read_text())['rows'];assert len(er)==60;evm={}
    for arm in ['U','D']:
        rr=[r for r in er if r['arm']==arm];assert len(rr)==30
        assert all(r['returnedTrue'] and r['floorNotBinding'] and r['receipt']['status']=='0x1' for r in rr)
        sizes={r['canonicalABIBytes'] for r in rr};assert len(sizes)==1
        evm[arm]={'verificationExecutionGas':stats([r['verificationExecutionGas'] for r in rr]),'canonicalABIBytes':sizes.pop(),'runs':{i:stats([r['verificationExecutionGas'] for r in rr if r['run']==i]) for i in range(1,4)}}
    cmp['D_to_U_gas_reduction_percent']=reduction(evm['D']['verificationExecutionGas']['median'],evm['U']['verificationExecutionGas']['median'])
    cmp['D_to_U_ABI_reduction_percent']=reduction(evm['D']['canonicalABIBytes'],evm['U']['canonicalABIBytes'])
summary={'environment':runs[0]['environment'],'official_native_rows':90,'native':arms,'circuitMetadata':metadata,'comparisons':cmp,'evm':evm}
(ROOT/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
with (ROOT/'summary.csv').open('w') as f:
    w=csv.writer(f);w.writerow(['arm','prove_median_ms','prove_min_ms','prove_max_ms','verify_median_ms','verify_min_ms','verify_max_ms','native_proof_B','solidity_proof_B','canonical_native_B','canonical_ABI_B','unique_PK_B','verification_execution_gas'])
    for arm,v in arms.items():
        key=arm[0];gas=evm[key]['verificationExecutionGas']['median'] if evm else '';abi=evm[key]['canonicalABIBytes'] if evm else ''
        w.writerow([arm,v['proveMS']['median'],v['proveMS']['min'],v['proveMS']['max'],v['verifyMS']['median'],v['verifyMS']['min'],v['verifyMS']['max'],v['NativeProofBytes'],v['SolidityProofBytes'],v['CanonicalNativeBytes'],abi,metadata[key]['uniqueProvingKeyBytes'],gas])
print(json.dumps({'comparisons':cmp,'prove':{a:v['proveMS'] for a,v in arms.items()},'verify':{a:v['verifyMS'] for a,v in arms.items()},'evm':evm},indent=2))
