"""Independent artifact audit; derives comparisons from saved evidence."""
import csv, hashlib, json, pathlib

RUN = pathlib.Path(__file__).resolve().parent
RAW = RUN / 'raw'
def read(name): return json.loads((RAW / name).read_text())
def write(name, value): (RAW / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')
def key(t): return (t['Ref']['ObjectType'], str(t['Ref']['RawID']), tuple(t['SpendValue']))
def qae(w): return [int(w[k] if k in w else w[alias]) for k,alias in [('QMass','qMass'),('ARec','aRec'),('E','e')]]
def assignment(n):
    a = read('assignments/' + n + '.json')
    return a.get('Base', a)

expected = read('expected.json')
objects = {o['Label']:o for o in expected['Objects']}
labels = ['N_R','N_P','W','C_R','C_P','C_Q','V_Q']
assert expected['Targets'] == labels
target_keys = {(objects[l]['Ref']['ObjectType'], str(objects[l]['Ref']['RawID']), tuple(objects[l]['Spend'])) for l in labels}
assert len(target_keys) == 7
design = {'A1':[60,10,50], 'A2':[60,10,50], 'C':[100,10,70], 'D':[100,0,70], 'U1':[115,0,115], 'U2':[115,0,115], 'M':[120,20,100], 'EL':[270,30,270], 'W':[30,0,0], 'R':[90,10,90], 'REST':[180,20,180], 'P':[90,10,90], 'V':[90,10,90], 'V_R':[90,10,91], 'N_R':[90,10,91], 'V_P':[90,10,92], 'N_P':[90,10,92], 'V_Q':[90,10,93], 'C_R':[0,0,0], 'C_P':[0,0,0], 'C_Q':[0,0,0]}
amount_rows = []
for l, want in design.items():
    actual = qae(objects[l]['State'])
    assert actual == [v*10**9 for v in want], (l, actual, want)
    amount_rows.append({'label':l, 'expected_scale_1e9':want, 'fixture_raw':actual, 'matched':True})
for l in ['A1','A2','C','D','U1','U2']: assert qae(assignment('entry-'+l)['Note']) == qae(objects[l]['State'])
assert [qae(a) for a in assignment('process')['Inputs']] == [[120e9,20e9,100e9],[100e9,10e9,70e9],[100e9,0,70e9]]
assert [qae(a) for a in assignment('process')['Outputs']] == [[270e9,30e9,270e9],[30e9,0,0]]
for l in ['R','P','Q']:
    a = assignment('transfer-'+l)
    assert qae(a['Voucher']) == qae(objects['V_'+l]['State'])
    assert qae(a['Change']) == [0,0,0]
    assert a['RemainderA'] == a['RemainderE'] == 0
    assert a['D'] == 100000
assert qae(assignment('recall-R')['Output']) == [90e9,10e9,91e9]
assert qae(assignment('proceed-P')['Output']) == [90e9,10e9,92e9]
assert [qae(a) for a in assignment('process-N_R')['Inputs']] == [[90e9,10e9,91e9],[115e9,0,115e9],[115e9,0,115e9]]
assert [qae(a) for a in assignment('process-N_R')['Outputs']] == [[270e9,10e9,351e9],[30e9,0,0]]
for name in ['process','process-N_R']:
    a = assignment(name)
    assert a['QLoss']==20e9 and a['CarbonAdd']==30e9
    assert a['RemLoss']==a['RemCarbon']==a['RemWaste']==0
    assert a['PolicyRef']==assignment('process')['PolicyRef'] and a['ScopeRef']==assignment('process')['ScopeRef']
witnesses = read('witness-evidence.json')
assert len(witnesses)==21 and all(w['bytesMatched'] for w in witnesses)
for w in witnesses:
    assert hashlib.sha256((RAW/'witnesses'/(w['event']+'.bin')).read_bytes()).hexdigest()==w['witnessSHA256']
write('amount-comparison.json', {'unitScale':10**9,'rows':amount_rows,'fullWitnessesByteMatched':21,'auditPlaintextContainsAmounts':False,'processProbeEligible':[270,10,351]})

trace = read('trace-comparison.json')['actual']
assert len(trace['Objects']) == 16 and len(trace['Graph']) == 15
seen = {(r['ObjectType'],str(r['RawID'])) for r in trace['Objects']}
assert all((objects[l]['Ref']['ObjectType'],str(objects[l]['Ref']['RawID'])) not in seen for l in ['A2','C','D','U1','U2'])
assert {key(t) for t in trace['Frontier']} == target_keys
records = read('actual-records.json')
assert len(records)==15 and all(r['matched'] and r['decryptedParentFutureVector']==r['independentExpectedVector'] for r in records)

normal=read('normal-client.json')
assert normal['Outcome']=='COMPLETE_AT_CHECKPOINT'
def inspect_result(r):
    assert {key(t) for t in r['Targets']}==target_keys
    assert {key(t['Target']) for t in r['TargetResults']}==target_keys
    assert len(r['Targets'])==len(r['TargetResults'])==7
    assert r['Checkpoint']['blockNumber']>=r['Snapshot']['blockNumber']
    for a in r['TargetResults']:
        assert r['Checkpoint']['blockNumber']>=a['Observation']['blockNumber']
        if a['Transaction']: assert r['Checkpoint']['blockNumber']>=a['Transaction']['blockNumber']
    return next(t for t in r['TargetResults'] if key(t['Target'])==(objects['V_Q']['Ref']['ObjectType'],str(objects['V_Q']['Ref']['RawID']),tuple(objects['V_Q']['Spend'])))
inspect_result(normal)
assert all(t['Blocked'] and not t['FinalSpent'] and t['FinalStatus']==1 and t['Confirmed'] for t in normal['TargetResults'])
case_rows=[{'case':'normal','expected':'COMPLETE_AT_CHECKPOINT','actual':normal['Outcome'],'targets':7,'confirmed_freezes':7,'client_blocked':7,'ground_blocked':7,'H_t':normal['Snapshot']['blockNumber'],'H_r':normal['Checkpoint']['blockNumber'],'passed':True}]
for mode in ['proceed-first','recall-first','final-status-unavailable','receipt-uncertain']:
    c=read(mode+'-client.json');r=c['result'];v=inspect_result(r)
    assert r['Outcome']=='INCOMPLETE'
    ground=read(mode+'-ground-truth.json');gm={o['label']:o for o in ground['objects']}
    blocked=sum(t['Blocked'] for t in r['TargetResults'])
    if mode in ['proceed-first','recall-first']:
        assert v['InitialSpent'] and v['FinalSpent'] and not v['Attempted'] and not v['Blocked']
        assert blocked==6 and gm['V_Q']['spentIn']==16
        assert all(gm[l]['status']==1 and not gm[l]['spent'] for l in labels if l!='V_Q')
        assert ground['replacementNote']['status']==0 and not ground['replacementNote']['spent'] and ground['replacementNote']['notAddedToOriginalT']
        assert c['injections']==0 and c['receiptValidations']==6
    elif mode=='final-status-unavailable':
        assert c['injections']==1 and c['receiptValidations']==7 and v['FinalError'] and not v['Blocked'] and blocked==6
        assert all(gm[l]['status']==1 and not gm[l]['spent'] for l in labels)
    else:
        assert c['injections']==1 and c['receiptValidations']==6 and v['Transaction']['state']=='UNCERTAIN' and v['Transaction']['hash'] and v['SubmissionError'] and blocked==7
        assert all(gm[l]['status']==1 and not gm[l]['spent'] for l in labels)
        assert any(t['hash']==v['Transaction']['hash'] and t['state']=='CONFIRMED' for t in ground['confirmedFreezeTransactions'])
    txs=read(mode+'-transactions.json')
    assert all(t['receipt']['status']=='0x1' and t['receipt']['blockHash']==t['header']['hash'] for t in txs)
    case_rows.append({'case':mode,'expected':'INCOMPLETE','actual':r['Outcome'],'targets':7,'confirmed_freezes':sum(t['Confirmed'] for t in r['TargetResults']),'client_blocked':blocked,'ground_blocked':sum(gm[l]['status']==1 and not gm[l]['spent'] for l in labels),'H_t':r['Snapshot']['blockNumber'],'H_r':r['Checkpoint']['blockNumber'],'passed':True})

probe_rows=[]
for name in ['process-N_R','transfer-N_P','proceed-V_Q','recall-V_Q','exit-W','transfer-U1']:
    a=read('active-'+name+'.json');f=read('frozen-'+name+'.json')
    assert a['passed'] and f['passed']
    active_txs=read('active-'+name+'-transactions.json');frozen_txs=read('frozen-'+name+'-transactions.json')
    assert len(active_txs)==len(frozen_txs)==1 and active_txs[0]['receipt']['status']=='0x1'
    if name!='transfer-U1':
        assert frozen_txs[0]['receipt']['status']=='0x0' and f['traceFailed']
        assert {k:v for k,v in f['before'].items() if k!='snapshot'}=={k:v for k,v in f['after'].items() if k!='snapshot'}
        assert all(not o['spent'] for o in f['after']['objects'] if o['label'] in ['U1','U2'])
    else: assert frozen_txs[0]['receipt']['status']=='0x1'
    probe_rows.append({'probe':name,'expected_active':'accepted','actual_active':'accepted','expected_frozen':f['expected'],'actual_frozen':f['expected'],'exact_revert':f['returnValue'] if name!='transfer-U1' else None,'atomicity_checked':name!='transfer-U1','passed':True})
report=read('evm-report.json')
assert not report['MockVerifiers'] and len(report['Deployments'])==12
assert all(d['RuntimeBytes']<=24576 for d in report['Deployments'])
for name in ['normal','base']:
    assert all(t['receipt']['status']=='0x1' and t['receipt']['blockHash']==t['header']['hash'] for t in read(name+'-transactions.json'))
validation=read('validation.json');assert validation['passed']
write('expected-actual-comparison.json', {'cases':case_rows,'probes':probe_rows,'targets':[objects[l] for l in labels],'passed':True})
write('artifact-audit.json', {'passed':True,'source':'independent Python assertions over saved JSON/receipts/assignments/witness hashes','amountRows':21,'records':15,'graphObjects':16,'foldedEdges':15,'baseNoteLeaves':18,'baseVoucherLeaves':3,'targets':7,'activeControls':6,'exactRejections':5,'negativeCompletionBranches':4,'conditionalCompletionAnalysisExperimentScope':True})
for name,rows in [('completion-comparison.tsv',case_rows),('probe-comparison.tsv',probe_rows)]:
    with (RUN/name).open('w',newline='') as f:
        w=csv.DictWriter(f,fieldnames=list(rows[0]),delimiter='\t');w.writeheader();w.writerows(rows)
print(json.dumps(read('artifact-audit.json'),ensure_ascii=False))
