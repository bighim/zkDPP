package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type stats struct{ Median, Min, Max float64 }
type gadget struct {
	Gadget                                                                              string `json:"gadget"`
	Constraints, PublicInputs, ProofBytes, ProvingKeyBytes, VerifyingKeyBytes, SRSBytes int
	ProveMS                                                                             float64 `json:"proveMs"`
	VerifyMS                                                                            float64 `json:"nativeVerifyMs"`
}
type gadgetReport struct {
	Results []gadget `json:"results"`
}
type payload struct {
	Event                string `json:"event"`
	Proofs, LogicalBytes int
	ProveMS              float64 `json:"proveMs"`
	TotalMS              float64 `json:"totalMs"`
}
type payloadReport struct {
	Results []payload `json:"results"`
}
type evmItem struct {
	Event     string `json:"event"`
	CaseCount int    `json:"caseCount"`
	Gas       stats  `json:"receiptGasUsed"`
	Calldata  stats  `json:"calldataBytes"`
	Prove     stats  `json:"proveMs"`
	Submit    stats  `json:"submitToReceiptMs"`
	E2E       stats  `json:"e2eMs"`
}
type evmReport struct {
	Results []evmItem `json:"results"`
}
type compareItem struct {
	SahaiEvent     string  `json:"sahaiEvent"`
	ZkDPPRelation  string  `json:"zkdppClosestRelation"`
	SahaiCaseCount int     `json:"sahaiCaseCount"`
	ZkDPPCaseCount int     `json:"zkdppCaseCount"`
	SahaiGas       float64 `json:"sahaiGasMedian"`
	ZkDPPGas       float64 `json:"zkdppGasMedianAcrossCases"`
	GasRatio       float64 `json:"sahaiOverZkdppGas"`
	SahaiE2EMS     float64 `json:"sahaiE2EMsMedian"`
	ZkDPPE2EMS     float64 `json:"zkdppE2EMsMedianAcrossCases"`
	E2ERatio       float64 `json:"sahaiOverZkdppE2E"`
}
type compareReport struct {
	Results []compareItem `json:"results"`
}
type paperIII struct {
	Gadget                                       string
	ProveSTMS, ProveMTMS, VerifySTMS, VerifyMTMS float64
}
type paperV struct {
	Event                        string
	Proofs, SizeBytes            int
	TimeSTSeconds, TimeMTSeconds float64
}
type paperReport struct {
	TableIII []paperIII `json:"tableIII"`
	TableV   []paperV   `json:"tableV"`
}
type gate struct {
	Passed bool    `json:"passed"`
	Submit float64 `json:"submitToReceiptMs"`
}
type environment struct {
	GoVersion, CPU, ProofSystem, Gnark, GnarkCrypto, Foundry, Solidity, EVMRevision, Besu, BesuImage string
	CPUCores                                                                                         int
}

