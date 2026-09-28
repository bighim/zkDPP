// One-time transcription/update from preserved experiment data. Never overwrite edited TeX.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const dir=__dirname,root=path.resolve(dir,'../../..');
const read=p=>JSON.parse(fs.readFileSync(path.join(root,p),'utf8'));
const R=String.raw,med=a=>a.slice().sort((a,b)=>a-b)[Math.floor(a.length/2)];
const fmt=n=>Number(n).toLocaleString('en-US',{maximumFractionDigits:0});
const texEsc=s=>s.replaceAll('_',String.raw`\_`);
const files={};
const table=(n,title,heads,rows,align)=>R`\begin{center}\small\setlength{\tabcolsep}{6pt}`+'\n'+`\\textbf{表 ${n}. ${title}}\\par\\smallskip\n`.replace('表','표')+`\\begin{tabular}{@{}${align||'l'+'r'.repeat(heads.length-1)}@{}}\\toprule\n`+heads.map(h=>`\\textbf{${h}}`).join(' & ')+R` \\ \midrule`+'\n'+rows.map(r=>r.join(' & ')+R` \\`).join('\n')+'\n'+R`\bottomrule\end{tabular}\end{center}`+'\n';
const note=s=>R`{\small `+s+R`}\par`+'\n';
const graph=(options,series,legend='')=>R`\begin{tikzpicture}\begin{axis}[`+R`width=7cm,height=5.6cm,grid=major,major grid style={gray!18},scaled ticks=false,tick label style={font=\small},label style={font=\small},`+options+']\n'+series.map((p,i)=>R`\addplot+[thick,mark options={fill=white},color=`+(i?'orange!80!black':'blue!65!black')+',mark='+(i?'square*':'*')+'] coordinates {'+p.map(x=>'('+x.join(',')+')').join(' ')+R`};`).join('\n')+'\n'+(legend?R`\legend{`+legend+'}\n':'')+R`\end{axis}\end{tikzpicture}`;
const paired=(a,b)=>R`\begin{center}`+a+R`\hspace{0.25cm}`+b+R`\end{center}`+'\n';
const exp=read('output/experiments/experiment-1.json');
if(exp.status!=='complete'||exp.repetitions!==5||!exp.rows.every(r=>r.Matched))throw Error('experiment-1 validation');
const selection=[['Entry','01'],['Transfer','03'],['Proceed','06'],['Recall','04'],['Merge','09'],['Split','12'],['Process','11'],['Exit','14'],['Issue','13']];
const basic=selection.map(([name,id])=>{const rows=exp.rows.filter(r=>!r.Warmup&&r.Case==='exp1-lifecycle-'+id);if(rows.length!==5)throw Error(name);return [name,rows[0].Constraints,med(rows.map(r=>r.Tx.Gas)),med(rows.map(r=>r.E2EMS))]});
const e3=read('output/experiments/experiment-3.json');
const csv=fs.readFileSync(path.join(root,'docs/paper-data/audit-overhead.csv'),'utf8').trim().split('\n').slice(1).map(l=>l.split(','));
const tracing=read('output/experiments/experiment-4.json');
const trace=[10,100,1000].map(n=>[n,...['forward','backward','freeze'].map(op=>med(tracing.rows.filter(r=>!r.Warmup&&r.Case==='chain-'+n&&r.Operation===op).map(r=>r.TotalMS))/1000)]);
const keys=read('output/paper-supplement/prepared-recovery/result.json');
const recovery=keys.rows.filter(r=>!r.warmup&&r.n===10).sort((a,b)=>a.k-b.k);
if(recovery.length!==5||recovery.some(r=>r.fullCommitteeValidationsInTimer!==0))throw Error('prepared recovery');
const shared=read('output/paper-supplement/shared-transfer/verification.json');if(shared.status!=='verified'||shared.gas!==2724242)throw Error('shared transfer');
const proc=read('output/paper-supplement/process-independent.json').rows.filter(r=>r.Variant==='audit');
const shapes=[[2,1],[3,1],[4,1],[2,2],[2,3]];
const process=shapes.map(([m,n])=>{const r=proc.find(r=>r.Profile===`process-${m}-${n}-audit`);const c=read(`output/paper-supplement/process-${m}-${n}-audit.json`);for(const [side,want]of [['inputs',[320,30,231]],['outputs',[300,30,261]]]){const got=['qMass','aRec','e'].map(k=>c[side].reduce((s,x)=>s+BigInt(x.State[k]),0n));if(got.some((x,i)=>x!==BigInt(want[i])*1000000000n))throw Error('sum '+side)}return [m,n,r.Constraints,r.Tx.Gas]});
const hf=read('output/m1-hotfix-audit.json');const freeze=hf.Audits.find(x=>x.group==='audit'&&x.result)?.result;
if(hf.MockVerifiers||freeze?.Outcome!=='COMPLETE_AT_CHECKPOINT'||freeze.Targets.length!==3)throw Error('HF');
const scope=R`\textbf{資料 범위.} `.replace('資料','자료')+R`동료의 평가 보고서를 TeX로 재구성하고 보존된 Raw와 후속 검증 결과를 반영했다.
새로운 성능 실험은 실행하지 않았다.
M1-HF, 기본 실험, 보충 실험은 서로 다른 실행이므로 수치와 통계 조건을 구분한다.
논문 초안의 두 키 구성 전체를 구현했다는 의미는 아니다.\par`;
files['01.Methods.tex']=R`\section{실험 평가}
`+scope+R`
\subsection{실험 환경 및 방법}
기본 환경은 표 1과 같다. 별도 설명이 없으면 Audit을 포함한 zkDPP Event를 다룬다.
`+table(1,'공통 소프트웨어 설정',['항목','설정'],[['언어','Go 1.25.7'],['Circuit','gnark 0.15.0 / gnark-crypto 0.20.1'],['증명계 / 곡선','PLONK-KZG / BLS12-381'],['감사 암호화','Jubjub / Poseidon2'],['병렬 설정','GOMAXPROCS=8'],['노드 / 컴파일러','Anvil 1.7.1 / Solidity 0.8.30'],['EVM / Chain ID','Prague / 31337'],['Transaction gas 한도','30,000,000']],'ll')+R`
\textbf{측정 단위와 범위}
\begin{itemize}
\item Constraints는 컴파일한 Circuit의 제약 수다. Gas는 성공 receipt의 gasUsed다.
\item Event E2E는 필요한 입력·path·Policy를 확보한 뒤 witness 준비부터 성공 receipt 확인까지다. 로컬 Proof Verify와 결과 대조는 타이머 밖이다.
\item Tracing은 고유 방문 AuditRecord 수를, Freeze는 동결 대상 수를 작업량으로 사용한다. 두 작업의 시간을 더해 AuditAndFreeze E2E라고 표시하지 않는다.
\end{itemize}
\textbf{반복 조건}
\begin{itemize}
\item 기본 Event·Audit 비교·Tracing·Freeze: warm-up 1회, 공식 5회, median.
\item Process 보충 실험·SharedSecret 직접 전달: 조건별 공식 1회, warm-up 없음.
\item 사전 검증 후 키 복구: warm-up 1묶음, 공식 1묶음. 묶음마다 1,000회 복구하며, 표에는 공식 묶음의 1회당 평균을 사용한다.
\end{itemize}
\textbf{환경의 적용 범위}
보충 Process에는 macOS 26.3, 호스트 논리 CPU 10개·메모리 32 GiB, Anvil 2 CPU·4 GiB가 기록되어 있다.
이를 기본 실험 전체의 측정 환경으로 소급하여 대입하지 않는다.
기본 실험의 하드웨어·OS는 해당 실행 근거를 별도로 명시해야 한다.
`+note('근거: [D1]~[D6]. 시간 통계가 다른 실행을 하나의 반복 집합으로 합치지 않았다.');
files['02.Functional-Scenarios.tex']=R`\subsection{시스템 동작 및 성능 평가}
\subsubsection*{기본 성능을 측정한 전체 공급망 시나리오}
표 3의 비용은 Entry에서 Claim 발급·Exit까지 연결한 실험 1의 대표 Event에서 측정했다.
각 회차는 다음 14개 Event를 실행했다.
\begin{enumerate}
\item 같은 소유자의 Note 두 개를 Entry한다. 각 State는 $(160,15,115)$다.
\item 첫 Note를 전량 Transfer하며 운송 탄소 1을 더한다. Recall한 뒤 다시 Transfer하고 Receiver가 Proceed한다.
\item 두 번째 Note도 Transfer·Proceed한다. Receiver가 두 Note를 Merge해 $(320,30,231)$을 만든다.
\item Split으로 두 입력을 준비하고 Process $2\to1$로 $(300,30,261)$을 만든다.
\item 다시 Split한 두 제품 중 하나는 Issue, 다른 하나는 Exit한다.
\end{enumerate}
수치는 $10^9$ scale 정수다. 공식 5회와 warm-up을 포함한 84개 Event의 Raw에 결과 대조 성공이 기록되어 있다.
선택한 대표 위치는 Entry 1, Transfer 3, Proceed 6, Recall 4, Merge 9, Split 12, Process 11, Exit 14, Issue 13이다.

\subsubsection*{추적·동결을 확인한 별도 시나리오}
Entry $A\to(B,C)$, $B\to(D,E)$의 분기 이력은 기본 비용 시나리오와 별개다.
M1-HF에서는 실제 Proof로 해당 이력을 만들고 RPC로 $C,D,E$를 찾아 실제 동결 3건을 제출했다.
이 실행의 기준 블록은 64, 공통 확인 블록은 67이며 결과는 COMPLETE\_AT\_CHECKPOINT다.
`+table(2,'기능 검증의 근거를 실행별로 구분',['검증 대상','확인한 결과','근거'],[
['기본 공급망','14 Event 연결, 모든 결과 대조 성공','실험 1 [D1]'],
['작은 분기 이력','C·D·E, 동결 3건, 공통 시점 완료','M1-HF [D2]'],
['Claim·Exit 종단','Claim 2개·Exit 1개 종단 처리','M1-HF [D2]'],
['Backward','Claim에서 원자재 Entry 4개 복원','M1-HF [D2]'],
['경합·오류','소비 경합은 미완료, 잘못된 chain은 추적 실패','M1-HF·실험 4 [D2,D3]']
],'p{2.7cm}p{7cm}p{3.8cm}')+R`
이 결과는 과거 M1의 시험용 verifier 실행이 아니다. M1-HF Raw는 MockVerifiers=false이고, 후속 실험에도 실제 verifier·Proof·receipt가 보존되어 있다.
다만 M1-HF의 20 Event·Process $3\to2$와 기본 실험의 14 Event·Process $2\to1$은 서로 다른 시나리오다.
현재 논문 초안의 두 키 wrapping 방식은 이 한 단계 Master Key 구현과 별도로 대응시켜야 한다.
`;
files['03.Event-Costs.tex']=R`\subsubsection*{기본 거래의 비용}
State 성분은 세 개이고 기본 Process는 $2\to1$이다. 표의 E2E와 Gas는 각각 공식 5회의 median이다.
`+table(3,'Audit을 포함한 Event별 실행 비용',['Event','Constraints','Gas','E2E (ms)'],basic.map(r=>[r[0],fmt(r[1]),fmt(r[2]),r[3].toFixed(3)]))+R`
Process의 Constraints가 가장 크지만, Gas는 Transfer가 가장 크다. E2E의 최대값은 Split에서 관측됐다.
Constraints는 증명할 관계를, Gas는 Proof 검증과 원장 상태 변경을, E2E는 실제 실행 시간을 반영하므로 순위가 같을 필요는 없다.

Transfer는 입력 Note의 membership을 한 번 검증하고 Voucher와 Change Note를 등록한다.
별도 compile profile에서 Merkle Path 검증 9,825개, Audit 추가 관계 14,238개, State·객체 관계 17,994개, 나머지 commitment·nullifier·주소 계산 7,826개로 분리됐다.
이는 하나의 전체 Circuit에서 호출 stack에 귀속시킨 값이지 독립 gadget을 제거한 ablation은 아니다.

동일한 기존 Proof를 재실행한 Gas 3,577,414도 trace 합계와 일치했다.
Tree Hash 호출 1,422,404와 Tree 조회·저장 1,276,500이 큰 비중을 차지했다.
Tree 루프·분기 등의 일부 비용은 나머지 실행에 포함되므로 이 둘의 합을 전체 Tree 함수 비용이라고 단정하지 않는다.
`+note('기본 성능: [D1]. Constraints·Gas 분리 진단: [D7]. 진단 실행은 공식 E2E 반복에 추가하지 않았다.');
const bar=(col,ylab,scale)=>{let s=R`\begin{tikzpicture}\begin{axis}[width=7cm,height=5.7cm,ybar,bar width=4pt,ymin=0,grid=major,major grid style={gray!18},symbolic x coords={Entry,Transfer,Proceed,Recall,Merge,Split,Process,Exit,Issue},xtick=data,x tick label style={rotate=45,anchor=east,font=\scriptsize},ylabel={`+ylab+R`},label style={font=\small},legend style={at={(0.5,-0.35)},anchor=north,legend columns=2,draw=none,font=\small},legend image code/.code={\draw[##1] (0cm,-0.08cm) rectangle (0.13cm,0.08cm);} ]`.replace('##1','#1');for(let i=0;i<2;i++){s+='\n'+R`\addplot[fill=`+(i?'orange!65':'blue!50')+'] coordinates {'+csv.map(r=>'('+r[0]+','+(+r[col+i]/scale)+')').join(' ')+R`};`;}return s+'\n'+R`\legend{Audit off,Audit on}\end{axis}\end{tikzpicture}`};
files['04.Audit-Overhead.tex']=R`\subsubsection*{Event別のAudit追加費用}`.replace('Event別のAudit追加費用','Event별 Audit 추가 비용')+'\n'+R`같은 Event 종류의 Audit off/on을 비교한다. $\Delta C=C_{on}-C_{off}$, $\Delta G=G_{on}-G_{off}$이며 증가율의 분모는 off다.`+'\n'+paired(bar(1,'Constraints (thousands)',1000),bar(3,'Gas (millions)',1000000))+note('그림 1. Event별 Audit 추가 비용. 두 패널의 단위가 다르다.')+table(4,'Audit 유무와 증가량',['Event',R`$C_{off}$`,R`$C_{on}$`,R`$\Delta C$`,R`$G_{off}$`,R`$G_{on}$`,R`$\Delta G$`],csv.map(r=>[r[0],fmt(r[1]),fmt(r[2]),fmt(+r[2]-r[1]),fmt(r[3]),fmt(r[4]),fmt(+r[4]-r[3])]))+R`
추가 Constraints는 11,533~14,238개, 추가 Gas는 130,705~356,287이다.
증가율은 각각 31.70~202.93\%, 9.69~54.67\%다.
기본 비용이 작은 Event는 같은 절대 증가량에도 상대 증가율이 커진다.

암호화 관계와 출력 소비값 계산, 추가 public input·calldata 처리, 감사·연결 자료 저장이 이 차이에 포함된다.
따라서 Audit on/off의 Gas 차이는 순수 SSTORE 차이가 아니다.
별도 Transfer 진단의 AuditRecord 조회·저장 294,700 Gas 역시 전체 추가 비용 356,146 Gas와 같은 항목이 아니다.
`+note('근거: 실험 1·3의 원자료와 기존 비교표 [D1], Transfer 진단 [D7].');
files['05.Tracing-and-Freeze.tex']=R`\subsubsection*{Tracing과 Freeze}
고유 방문 AuditRecord 수 $V$와 동결 대상 수 $F$를 구분한다.
아래 값은 실험 4의 연속 Split graph에서 얻은 공식 5회 median이다. graph 구성에는 실제 Proof를 사용했다.
`+paired(graph('xmode=log,ymode=log,xlabel={Unique visited AuditRecords},ylabel={Time (s)},ymin=0.05,ymax=100,legend style={at={(0.5,-0.25)},anchor=north,legend columns=2,draw=none,font=\\small}',[trace.map(r=>[r[0],r[1]]),trace.map(r=>[r[0],r[2]])],'Forward,Backward'),graph('xmode=log,ymode=log,xlabel={Freeze targets},ylabel={Time (s)},ymin=0.05,ymax=100',[trace.map(r=>[r[0],r[3]])]))+note('그림 2. Tracing과 Freeze. 선은 관측점을 연결하며 복잡도 증명을 뜻하지 않는다.')+table(5,'방문 기록 수·동결 대상 수에 따른 시간',['V 또는 F','Forward (s)','Backward (s)','Freeze (s)'],trace.map(r=>[fmt(r[0]),...r.slice(1).map(v=>v.toFixed(3))]))+R`
Tracing은 기록 조회·복호화·탐색을 포함하며, 사전 키 복구와 Snapshot 확보는 제외한다.
Freeze는 제출 전 상태 검사부터 transaction·receipt 및 공통 시점의 최종 상태 확인까지다.
대상마다 transaction 하나를 제출한다. 서로 다른 작업의 median을 합쳐 전체 감사 E2E라고 표시하지 않는다.

1,000개 기록의 Forward는 9.760초, Backward는 8.102초였고, 1,000개 대상 Freeze는 60.540초였다.
관측 증가율만으로 일반적인 선형성·점근 복잡도를 주장하지 않는다. graph 형태와 실제 복호화·RPC 수의 영향도 구분해야 한다.
한 Note의 Active→Frozen transaction은 별도 확인 case에서 49,434 Gas였다.

실험 4에는 이미 Frozen·Revoked인 대상, Snapshot 이후 소비된 대상, 잘못된 chain 조회의 correctness 결과도 기록되어 있다.
다만 시간표 하나만으로 모든 실패 순서나 두 키 방식의 통합 적합성을 검증했다고 해석하지 않는다.
`+note('근거: [D3]. Backward에서 최초 Entry는 방문하되 부모 복호화는 하지 않으므로 방문 수와 복호화 수가 다를 수 있다.');
files['06.Key-Recovery.tex']=R`\subsubsection*{사전 검증 후 로컬 Master Key 복구}
Committee 공개키·public share 자료는 사전에 검증해 보관한다.
타이머에는 받은 $k$개 share의 검증, Lagrange 보간, 복구한 키의 공개키 대조와 best-effort scalar 정리가 포함된다.
키 생성·DKG·통신·승인 대기는 제외한다.

전체 위원 수 $n=10$을 고정하고 $k$를 변화시켰다.
각 case는 warm-up 1,000회 후 공식 1,000회를 실행했으며, 공식 묶음의 전체 시간을 1,000으로 나눈 평균이다.
기존 5회 median과 섞지 않는다.
\begin{center}
`+graph('width=9cm,xlabel={Shares used (k)},ylabel={Local recovery time (ms)},xtick={2,3,5,7,10},ymin=0,ymax=0.8',[recovery.map(r=>[r.k,r.recoveryMS])])+R`\end{center}`+'\n'+note('그림 3. n=10 고정, k에 따른 로컬 복구 시간.')+table(6,'사용 share 수에 따른 로컬 복구',['k','시간 (ms)'],recovery.map(r=>[r.k,r.recoveryMS.toFixed(3)]))+R`
이 구현은 타이머 안에서 전체 $n$명의 자료를 다시 순회하지 않는다.
따라서 이전의 $n$ 증가 그래프는 본문에서 제외했다. $n=10$이라는 실험 조건은 유지한다.
$k$가 늘면 검증할 share와 보간 연산량이 증가하지만, 이 값만으로 모든 시스템의 복잡도나 성능을 일반화하지 않는다.

이 측정은 Shamir share로 한 단계 감사 Master Key를 복구하는 비용이다.
논문 초안에서 별도 감사 데이터 키를 threshold encryption으로 감싸는 절차 전체의 비용이 아니다.
해당 두 키 구성의 성능으로 사용하려면 별도의 구현 대응과 측정이 필요하다.
`+note('근거: prepared-recovery/result.json과 측정 경계 설명 [D4]. 각 case는 threshold에 맞는 유효한 share k개를 사용한다.');
files['07.Transfer-Tradeoff.tex']=R`\subsubsection*{전달 방식의 기능과 비용}
직접 전달은 Receiver Note와 Change Note를 한 transaction에서 만든다.
Voucher 경로는 Transfer 이후 Receiver의 Proceed 또는 Sender의 Recall로 해결된다.
정상 수령과 회수는 최종 소유자가 다르므로 경로를 구분한다.
`+table(7,'전달·회수 경로의 Gas',['경로','Audit','Gas','통계·값의 성격'],[
['직접 전달 기준선','없음','2,366,228','5회 median'],
['SharedSecret 직접 전달','있음','2,724,242','공식 1회'],
[R`Transfer $\to$ Proceed`,'있음','5,361,496','Event별 median 합'],
[R`Transfer $\to$ Recall`,'있음','5,362,498','Event별 median 합']],'lcrl')+R`
Proceed 경로는 3,577,414+1,784,082이며, Recall 경로는 3,577,414+1,785,084이다.
두 값은 각각의 대표 Event median을 합한 값이다. 동일 경로를 한 번에 실행해 측정한 Gas median이나 E2E가 아니다.
직접 전달과의 차이에는 Audit, 추가 transaction, 출력·Tree 작업의 차이가 함께 들어가므로 Voucher만의 순수 비용으로 해석하지 않는다.

SharedSecret 대안은 공유 opening에서 파생한 값으로 Receiver Note의 nf를 계산한다.
Sender도 그 값을 알 수 있어 다음 소비를 연결할 수 있지만, 소비에는 별도로 Receiver secret이 필요하다.
새 측정은 실제 감사 자료 저장·복호화와 Receiver Exit의 동일 nf 소비를 확인했다.

Voucher에서는 Sender가 rvnf와 Proceed의 출력을 연결할 수 있지만 Receiver Note의 nf는 Receiver secret에 의존한다.
감사자는 각 기록의 암호문을 통해 소비 식별자를 복원하여 다음 거래를 찾는다.
이 구분은 공개 시각·메타데이터 등 다른 단서에 의한 추론까지 모두 배제한다는 뜻이 아니다.
`+note('근거: [D1,D5]. SharedSecret nf는 기존 owner-secret nf와 다른 실험용 관계이며, 별도 Ledger를 사용했다.');
const input=process.filter(x=>x[1]===1),output=[process[0],...process.filter(x=>x[1]>1)];
files['08.Process-Scaling.tex']=R`\subsubsection*{Process 입출력 규모의 영향}
State는 3개, 입력 합계는 $(320,30,231)$, 출력 합계는 $(300,30,261)$로 고정한다.
질량 손실률 6.25\%, 추가 탄소 30, WASTE 생성률 0\%이며 모든 출력은 ELIGIBLE이다.
출력 $N=1$에서 입력 $M$을, 입력 $M=2$에서 출력 $N$을 각각 늘렸다.
`+paired(graph('xlabel={Input count (M)},ylabel={Constraints (thousands)},xtick={2,3,4},ymin=0,ymax=100',[input.map(r=>[r[0],r[2]/1000])]),graph('xlabel={Input count (M)},ylabel={Gas (millions)},xtick={2,3,4},ymin=0,ymax=4.2',[input.map(r=>[r[0],r[3]/1e6])]))+paired(graph('xlabel={Output count (N)},ylabel={Constraints (thousands)},xtick={1,2,3},ymin=0,ymax=100',[output.map(r=>[r[1],r[2]/1000])]),graph('xlabel={Output count (N)},ylabel={Gas (millions)},xtick={1,2,3},ymin=0,ymax=4.2',[output.map(r=>[r[1],r[3]/1e6])]))+note('그림 4. Audit on Process의 독립 입력·출력 증가. 같은 지표의 축 범위를 맞췄다.')+table(8,'Process 입출력 개수별 비용',['M','N','Constraints','Gas'],process.map(r=>[r[0],r[1],fmt(r[2]),fmt(r[3])]))+R`
공식 측정은 조건별 1회다. 개별 case의 입력·출력 합계와 실제 receipt가 원자료에 보존되어 있으며, 이번 정리에서도 합계를 대조했다.
입력 2→4에서 Constraints는 60.34\%, Gas는 6.41\% 증가했다. 출력 1→3에서는 각각 49.71\%, 99.36\% 증가했다.
이는 측정한 형상과 상태에서의 관측이며, 일반적인 선형 법칙은 아니다.

기준 $2\to1$의 Gas 1,887,935는 기본 Event 실험의 1,866,617과 다르다.
기본 실험은 전체 lifecycle 중간의 Process이고, 보충 실험은 별도 Entry로 준비한 원장에서 실행했다.
보충 Ledger는 arity 허용 조건도 확장했다. 차이 21,318을 특정 연산 하나의 효과로 단정하거나 두 수치를 통일하지 않는다.
`+note('근거: process-independent.json, 각 case의 inputs·outputs·receipt, verification-after.json [D6].');
files['09.Related-Work.tex']=R`\subsection{기존 연구와의 비교}
기능과 비용은 별도로 비교한다. 아래 문헌 비교는 원 보고서의 범위를 유지하며, 외부 구현을 이번에 재실행한 결과가 아니다.
`+table(9,'관련 기능과 비교 관점',['연구','관련 기능','비교할 내용'],[
['Sahai 등 [2]','공급망 문서 생성·소비와 이력 추적','입력 참조의 공개 범위'],
['Babel 등 [3]','자격증명·비공개 토큰 이전·지분 소비','DPP 생애주기와 후속 대응'],
['GGM [4]','거래에 결합된 추적 자료·후속 출력 추적','객체 타입과 완료 조건'],
['Azeroth [5]','비공개 지급·거래 결합 감사 정보','지급과 공급망 전이의 작업량'],
['CAP [6]','정책별 열람·동결·소비 후 재발행','권한·동결 단위와 조건']],'p{2.1cm}p{6.2cm}p{5.3cm}')+R`
표에 없는 기능을 미지원으로 판정하지 않는다. CAP의 열람은 공개 필드와 거래량 임곗값 등 정책 조건에 따른다.
기존 연구에 같은 완료 절차가 명시되지 않았다는 이유만으로, 그 절차를 추가할 수 없다고 주장하지 않는다.

Azeroth 저자 원고 Table 1(b)의 기본 설정은 MiMC7, Tree depth 32, Groth16이며 zkTransfer는 1,555,957 Gas다.
Azeroth는 잔여 금액을 ENA에 반영하지만 자체 직접 전달은 Change Note를 새로 등록한다.
증명계·Hash·작업량이 다르므로 Gas 차이를 잔여분 처리 하나의 효과로 귀속하지 않는다.
`+table(10,'출처를 구분한 참고 Gas 비교',['대상','Gas','범위'],[
['Azeroth zkTransfer','1,555,957','저자 보고값; Audit 포함'],
['자체 직접 전달','2,366,228','Audit 없음; 5회 median'],
['SharedSecret 직접 전달','2,724,242','Audit 있음; 공식 1회'],
['zkDPP Transfer','3,577,414','Voucher·Change 생성'],
[R`Transfer $\to$ Proceed`,'5,361,496','두 Event median 합']],'lrl')+R`
이 표는 동일 조건에서의 효율 순위가 아니다. Sahai의 Fabric 지표를 EVM Gas와 동일 단위로 비교하지 않는다.
원 보고서가 설명한 Babel의 운영자 원장 시간과 receipt까지 포함한 E2E도 측정 경계가 다르다.
CAP는 상태를 바꿔 재발행하는 동결을 사용하므로 현재 객체의 상태 mapping을 바꾸는 본 구현과 작업을 맞춰야 한다.

이번 결과는 구현한 Event·감사 흐름과 그 비용을 보여준다.
같은 기능·신뢰·작업량에서 외부 시스템보다 우수하다는 결론은 별도 비교가 필요하다.
`;
files['10.Sources.tex']=R`\section*{참고문헌과 실행 근거}
\textbf{외부 문헌 — 원본 보고서의 인용 번호 유지}
\begin{enumerate}
\item zkDPP 프로젝트. 과거 M1 결과, commit 8560fd3. 이 수정본의 주요 기능 근거는 아래 [D1]~[D3]으로 갱신했다.
\item Sahai, Singh, Dayama. Enabling Privacy and Traceability in Supply Chains using Blockchain and Zero Knowledge Proofs. IEEE Blockchain 2020, pp. 134--143. DOI: \url{https://doi.org/10.1109/Blockchain50366.2020.00024}.
\item Babel et al. Human-Centric Digital Product Passports: Enabling Verifiable Information Sharing for Sustainable Consumption through Wallet-Based Identity Management and Zero-Knowledge Proofs. HICSS 2025. DOI: \url{https://doi.org/10.24251/HICSS.2025.527}.
\item Garman, Green, Miers. Accountable Privacy for Decentralized Anonymous Payments. FC 2016 proceedings, 2017. DOI: \url{https://doi.org/10.1007/978-3-662-54970-4_5}.
\item Jeong et al. Azeroth: Auditable Zero-Knowledge Transactions in Smart Contracts. IEEE Access 2023. DOI: \url{https://doi.org/10.1109/ACCESS.2023.3279408}. 비용은 저자 원고 Table 1(b).
\item Espresso Systems. Configurable Asset Privacy specification, release 2022-03-04, revision b4e67cb.
\end{enumerate}
\textbf{내부 근거 — zkDPP-poc-v2 기준 경로}
\begin{description}
\item[D1] \path{output/experiments/experiment-1.json}, \path{experiment-2.json}, \path{experiment-3.json}, \path{verification.json}; \path{internal/experiments/run.go}.
\item[D2] \path{output/m1-hotfix-audit.json}, \path{m1-hotfix-anvil.json}; \path{milestones/M1-HF-spec-conformance-result.md}.
\item[D3] \path{output/experiments/experiment-4.json}; \path{internal/experiments/graphs.go}.
\item[D4] \path{output/paper-supplement/prepared-recovery/result.json}, \path{README.md}; \path{internal/experiments/prepared_recovery.go}.
\item[D5] \path{output/paper-supplement/shared-transfer/result.json}, \path{verification.json}, \path{README.md}.
\item[D6] \path{output/paper-supplement/process-independent.json}, \path{process-*-audit.json}, \path{verification-after.json}, \path{environment-final.json}.
\item[D7] \path{output/paper-supplement/constraint-attribution/constraints.json}; \path{transfer-gas-attribution/breakdown.json}, \path{other-execution-breakdown.json}（いずれも同じ補足測定ディレクトリ）.
\end{description}
`.replace('（いずれも同じ補足測定ディレクトリ）','(모두 paper-supplement 아래)');
files['11.Change-Log.tex']=R`\section*{원본 PDF에서 변경한 내용}
\begin{enumerate}
\item \textbf{1.2 시나리오를 분리했다.} 표 3의 14 Event 기본 성능 경로와 Entry→Split→Split 감사 예시를 구분했다.
\item \textbf{기능 근거를 갱신했다.} 과거 M1 mock 실행 대신 M1-HF 실제 Proof·RPC 결과와 후속 실험 자료를 사용했다. 과거 7→10 블록을 새 실행의 64→67로 구분했다.
\item \textbf{반복 통계를 명시했다.} 기본 5회 median, 보충 공식 1회, 키 복구 1,000회 묶음 평균을 분리했다. 보충 실험의 하드웨어를 기본 실험에 소급하지 않았다.
\item \textbf{키 복구 표·그래프를 교체했다.} 전체 Committee 사전 검증을 제외한 새 k 측정을 사용했다. n 변화 그래프는 삭제하고 n=10 고정 조건은 남겼다.
\item \textbf{SharedSecret 비용을 추가했다.} 미제공으로 비워 둔 값을 실제 공식 1회의 2,724,242 Gas로 채웠다. 기존 nf와 다른 실험용 대안임을 명시했다.
\item \textbf{Process 조건을 확인했다.} 보존된 입력·출력에서 합계를 대조하고 실제 측정 조건을 설명했다. 두 기준 Gas 값은 서로 다른 실행이므로 그대로 유지했다.
\item \textbf{비용 분해 근거를 추가했다.} 이후 수행한 Transfer compile profile과 opcode trace를 별도 진단으로 설명했다.
\item \textbf{기존의 타당한 제한은 유지했다.} 경로 합산과 실측 E2E, 방문 수와 동결 수, 기본 키 복구와 두 키 구성, 외부 논문 비용 비교의 차이를 유지했다.
\end{enumerate}
\textbf{아직 남는 확인}\par
논문 초안의 두 키 방식과 실제 구현의 대응, 기본 실험의 하드웨어·OS 출처, Process 기준 Gas 차이의 정확한 연산별 기여도는 별도 확인 대상이다.
이번 작업에서 새 실험이나 외부 시스템 재실행은 하지 않았다. 원본 PDF는 수정하지 않았다.
`;
const main=R`% !TEX program = xelatex
\documentclass[11pt,a4paper]{article}
\usepackage{kotex}
\usepackage[margin=18mm]{geometry}
\usepackage{amsmath,amssymb,booktabs,array,graphicx,needspace,pgfplots}
\usepackage[hidelinks]{hyperref}
\usepackage{xurl}
\pgfplotsset{compat=1.18}
\IfFontExistsTF{Nanum Myeongjo}{\setmainhangulfont{Nanum Myeongjo}}{\setmainhangulfont[Path=/System/Library/AssetsV2/com_apple_MobileAsset_Font8/4c932c71d74fc9e4c1bd9cbf270374b0b3ee7519.asset/AssetData/,FontIndex=0,BoldFont=NanumMyeongjo.ttc,BoldFeatures={FontIndex=1}]{NanumMyeongjo.ttc}}
\setlength{\parindent}{0pt}
\setlength{\parskip}{7pt}
\setlength{\emergencystretch}{3em}
\renewcommand{\arraystretch}{1.2}
\begin{document}
{\LARGE\bfseries zkDPP 실험 평가 — 검토 반영본}\par\medskip
`+Object.keys(files).map((f,i)=>(i?R`\clearpage`+'\n':'')+`\\input{${f}}`).join('\n')+'\n'+R`\end{document}`+'\n';
files['main.tex']=main;
for(const [name,body] of Object.entries(files))fs.writeFileSync(path.join(dir,name),(name==='main.tex'?'':'% !TEX root = main.tex\n')+body+'\n',{flag:'wx'});
const source='/Users/bighim/Downloads/paper-evaluation-revised.pdf';
fs.copyFileSync(source,path.join(dir,'original.pdf'),fs.constants.COPYFILE_EXCL);
const sources=['output/experiments/experiment-1.json','output/experiments/experiment-2.json','output/experiments/experiment-3.json','output/experiments/experiment-4.json','output/experiments/verification.json','output/m1-hotfix-audit.json','output/m1-hotfix-anvil.json','output/paper-supplement/prepared-recovery/result.json','output/paper-supplement/shared-transfer/verification.json','output/paper-supplement/process-independent.json','output/paper-supplement/verification-after.json','output/paper-supplement/environment-final.json','docs/paper-data/audit-overhead.csv'];
const hashes=Object.fromEntries(sources.map(p=>[p,crypto.createHash('sha256').update(fs.readFileSync(path.join(root,p))).digest('hex')]));
fs.writeFileSync(path.join(dir,'data-provenance.json'),JSON.stringify({originalPDF:crypto.createHash('sha256').update(fs.readFileSync(source)).digest('hex'),sources:hashes,checks:{basicFiveOfficialRuns:true,allBasicRowsMatched:true,processTotalsChecked:true,hotfixRealVerifiers:true,keyPrevalidationOutsideTimer:true,sharedTransferGas:shared.gas}},null,2)+'\n',{flag:'wx'});
console.log('Created',Object.keys(files).length,'TeX files.');