func main() {
	root := flag.String("root", ".", "sahai-poc root")
	out := flag.String("out", "Sahai POC Evaluation Report.tex", "output TeX")
	flag.Parse()
	var p2st, p2mt, shast, shamt gadgetReport
	var p2payload, shapayload payloadReport
	var p2gas, shagas, p2e2e, shae2e, besue2e evmReport
	var comparison compareReport
	var paper paperReport
	var env environment
	var parity, permission, node gate
	var tolerance gate
	read(*root, "benchmarks/gadgets-poseidon2-st.json", &p2st)
	read(*root, "benchmarks/gadgets-poseidon2-mt.json", &p2mt)
	read(*root, "benchmarks/gadgets-sha256-st.json", &shast)
	read(*root, "benchmarks/gadgets-sha256-mt.json", &shamt)
	read(*root, "benchmarks/event-payload-canonical-poseidon2-mt.json", &p2payload)
	read(*root, "benchmarks/event-payload-canonical-sha256-mt.json", &shapayload)
	read(*root, "benchmarks/anvil-poseidon2-gas.json", &p2gas)
	read(*root, "benchmarks/anvil-sha256-gas.json", &shagas)
	read(*root, "benchmarks/anvil-poseidon2-mt-e2e.json", &p2e2e)
	read(*root, "benchmarks/anvil-sha256-mt-e2e.json", &shae2e)
	read(*root, "benchmarks/besu-poseidon2-mt-e2e.json", &besue2e)
	read(*root, "benchmarks/zkdpp-poc-comparison.json", &comparison)
	read(*root, "config/paper-benchmarks.json", &paper)
	read(*root, "benchmarks/environment.json", &env)
	read(*root, "benchmarks/backend-parity.json", &parity)
	read(*root, "benchmarks/besu-permission-smoke.json", &permission)
	read(*root, "benchmarks/besu-node-permission-smoke.json", &node)
	read(*root, "benchmarks/besu-validator-tolerance.json", &tolerance)
	if !parity.Passed || !permission.Passed || !node.Passed || !tolerance.Passed {
		panic("M7 gate failed")
	}

	var b strings.Builder
	b.WriteString(preamble)
	writePurpose(&b)
	writeScenario(&b)
	writeEnvironment(&b, env)
	writeGadgets(&b, p2st, p2mt, shast, shamt)
	writePayload(&b, p2payload, shapayload)
	writeHashAblation(&b, p2gas, shagas, p2e2e, shae2e)
	writeComparison(&b, comparison)
	writeBesu(&b, p2e2e, besue2e, tolerance)
	writeLimits(&b)
	writePaper(&b, paper, shast, shamt, p2st, p2mt, shapayload)
	b.WriteString("\\end{document}\n")
	path := filepath.Join(*root, *out)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
}

const preamble = `\documentclass[10pt]{article}
\usepackage{kotex}
\usepackage[margin=18mm]{geometry}
\usepackage{booktabs,longtable,tabularx,array,xcolor,hyperref}
\usepackage{siunitx}
\hypersetup{hidelinks}
\setlength{\parindent}{0pt}
\setlength{\parskip}{0.45em}
\newcolumntype{Y}{>{\raggedright\arraybackslash}X}
\title{RSA-free Sahai-POC와 zkDPP-POC 성능 평가 보고서}
\author{gnark PLONK-KZG 및 EVM 재현 실험}
\date{2026년 8월}
\begin{document}
\maketitle
\tableofcontents
\newpage
`

func writePurpose(b *strings.Builder) {
	b.WriteString(`\section{목적과 핵심 비교 질문}
이 보고서는 Sahai et al. (2020)의 여섯 supply-chain Event predicate를 재현한 \textbf{Sahai-POC}와 기존 \textbf{zkDPP-POC}의 성능을 비교한다. 공식 Sahai-POC는 RSA accumulator를 제거하고 \texttt{DocHash}를 commitment 겸 ledger document ID로 사용한다. 모든 input은 한 번만 소비되는 hardened semantics다.

핵심 질문은 (1) 같은 Poseidon2/PLONK/EVM 환경에서 Event별 gas와 live-proof E2E가 얼마인가, (2) SHA-256을 쓰면 constraint와 proving cost가 얼마나 변하는가, (3) Anvil과 permissioned Besu QBFT에서 EVM gas가 같은가이다. 두 protocol의 Event semantics는 완전히 같지 않으므로 기능적으로 가장 가까운 관계의 실제 값을 나란히 제시하며 동등 protocol이라고 주장하지 않는다.

\subsection{용어와 측정 원칙}
\begin{description}
\item[Sahai-POC] RSA-free, \texttt{DocHash}-ID adaptation. 직접 provenance는 \texttt{InDocs}로 유지한다.
\item[zkDPP-POC] 기존 \texttt{poc-v2} protocol/circuit. 원본 checkout은 수정하지 않고 임시 복사본에서 실행했다.
\item[MT] \texttt{GOMAXPROCS=8}. 독립 proof 호출은 순차적이고 gnark 내부 병렬성만 사용한다. 공식 Event 비교는 MT만 사용한다.
\item[ST] \texttt{GOMAXPROCS=1}. 원 논문 Table III와 gadget 단위로 비교하기 위한 진단값이며 공식 Event 비교에는 사용하지 않는다.
\item[Gas] fixed proof transaction receipt의 \texttt{gasUsed}. 통화 단위 fee가 아니며 proof 생성 시간과 별개다.
\item[E2E] build, witness, 순차 prove, ABI encode, transaction 제출부터 receipt까지의 시간. circuit compile/setup, artifact load, deployment는 제외한다.
\end{description}
각 공식 조합은 한 번만 실행했다. 같은 Event가 시나리오에 여러 번 나타나면 독립 반복 run이 아니라 서로 다른 case이며, 표에는 case 수와 case 중앙값을 쓴다.
`)
}

func writeScenario(b *strings.Builder) {
	b.WriteString(`\section{연결 canonical 시나리오}
\begin{longtable}{cllllrr}
\toprule 순서 & Case & Event & Sender & Recipient & 입력 수량 & 출력 수량\\
\midrule\endhead
1 & entry\_a & Entry & Supplier A & Supplier A & -- & 100\\
2 & ship\_a & Ship & Supplier A & Processor & 100 & 100\\
3 & split\_a & Split & Processor & Processor & 100 & 40+60\\
4 & merge\_a & Merge & Processor & Manufacturer & 40+60 & 100\\
5 & entry\_b & Entry & Supplier B & Supplier B & -- & 50\\
6 & ship\_b & Ship & Supplier B & Manufacturer & 50 & 50\\
7 & process\_product & Process & Manufacturer & Exit actor & 100+50 & 150\\
8 & exit\_product & Exit & Exit actor & Exit actor & 150 & 150 terminal\\
\bottomrule
\end{longtable}
이 표는 공식 성능 비교에서 사용하는 연결된 8-transaction DAG를 정의한다. 각 output \texttt{DocHash}는 다음 Event의 \texttt{inputId}와 \texttt{InDocs}에 실제로 연결된다. Entry와 Ship은 각각 두 case, 나머지는 한 case다. Exit은 새 terminal document를 만들고 output의 \texttt{Sender=Recipient}를 Path 1개와 Eq 1개로 증명한 뒤 input을 consumed 처리한다. zkDPP-POC는 자체 battery/material 시나리오를 사용하므로 case 수와 predicate가 다르다.
`)
}

func writeEnvironment(b *strings.Builder, e environment) {
	fmt.Fprintf(b, `\section{실험 환경과 측정 경계}
\begin{tabularx}{\textwidth}{lY}
\toprule 항목 & 값\\\midrule
CPU & %s (%d logical cores)\\
Go / gnark & %s / %s, gnark-crypto %s\\
Proof & %s, Poseidon2 기본 / SHA-256 ablation\\
EVM & Foundry %s, Solidity %s, %s, chain ID 31337, block gas 30M\\
Besu & %s, 4 validators + 1 RPC, QBFT block period 1초\\
실행 횟수 & profile/thread/backend의 공식 조합당 1회\\
\bottomrule
\end{tabularx}
Gas는 fixed proof의 top-level receipt만 포함하며 deployment와 setup용 state seed는 제외한다. E2E는 fresh proof 생성과 submit-to-receipt를 포함한다. MT에서도 Event 안 proof를 동시에 만들지 않으므로 protocol-level parallel batching 효과는 포함되지 않는다.
`, tex(e.CPU), e.CPUCores, tex(e.GoVersion), tex(e.Gnark), tex(e.GnarkCrypto), tex(e.ProofSystem), tex(e.Foundry), tex(e.Solidity), tex(e.EVMRevision), tex(e.Besu))
}

func writeGadgets(b *strings.Builder, p2st, p2mt, shast, shamt gadgetReport) {
	b.WriteString(`\section{Poseidon2와 SHA-256 gadget 비교}
\begin{longtable}{llrrrrrr}
\toprule Profile & Gadget & Constraint & Public & Proof B & ST prove ms & MT prove ms & MT verify ms\\
\midrule\endhead
`)
	for _, profile := range []struct {
		name   string
		st, mt gadgetReport
	}{{"Poseidon2", p2st, p2mt}, {"SHA-256", shast, shamt}} {
		for _, name := range []string{"merklepath", "eq", "add", "and"} {
			s, m := findG(profile.st.Results, name), findG(profile.mt.Results, name)
			fmt.Fprintf(b, "%s & %s & %d & %d & %d & %.1f & %.1f & %.3f\\\\\n", profile.name, gadgetName(name), m.Constraints, m.PublicInputs, m.ProofBytes, s.ProveMS, m.ProveMS, m.VerifyMS)
		}
	}
	b.WriteString(`\bottomrule\end{longtable}
이 표는 hash profile이 독립 PLONK gadget의 constraint와 prove/verify에 미치는 영향을 묻는다. 각 행은 동일 fixture와 key로 ST 1회, MT 1회 실행한 값이다. verify는 Go native verifier 1회이며 Solidity gas가 아니다. SHA-256은 bit-level compression 때문에 Poseidon2보다 constraint와 prove 시간이 크게 증가하지만 proof byte 차이는 작다. ST는 여기와 마지막 원 논문 비교에서만 사용한다.
`)
}

func writePayload(b *strings.Builder, p2, sha payloadReport) {
	b.WriteString(`\section{Sahai-POC Event payload와 hash ablation}
\begin{longtable}{lrrrrrrr}
\toprule Event & Cases & Proof & P2 logical B & SHA logical B & P2 prove ms & SHA prove ms & SHA/P2\\
\midrule\endhead
`)
	for _, name := range events() {
		pa, pc := aggregatePayload(p2.Results, name)
		sa, _ := aggregatePayload(sha.Results, name)
		fmt.Fprintf(b, "%s & %d & %d & %d & %d & %.1f & %.1f & %.1f\\\\\n", name, pc, pa.Proofs, pa.LogicalBytes, sa.LogicalBytes, pa.ProveMS, sa.ProveMS, ratio(sa.ProveMS, pa.ProveMS))
	}
	b.WriteString(`\bottomrule\end{longtable}
이 표는 연결 시나리오에서 Event bundle을 만들 때 hash 선택이 payload와 순차 proof 생성에 미치는 영향을 보여준다. Logical byte는 JSON payload이며 ABI calldata와 다르다. Entry/Ship은 두 case 중앙값, 나머지는 한 case 값이다. Proof 수는 각각 Entry 2, Ship 5, Merge/Split/Process 6, Exit 2다. 모두 MT이며 setup과 artifact load는 제외한다. 마지막 열은 SHA-256 prove / Poseidon2 prove다.
`)
}

func writeHashAblation(b *strings.Builder, p2g, shag, p2e, shae evmReport) {
	b.WriteString(`\subsection{Anvil에서의 hash profile 영향}
\begin{longtable}{lrrrrrr}
\toprule Event & Cases & P2 gas & SHA gas & Gas 비 & P2 E2E ms & SHA E2E ms\\
\midrule\endhead
`)
	for _, name := range events() {
		pg, sg := findE(p2g.Results, name), findE(shag.Results, name)
		pe, se := findE(p2e.Results, name), findE(shae.Results, name)
		fmt.Fprintf(b, "%s & %d & %.0f & %.0f & %.3f & %.1f & %.1f\\\\\n", name, pg.CaseCount, pg.Gas.Median, sg.Gas.Median, ratio(sg.Gas.Median, pg.Gas.Median), pe.E2E.Median, se.E2E.Median)
	}
	b.WriteString(`\bottomrule\end{longtable}
Gas 열은 profile별 fixed proof transaction을 한 번 실행한 receipt \texttt{gasUsed}의 case 중앙값이고, E2E는 fresh proof MT 1회의 case 중앙값이다. Gas 비는 SHA-256/Poseidon2다. SHA-256의 E2E 증가는 대부분 prove에 기인하며 gas 증가는 proof/public-input calldata와 verifier 차이만 반영한다. 즉 native proof 생성비와 EVM 검증비는 서로 다른 병목이다.
`)
}

func writeComparison(b *strings.Builder, c compareReport) {
	b.WriteString(`\section{Anvil 기반 zkDPP-POC 비교}
\subsection{Event receipt gas}
\begin{longtable}{llrrrrr}
\toprule Sahai & zkDPP 대응 & S cases & Z cases & Sahai gas & zkDPP gas & S/Z\\
\midrule\endhead
`)
	for _, v := range c.Results {
		fmt.Fprintf(b, "%s & %s & %d & %d & %.0f & %.0f & %.3f\\\\\n", v.SahaiEvent, v.ZkDPPRelation, v.SahaiCaseCount, v.ZkDPPCaseCount, v.SahaiGas, v.ZkDPPGas, v.GasRatio)
	}
	b.WriteString(`\bottomrule\end{longtable}
이 표는 Poseidon2 MT의 실제 gas를 먼저 보여주고 마지막 열에서만 Sahai-POC/zkDPP-POC 비율을 계산한다. 각 protocol의 연결 시나리오 case 중앙값이며 조합당 fixed-proof run은 1회다. Entry와 Ship은 Sahai-POC의 gas가 더 작지만 Merge와 Exit은 더 크다. 이는 proof 수만이 아니라 각 protocol의 verifier 호출, calldata, state transition 차이를 함께 반영한다.

\subsection{MT live-proof E2E}
\begin{longtable}{llrrrrr}
\toprule Sahai & zkDPP 대응 & S cases & Z cases & Sahai ms & zkDPP ms & S/Z\\
\midrule\endhead
`)
	for _, v := range c.Results {
		fmt.Fprintf(b, "%s & %s & %d & %d & %.1f & %.1f & %.3f\\\\\n", v.SahaiEvent, v.ZkDPPRelation, v.SahaiCaseCount, v.ZkDPPCaseCount, v.SahaiE2EMS, v.ZkDPPE2EMS, v.E2ERatio)
	}
	b.WriteString(`\bottomrule\end{longtable}
이 표는 양쪽 모두 Poseidon2, MT, proof 호출 순차 실행 조건의 E2E를 비교한다. 실제 값을 먼저 제시하고 마지막 열에서 Sahai-POC/zkDPP-POC 비율을 계산한다. Process와 Exit에서는 Sahai-POC가 빠르지만 나머지에서는 느리다. \texttt{proceed}와 \texttt{recall}은 Sahai에 대응 Event가 없어 제외했다. 같은 이름의 Event도 commitment, nullifier/ownership, predicate와 scenario가 달라 절대적인 protocol 우열로 해석할 수 없다.
`)
}

func writeBesu(b *strings.Builder, anvil, besu evmReport, tolerance gate) {
	b.WriteString(`\section{Besu QBFT 결과}
고정 MT proof 파일의 checksum과 calldata가 Anvil/Besu에서 같을 때 여섯 Event와 두 baseline의 \texttt{gasUsed}가 모두 정확히 일치했다. account allowlist 밖 transaction, allowlist 밖 node 연결, Contract participant role 없는 호출은 모두 거부되었다. validator 4개 중 1개를 중단한 상태에서도 transaction이 finality를 얻었다.
\begin{longtable}{lrrrr}
\toprule Event & Cases & Anvil receipt ms & Besu receipt ms & Besu E2E ms\\
\midrule\endhead
`)
	for _, name := range events() {
		a, e := findE(anvil.Results, name), findE(besu.Results, name)
		fmt.Fprintf(b, "%s & %d & %.1f & %.1f & %.1f\\\\\n", name, e.CaseCount, a.Submit.Median, e.Submit.Median, e.E2E.Median)
	}
	fmt.Fprintf(b, `\bottomrule\end{longtable}
이 표는 Poseidon2 MT live-proof 1회에서 backend finality latency를 분리해 보여준다. Besu E2E는 prove와 QBFT submit-to-receipt를 함께 포함한다. Anvil은 즉시 local mining이므로 receipt가 수십 ms인 반면 Besu는 약 1초 block period의 영향을 받는다. validator 1개 중단 Gate의 별도 transaction finality는 %.1f ms였다. 이 section은 permissioned backend overhead를 설명하며 zkDPP 비교 비율에는 사용하지 않는다.
`, tolerance.Submit)
}

func writeLimits(b *strings.Builder) {
	b.WriteString(`\section{RSA-free adaptation으로 제외된 기능과 해석 한계}
제거한 항목은 \texttt{DocPrime}, 500-bit prime certificate, \texttt{DocAccumulator}, membership/non-membership proof, RPoKE와 EVM MODEXP 검증이다. 따라서 \texttt{InDocs}를 따라 직접 provenance DAG를 조회할 수는 있지만 accumulator로 upstream 전체 set을 압축하거나 contamination membership/non-membership을 검증하는 기능은 없다. 같은 document 내용도 salt가 다르면 다른 \texttt{DocHash}가 되며 contract는 중복 \texttt{DocHash}를 거부한다.

이 제거는 PLONK gadget의 constraint를 줄이지 않는다. RSA는 gnark circuit 밖 component였기 때문이다. 대신 payload, calldata, native RSA 시간과 RSA-on-EVM gas가 사라진다. 결과는 “원 Sahai protocol 전체”가 아니라 Event predicate workload의 RSA-free adaptation이며 production 보안 또는 정확한 2020 hardware 시간 재현을 주장하지 않는다.
`)
}

func writePaper(b *strings.Builder, p paperReport, shast, shamt, p2st, p2mt gadgetReport, payload payloadReport) {
	b.WriteString(`\section{Sahai 원 논문과의 비교}
이 section을 마지막에 둔 이유는 본 실험의 핵심 질문이 zkDPP-POC와의 비교이기 때문이다. 논문 Table II의 RSA component와 Table IV의 non-membership은 공식 구현에서 제거되었으므로 stale POC 숫자를 제시하지 않는다.
\subsection{논문 Table III와 SHA-256/Poseidon2 gadget}
\paragraph{Circuit 규모.}
{\small\setlength{\tabcolsep}{5pt}
\begin{longtable}{lcrrrr}
\toprule & \multicolumn{1}{c}{원 논문 SHA-256} & \multicolumn{2}{c}{Sahai-POC SHA-256} & \multicolumn{2}{c}{Sahai-POC Poseidon2}\\
\cmidrule(lr){2-2}\cmidrule(lr){3-4}\cmidrule(lr){5-6}
Gadget & Constraints & Constraints & Public inputs & Constraints & Public inputs\\
\midrule\endhead
`)
	mapName := map[string]string{"MerklePath": "merklepath", "G-Add": "add", "G-Eq": "eq"}
	for _, v := range p.TableIII {
		sha := findG(shamt.Results, mapName[v.Gadget])
		p2 := findG(p2mt.Results, mapName[v.Gadget])
		fmt.Fprintf(b, "%s & 미보고 & %d & %d & %d & %d\\\\\n", tex(v.Gadget), sha.Constraints, sha.PublicInputs, p2.Constraints, p2.PublicInputs)
	}
	b.WriteString(`\bottomrule\end{longtable}
}
이 표는 각 gadget의 circuit 규모를 보여준다. Constraints는 PLONK circuit이 증명해야 할 제약식 수이고, Public inputs는 verifier에게 공개되는 field element 수다. 원 논문은 libsnark 구현의 constraint와 public input 수를 보고하지 않았으므로 임의로 추정하지 않는다. SHA-256은 bit-level hash 연산으로 Poseidon2보다 약 163--251배 많은 constraint를 사용하며, 이 차이가 아래 prove 시간 차이의 주요 원인이다. Public inputs은 SHA-256 digest를 128-bit limb 2개로 표현하기 때문에 Poseidon2보다 2배 많다.

\paragraph{Proof 생성 시간.}
{\small\setlength{\tabcolsep}{3pt}
\begin{longtable}{lrrrrrr}
\toprule & \multicolumn{2}{c}{원 논문 SHA-256} & \multicolumn{2}{c}{Sahai-POC SHA-256} & \multicolumn{2}{c}{Sahai-POC Poseidon2}\\
\cmidrule(lr){2-3}\cmidrule(lr){4-5}\cmidrule(lr){6-7}
Gadget & ST & MT & ST & MT & ST & MT\\
\midrule\endhead
`)
	for _, v := range p.TableIII {
		shaS, shaM := findG(shast.Results, mapName[v.Gadget]), findG(shamt.Results, mapName[v.Gadget])
		p2S, p2M := findG(p2st.Results, mapName[v.Gadget]), findG(p2mt.Results, mapName[v.Gadget])
		fmt.Fprintf(b, "%s & %.1f & %.1f & %.1f & %.1f & %.1f & %.1f\\\\\n", tex(v.Gadget), v.ProveSTMS, v.ProveMTMS, shaS.ProveMS, shaM.ProveMS, p2S.ProveMS, p2M.ProveMS)
	}
	b.WriteString(`\bottomrule\end{longtable}
}
두 Sahai-POC profile은 각각 ST 1회(\texttt{GOMAXPROCS=1})와 MT 1회(\texttt{GOMAXPROCS=8})를 측정했다. 단위는 ms이며 작을수록 빠르다. Sahai-POC SHA-256은 원 논문의 hash workload와 가장 가깝지만, libsnark 기반 원 논문과 gnark PLONK-KZG/BLS12-381 기반 POC의 proof system·curve·hardware가 다르므로 직접적인 속도 우열 비교는 아니다. Poseidon2열은 원 논문 재현값이 아니라 본 보고서의 기본 profile에서 hash-friendly circuit을 사용했을 때의 ablation 결과다.

SHA-256 circuit은 MerklePath 1,071,658개, G-Add 644,678개, G-Eq 561,113개 constraint를 사용하지만 Poseidon2는 각각 6,582개, 3,347개, 2,231개다. 따라서 Sahai-POC SHA-256의 느린 prove는 Eq/Add 연산 자체보다 bit-level SHA-256을 PLONK circuit에서 재계산하는 비용에서 주로 발생한다. 반면 MT는 ST 대비 약 6배 빠르므로, 9.1의 차이를 thread 활용 실패로 해석하지 않는다.

\paragraph{Proof 검증 시간.}
{\small\setlength{\tabcolsep}{3pt}
\begin{longtable}{lrrrrrr}
\toprule & \multicolumn{2}{c}{원 논문 SHA-256} & \multicolumn{2}{c}{Sahai-POC SHA-256} & \multicolumn{2}{c}{Sahai-POC Poseidon2}\\
\cmidrule(lr){2-3}\cmidrule(lr){4-5}\cmidrule(lr){6-7}
Gadget & ST & MT & ST & MT & ST & MT\\
\midrule\endhead
`)
	for _, v := range p.TableIII {
		shaS, shaM := findG(shast.Results, mapName[v.Gadget]), findG(shamt.Results, mapName[v.Gadget])
		p2S, p2M := findG(p2st.Results, mapName[v.Gadget]), findG(p2mt.Results, mapName[v.Gadget])
		fmt.Fprintf(b, "%s & %.3f & %.3f & %.3f & %.3f & %.3f & %.3f\\\\\n", tex(v.Gadget), v.VerifySTMS, v.VerifyMTMS, shaS.VerifyMS, shaM.VerifyMS, p2S.VerifyMS, p2M.VerifyMS)
	}
	b.WriteString(`\bottomrule\end{longtable}
}
단위는 ms이며 Sahai-POC의 검증은 Go native verifier 1회 결과이다. Solidity verifier gas와 transaction receipt 시간은 포함하지 않는다. 원 논문은 proving에서 MT가 ST보다 빠르지만, 짧은 verification은 35 ms에서 48 ms로 증가한다. 논문은 이 역전의 원인을 별도로 분석하지 않으며, Sahai-POC의 one-shot native verify와도 직접 비교하지 않는다.

현재 표의 Sahai-POC 값은 \texttt{LeafIndex}를 public statement로 binding하기 전 artifact에서 측정한 값이다. Protocol conformance 수정 후 circuit·key·verifier를 재생성하고 공식 수치를 다시 측정한다.

\newpage
\subsection{논문 Table V와 RSA-free payload}
아래 표는 논문 payload와 RSA-free POC payload의 구성 차이를 확인한다. POC 열은 SHA-256 MT 연결 시나리오의 case 중앙값이다. 논문 byte에는 RSA/prime/accumulator 관련 구성이 포함되지만 POC는 이를 제거했고 JSON logical payload를 측정했으므로 byte 값은 직접 동등 비교가 아니다. Proof 개수는 Event predicate bundle을 대응시키는 확인용이며 Split/CombineAcc 표 불일치는 RSA-free 범위 밖이다.
\begin{center}
\begin{tabular}{lrrrrr}
\toprule Event & 논문 proof & 논문 byte & 논문 MT s & POC proof & POC logical byte\\
\midrule
`)
	for _, v := range p.TableV {
		a, _ := aggregatePayload(payload.Results, v.Event)
		fmt.Fprintf(b, "%s & %d & %d & %.1f & %d & %d\\\\\n", v.Event, v.Proofs, v.SizeBytes, v.TimeMTSeconds, a.Proofs, a.LogicalBytes)
	}
	b.WriteString(`\bottomrule\end{tabular}
\end{center}
`)
}

func events() []string { return []string{"Entry", "Ship", "Merge", "Split", "Process", "Exit"} }
func findG(v []gadget, n string) gadget {
	for _, x := range v {
		if x.Gadget == n {
			return x
		}
	}
	panic("missing gadget " + n)
}
func findE(v []evmItem, n string) evmItem {
	for _, x := range v {
		if x.Event == n {
			return x
		}
	}
	panic("missing event " + n)
}
func gadgetName(v string) string {
	return map[string]string{"merklepath": "MerklePath", "eq": "Eq", "add": "Add", "and": "And"}[v]
}
func aggregatePayload(v []payload, event string) (payload, int) {
	var a []payload
	for _, x := range v {
		if x.Event == event {
			a = append(a, x)
		}
	}
	if len(a) == 0 {
		panic("missing payload " + event)
	}
	sort.Slice(a, func(i, j int) bool { return a[i].ProveMS < a[j].ProveMS })
	m := a[len(a)/2]
	if len(a)%2 == 0 {
		m.ProveMS = (a[len(a)/2-1].ProveMS + a[len(a)/2].ProveMS) / 2
		m.TotalMS = (a[len(a)/2-1].TotalMS + a[len(a)/2].TotalMS) / 2
		m.LogicalBytes = (a[len(a)/2-1].LogicalBytes + a[len(a)/2].LogicalBytes) / 2
	}
	return m, len(a)
}
func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
func read(root, path string, v any) {
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(raw, v); err != nil {
		panic(err)
	}
}
func tex(v string) string {
	r := strings.NewReplacer("_", "\\_", "%", "\\%", "&", "\\&", "#", "\\#")
	return r.Replace(v)
}
