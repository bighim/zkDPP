package experiments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 본문과 상세 문서는 같은 Raw를 사용하며 본문에는 비교할 값만 표시합니다.
type compactReport struct{ md, tex strings.Builder }

func compactTex(s string) string {
	parts := strings.Split(s, "**")
	for i := range parts {
		parts[i] = escapeTex(parts[i])
		if i%2 == 1 {
			parts[i] = "\\textbf{" + parts[i] + "}"
		}
	}
	paragraphs := strings.Split(strings.Join(parts, ""), "\n\n")
	for i := range paragraphs {
		paragraphs[i] = strings.ReplaceAll(paragraphs[i], "\n", "\\newline\n")
	}
	return strings.Join(paragraphs, "\\par\\smallskip\n")
}
func (w *compactReport) p(s string) {
	w.md.WriteString(strings.ReplaceAll(s, "\n", "  \n") + "\n\n")
	w.tex.WriteString(compactTex(s) + "\n\n")
}
func (w *compactReport) section(s string) {
	fmt.Fprintf(&w.md, "## %s\n\n", s)
	space := 8
	if strings.HasPrefix(s, "환경") {
		space = 24
	}
	fmt.Fprintf(&w.tex, "\\needspace{%d\\baselineskip}\\section*{%s}\n", space, escapeTex(s))
}
func (w *compactReport) table(title, note string, head []string, rows [][]string, textColumns int) {
	fmt.Fprintf(&w.md, "### %s\n\n", title)
	if note != "" {
		w.md.WriteString(strings.ReplaceAll(note, "\n", "  \n") + "\n\n")
	}
	w.md.WriteString("| " + strings.Join(head, " | ") + " |\n|")
	for i := range head {
		if i < textColumns {
			w.md.WriteString("---|")
		} else {
			w.md.WriteString("---:|")
		}
	}
	w.md.WriteString("\n")
	for _, row := range rows {
		cells := append([]string{}, row...)
		for i := range cells {
			cells[i] = reportNumber(cells[i])
		}
		w.md.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	w.md.WriteString("\n")
	w.tex.WriteString("\\par\\noindent\\begin{minipage}{\\linewidth}\n")
	fmt.Fprintf(&w.tex, "\\subsection*{%s}\n", escapeTex(title))
	if note != "" {
		w.tex.WriteString(compactTex(note) + "\\par\\smallskip\n")
	}
	w.tex.WriteString("\\begin{center}\\small\\begin{tabular}{@{}")
	for i := range head {
		if i < textColumns {
			w.tex.WriteString("l")
		} else {
			w.tex.WriteString("r")
		}
	}
	w.tex.WriteString("@{}}\\toprule\n")
	h := make([]string, len(head))
	for i := range head {
		h[i] = "\\textbf{" + escapeTex(head[i]) + "}"
	}
	w.tex.WriteString(strings.Join(h, " & ") + " \\\\\n\\midrule\n")
	for _, row := range rows {
		cells := make([]string, len(row))
		for i := range row {
			cells[i] = escapeTex(reportNumber(row[i]))
		}
		w.tex.WriteString(strings.Join(cells, " & ") + " \\\\\n")
	}
	w.tex.WriteString("\\bottomrule\\end{tabular}\\end{center}\n\\end{minipage}\\par\\medskip\n")
}
func Report(root string) error {
	if err := Check(root); err != nil {
		return err
	}
	env := map[int]Envelope{}
	for i := 1; i <= 8; i++ {
		e, err := loadEnvelope(root, i)
		if err != nil {
			return err
		}
		env[i] = e
	}
	var setup SetupResult
	if err := readJSON(filepath.Join(root, OutputDir, "setup.json"), &setup); err != nil {
		return err
	}
	w := &compactReport{}
	w.md.WriteString("# zkDPP 실험 결과\n\n")
	w.tex.WriteString(`\documentclass[11pt,a4paper]{article}
\usepackage{kotex}
\usepackage[margin=18mm]{geometry}
\usepackage{amsmath,booktabs,array,longtable,pdflscape,needspace,enumitem}
\usepackage[hidelinks]{hyperref}
\usepackage{xurl}
\setlength{\parindent}{0pt}
\setlength{\parskip}{5pt}
\setlength{\emergencystretch}{1em}
\renewcommand{\arraystretch}{1.15}
\begin{document}
\begin{landscape}
\thispagestyle{empty}
{\Large\bfseries zkDPP 실험 결과}\par\medskip
`)
	overview := [][]string{
		{"기본 성능", "공급망 Event별 실행 비용", "Base 설정", "Event 종류", "Entry / Transfer / Proceed / Recall / Merge / Split / Process / Exit / Issue"},
		{"", "제3자의 DPPClaim 확인 비용", "등록된 DPPClaim·문서·Policy", "없음 — 기본 성능", "Hash 재계산 → 온체인 등록·Issue 기록 대조 → 일치 여부 판정"},
		{"추가 기능 비용", "직접 전달과 Voucher 전달·반환의 비용 비교", "Audit 없음", "전달 방식", "Voucher 없이 Receiver Note 생성 / Transfer → Proceed / Transfer → Recall"},
		{"", "Event별 Audit 추가 비용", "Event 종류", "Audit 유무", "없음 / 있음"},
		{"확장성", "Tracing할 Split 기록 수 증가", "Split 방식", "기록 수", "10 / 100 / 1,000 — 한쪽만 계속 Split / 양쪽을 고르게 Split 각각"},
		{"", "길게 이어진 공급망과 고르게 분기한 공급망 비교", "기록 수", "Split 방식", "한쪽만 계속 Split / 양쪽을 고르게 Split"},
		{"", "분기·합류를 반복하는 공급망의 기록 수 증가", "두 Note로 Split한 뒤 다시 Merge", "기록 수", "11 / 101 / 1,001"},
		{"", "동일한 State에서 증명할 Claim 조건 변경", "State 3개", "검증 조건", "재활용률 / 탄소집약도 / 두 조건 모두"},
		{"", "Merge에서 합산·보존을 검증할 State 수 증가", "Merge 2→1·동일한 합산 규칙", "State 개수", "1 / 2 / 3 / 4 / 8 / 16"},
		{"", "Process가 소비·생성하는 Note 수 증가", "입력·출력 State 합계·공정 규칙", "입력·출력 Note 수", "2→1 / 2→2 / 3→2 / 3→3 / 4→3 — Audit 없음 / 있음 각각"},
		{"암호화 세부 비용", "복호화할 AuditRecord 수 증가", "암호문당 Field 3개", "암호문 개수", "1 / 10 / 100 / 1,000"},
		{"", "AuditRecord 한 건의 암호화 정보량 증가", "암호문 1개", "Field 개수", "1 / 2 / 3 / 5 / 10 / 20"},
		{"", "Master Key 복구에 사용하는 share 수 증가", "Committee n=10", "share k", "2 / 3 / 5 / 7 / 10"},
		{"", "동일한 share 수로 복구할 때 전체 Committee 수 증가", "share k=2", "Committee n", "3 / 5 / 10 / 20"},
	}
	head := []string{"구분", "실험", "고정하는 값", "변경하는 값", "변경 수치·방식"}
	w.md.WriteString("| " + strings.Join(head, " | ") + " |\n|---|---|---|---|---|\n")
	for _, r := range overview {
		w.md.WriteString("| " + strings.Join(r, " | ") + " |\n")
	}
	w.md.WriteString("\n")
	w.tex.WriteString("{\\small\\begin{longtable}{@{}p{.09\\linewidth}p{.255\\linewidth}p{.175\\linewidth}p{.125\\linewidth}p{.29\\linewidth}@{}}\\toprule\n")
	hh := make([]string, len(head))
	for i := range head {
		hh[i] = "\\textbf{" + head[i] + "}"
	}
	w.tex.WriteString(strings.Join(hh, " & ") + " \\\\\n\\midrule\\endhead\n")
	for i, r := range overview {
		if i == 2 || i == 4 || i == 10 {
			w.tex.WriteString("\\midrule\n")
		}
		c := make([]string, len(r))
		for j := range r {
			c[j] = escapeTex(r[j])
		}
		w.tex.WriteString(strings.Join(c, " & ") + " \\\\\n")
	}
	w.tex.WriteString("\\bottomrule\\end{longtable}}\n")
	w.p("**기록 수**는 Entry를 포함한 총 AuditRecord 수입니다.\n예를 들어 기록 10개는 Entry 1개와 Split 9개입니다.\n암호화 실험에서는 AuditRecord 한 건을 암호문 한 개로 계산합니다.\n이 암호문은 공개점 하나와 암호화된 Field 배열로 구성됩니다.")
	w.tex.WriteString("\\end{landscape}\n")

	w.section("1. 기본 성능")
	w.p("**기본 설정**\nState는 3개입니다.\nAudit을 사용합니다.\nProcess의 기본 입출력은 2→1입니다.\nIssue에는 Standard Policy를 사용합니다.\n\n**반복 기준**\n입력 Note·path·Policy를 확보한 뒤 측정을 시작합니다.\n각 case는 warm-up 1회 후 공식 5회 실행합니다.\n표에는 공식 5회의 **median**을 표시합니다.")
	w.p("**Event E2E**\n참여자가 해당 Event의 witness를 준비할 때부터 측정합니다.\nProof를 생성하고 transaction을 구성·제출합니다.\n성공 receipt를 확인하면 측정을 끝냅니다.\n로컬 Verify와 사후 상태 대조는 이 시간에서 제외합니다.\n\n**Gas**\n해당 Event를 제출한 계정의 실제 Anvil transaction **gasUsed**입니다.\n온체인 Proof 검증과 상태 변경 비용을 포함합니다.\nContract 배포 비용은 포함하지 않습니다.")
	base, plain := measureRows(env[1]), measureRows(env[3])
	names := []string{"Entry", "Transfer", "Proceed", "Recall", "Merge", "Split", "Process 2→1", "Exit", "Issue"}
	ids := []int{1, 3, 6, 4, 9, 12, 11, 14, 13}
	get := func(rs []Measurement, id string, metric string) float64 {
		return median(values(rs, id, func(r Measurement) float64 {
			switch metric {
			case "constraints":
				return float64(r.Constraints)
			case "gas":
				return float64(r.Tx.Gas)
			default:
				return r.E2EMS
			}
		}))
	}
	var basic, cc, gg, tt [][]string
	for i, n := range names {
		a, b := fmt.Sprintf("exp1-lifecycle-%02d", ids[i]), fmt.Sprintf("exp3-lifecycle-%02d", ids[i])
		basic = append(basic, []string{n, fmt.Sprintf("%.0f", get(base, a, "constraints")), fmt.Sprintf("%.0f", get(base, a, "gas")), fmt.Sprintf("%.3f", get(base, a, "time"))})
		for _, target := range []struct {
			metric string
			rows   *[][]string
			digits int
		}{{"constraints", &cc, 0}, {"gas", &gg, 0}, {"time", &tt, 3}} {
			av, bv := get(base, a, target.metric), get(plain, b, target.metric)
			*target.rows = append(*target.rows, []string{n, fmt.Sprintf("%.*f", target.digits, bv), fmt.Sprintf("%.*f", target.digits, av), fmt.Sprintf("%.*f", target.digits, av-bv)})
		}
	}
	w.table("공급망 Event별 실행 비용", "각 행은 해당 Event 한 번을 실행한 결과입니다.\n다른 Event를 준비하기 위한 선행 transaction 비용은 더하지 않습니다.", []string{"Event", "Constraints", "Gas", "E2E (ms)"}, basic, 1)
	var total, rpc, local, calls []float64
	for _, r := range env[1].DPP {
		if r.Run > 0 {
			total = append(total, r.TotalMS)
			rpc = append(rpc, r.RPCMS)
			local = append(local, r.LocalMS)
			calls = append(calls, float64(r.RPCCalls))
		}
	}
	w.table("제3자의 DPPClaim 확인 비용", "**확인 대상**\n제3자는 제시된 문서·Policy가 온체인에 등록된 DPPClaim과 일치하는지 확인합니다.\n\n**측정 범위**\n확인 함수를 호출하면 Snapshot을 확보합니다.\n문서로 Hash를 다시 계산하고 온체인 등록·Issue·Policy 기록과 대조합니다.\n일치 여부를 판정할 때 측정을 끝냅니다.\n새 Proof를 생성하거나 transaction을 제출하지는 않습니다.\n\n**표의 시간**\n전체는 확인 함수를 호출한 뒤 판정까지 걸린 시간입니다.\nRPC는 그중 온체인 조회 시간입니다.\n로컬은 조회 시간을 제외한 계산·대조 시간입니다.", []string{"전체 (ms)", "RPC (ms)", "로컬 (ms)", "RPC 수"}, [][]string{{stat(total), stat(rpc), stat(local), fmt.Sprintf("%.0f", median(calls))}}, 0)

	w.section("2. 추가 기능 비용")
	var routes [][]string
	for _, p := range []struct{ id, label string }{{"direct", "직접 Transfer"}, {"proceed", "Transfer → Proceed"}, {"recall", "Transfer → Recall"}} {
		var times, gas []float64
		count := 0
		for _, r := range env[2].Routes {
			if r.Run > 0 && r.Route == p.id {
				times = append(times, r.TotalMS)
				gas = append(gas, float64(r.Gas))
				count = r.Transactions
			}
		}
		routes = append(routes, []string{p.label, fmt.Sprint(count), fmt.Sprintf("%.0f", median(gas)), stat(times)})
	}
	w.table("직접 전달과 Voucher 전달·반환", "**고정 조건**\n세 경로 모두 Audit을 사용하지 않습니다.\n입력 State·전달량·운송 탄소·Change 규칙을 동일하게 맞췄습니다.\n\n**비교 경로**\n직접 Transfer는 Receiver Note를 바로 생성합니다.\nTransfer → Proceed는 Voucher를 거쳐 Receiver Note를 생성합니다.\nTransfer → Recall은 Voucher를 거쳐 Sender에게 Note를 반환합니다.\n\n**경로 E2E**\n첫 Transfer의 witness 준비부터 측정합니다.\n직접 전달·수령·반환 중 해당 경로의 마지막 성공 receipt에서 끝납니다.\n로컬 Voucher 전달과 최신 path 확보도 포함합니다.\n\n**총 Gas**\n해당 경로에서 실제 실행한 모든 transaction의 gasUsed를 합산합니다.", []string{"경로", "tx 수", "총 Gas", "경로 E2E (ms)"}, routes, 1)
	w.p("**Voucher를 사용하면 Proof와 transaction이 한 번 더 필요합니다.**\n그 대신 Sender가 Recall로 반환받는 경로를 제공합니다.\n직접 전달에는 Recall에 대응하는 동작이 없습니다.\n따라서 Recall의 비용은 동일 기능의 증가율이 아니라 추가 기능의 비용으로 봅니다.")
	w.table("Event별 Audit 추가 비용 — Constraints", "**고정 조건**\n동일한 Event의 입력·출력 State와 시작 Tree를 맞췄습니다.\nowner·membership·소비 방지·Status·Policy 검사는 유지했습니다.\n\n**변경 조건**\nAudit 암호화·기록·Tracing mapping을 추가한 경우와 추가하지 않은 경우를 비교합니다.\n\n**차이**\nAudit 있음에서 Audit 없음을 뺀 값입니다.", []string{"Event", "Audit 없음", "Audit 있음", "차이"}, cc, 1)
	w.table("Event별 Audit 추가 비용 — Gas", "**측정 대상**\n동일한 Event를 실행한 실제 transaction의 gasUsed를 비교합니다.\n\n**차이의 의미**\nAudit 있음에서 Audit 없음을 뺀 값입니다.\nProof 검증과 상태 변경을 포함한 전체 transaction의 차이입니다.\n**암호문을 저장하는 SSTORE 비용만 따로 뺀 값은 아닙니다.**", []string{"Event", "Audit 없음", "Audit 있음", "차이"}, gg, 1)
	w.p("**관찰 결과**\n9개 Event 모두 Constraints와 gas가 증가했습니다.\n\n**시간 결과의 해석**\nProcess 2→1의 E2E median은 Audit 없음 1,362.572 ms였습니다.\nAudit 있음은 1,335.974 ms였습니다.\n그러나 두 조건의 공식 5회 측정 범위가 겹칩니다.\n따라서 더 작은 median을 Audit에 의한 성능 개선으로 해석하지 않습니다.\n\n**보조 지표가 필요한 이유**\nConstraints만으로 실제 Prove 시간이나 메모리를 알 수는 없습니다.\ngas에도 오프체인 계산과 RPC 대기 시간은 포함되지 않습니다.\n시간 상세값은 별도 부록에 보존합니다.")

	w.section("3. 확장성")
	var ar []AuditRow
	if err := json.Unmarshal(env[4].Rows, &ar); err != nil {
		return err
	}
	ag := map[string][]AuditRow{}
	for _, r := range ar {
		if r.Run > 0 {
			ag[r.Case+"/"+r.Operation] = append(ag[r.Case+"/"+r.Operation], r)
		}
	}
	var tracing [][]string
	for _, shape := range []struct{ id, label string }{{"chain", "연속 Split"}, {"balanced", "균형 Split"}, {"diamond", "Split → Merge"}} {
		for _, size := range []int{10, 100, 1000} {
			if shape.id == "diamond" {
				size++
			}
			p := fmt.Sprintf("%s-%d/", shape.id, size)
			r := []string{shape.label, fmt.Sprint(size), fmt.Sprint(ag[p+"audit-and-freeze"][0].Targets)}
			for _, op := range []string{"forward", "backward", "freeze", "audit-and-freeze"} {
				var ts []float64
				for _, v := range ag[p+op] {
					ts = append(ts, v.TotalMS/1000)
				}
				r = append(r, stat(ts))
			}
			tracing = append(tracing, r)
		}
	}
	w.table("Tracing 기록 수·분기 구조와 Freeze 비용", "**시간 단위는 초(s)**입니다.\n\n**시작점과 대상**\nForward는 최초 cm에서 시작합니다.\nBackward는 가장 깊은 최종 Note에서 시작해 조상을 찾습니다.\n두 Tracing은 복구된 Master Key와 확보한 Snapshot을 사용합니다.\n각각 Tracing을 시작해서 완료할 때까지 측정합니다.\n키 복구와 Snapshot 확보 시간은 두 열에 포함하지 않습니다.\n대상 수는 동결할 미소비·Active Note/Voucher의 수입니다.\n\n**Freeze와 전체 시간**\nFreeze는 이미 찾은 대상의 동결 처리를 시작할 때부터 측정합니다.\n제출 전 상태 검사와 동결 transaction 실행을 포함합니다.\n공통 Snapshot에서 최종 상태를 확인하면 끝납니다.\n전체는 Master Key 복구부터 시작하는 **AuditAndFreeze** 시간입니다.\nSnapshot 확보 → Forward → Freeze → 최종 확인을 포함합니다.\nBackward는 전체 시간에 포함하지 않습니다.", []string{"구조", "기록 수", "대상 수", "Forward", "Backward", "Freeze", "전체"}, tracing, 1)
	w.p("**비교 방법**\n같은 구조의 행끼리 비교하면 기록 수 증가의 영향을 볼 수 있습니다.\n같은 기록 수에서 연속 Split과 균형 Split을 비교하면 분기 구조의 영향을 볼 수 있습니다.\nSplit → Merge는 11/101/1,001개입니다.\n다른 두 구조의 10/100/1,000개와 정확히 같은 기록 수는 아닙니다.\n\n**반복 조건**\n각 감사는 빈 cache에서 시작합니다.\n같은 graph의 읽기 전용 Tracing은 고정 Snapshot에서 반복합니다.\nFreeze는 매회 대상들을 같은 미소비·Active 상태로 되돌린 뒤 실행합니다.\ngraph 구축과 Proof 준비 시간은 제외합니다.\n\n**해석할 때 주의할 점**\nBackward는 전체 기록이 아니라 시작 Note의 조상만 방문합니다.\n각 열은 따로 측정한 시간의 median입니다.\n따라서 각 열을 더한 값이 전체 시간과 같지는 않습니다.")

	profile := func(exp int, name string) ([]Measurement, error) {
		var rs []Measurement
		for _, r := range measureRows(env[exp]) {
			if r.Run > 0 && r.Profile == name {
				rs = append(rs, r)
			}
		}
		if len(rs) != 5 {
			return nil, fmt.Errorf("profile %s: expected 5 runs, got %d", name, len(rs))
		}
		return rs, nil
	}
	performance := func(exp int, name, label string) ([]string, error) {
		rs, e := profile(exp, name)
		if e != nil {
			return nil, e
		}
		var gas, times []float64
		for _, r := range rs {
			gas = append(gas, float64(r.Tx.Gas))
			times = append(times, r.E2EMS)
		}
		return []string{label, fmt.Sprint(rs[0].Constraints), fmt.Sprintf("%.0f", median(gas)), stat(times)}, nil
	}
	var claims, states [][]string
	for _, p := range []struct{ id, label string }{{"issue-recycled-audit", "재활용률"}, {"issue-carbon-audit", "탄소집약도"}, {"issue-audit", "두 조건 모두"}} {
		r, e := performance(5, p.id, p.label)
		if e != nil {
			return e
		}
		claims = append(claims, r)
	}
	w.table("동일한 State에서 증명할 Claim 조건 변경", "**고정 조건**\n동일한 Note의 State (300,30,261)과 Audit 사용 조건을 유지합니다.\nState 순서는 질량·재활용 질량·탄소입니다.\n\n**변경 조건**\n검증 조건을 재활용률만·탄소집약도만·두 조건 모두로 바꿉니다.\n재활용률 기준은 10% 이상입니다.\n탄소집약도 기준은 1.00 kgCO2e/kg 이하입니다.\n\n**측정 대상**\nE2E와 Gas는 각 조건의 **Issue 한 번**을 실행한 비용입니다.", []string{"Claim 조건", "Constraints", "Gas", "E2E (ms)"}, claims, 1)
	for _, n := range []int{1, 2, 3, 4, 8, 16} {
		r, e := performance(5, fmt.Sprintf("vector-merge-%d", n), fmt.Sprint(n))
		if e != nil {
			return e
		}
		states = append(states, r)
	}
	w.table("Merge에서 합산·보존을 검증할 State 수 증가", "**고정 조건**\nMerge 2→1과 Audit 사용 조건을 유지합니다.\n각 State 값이 uint64 범위 안에 있는지 검증합니다.\n입력 State의 합이 출력 State와 같은지도 검증합니다.\n검증한 State는 Note commitment 계산에 포함합니다.\n\n**변경 조건**\n동일한 검증을 적용할 State 개수를 1·2·3·4·8·16개로 늘립니다.\n\n**측정 대상**\nE2E와 Gas는 **Merge 한 번**을 실행한 비용입니다.", []string{"State 수", "Constraints", "Gas", "E2E (ms)"}, states, 0)
	w.p("**두 실험은 변경한 대상이 다릅니다.**\nClaim 조건 실험은 State 수를 유지하고 검증할 조건을 바꿉니다.\nState 개수 실험은 같은 합산 규칙을 더 많은 State에 적용합니다.\nState 개수의 결과만으로 모든 Sustainability 조건의 비용을 설명할 수는 없습니다.")
	for _, metric := range []struct {
		id, title string
		digits    int
	}{{"constraints", "Constraints", 0}, {"gas", "Gas", 0}, {"time", "E2E (ms)", 3}} {
		var rows [][]string
		for _, a := range [][2]int{{2, 1}, {2, 2}, {3, 2}, {3, 3}, {4, 3}} {
			r := []string{fmt.Sprintf("%d→%d", a[0], a[1])}
			for _, variant := range []string{"plain", "audit"} {
				rs, e := profile(6, fmt.Sprintf("process-%d-%d-%s", a[0], a[1], variant))
				if e != nil {
					return e
				}
				r = append(r, fmt.Sprintf("%.*f", metric.digits, get(rs, "", metric.id)))
			}
			rows = append(rows, r)
		}
		note := "**고정 조건**\n입력 State 합계는 (320,30,231)입니다.\n출력 State 합계는 (300,30,261)입니다.\n질량 손실률은 6.25%입니다.\n추가하는 탄소는 30입니다.\nWASTE 생성률은 0%입니다.\n모든 Note는 ELIGIBLE입니다.\nState는 3개입니다.\n\n**변경 조건**\n입력·출력 Note 수를 2→1·2→2·3→2·3→3·4→3으로 바꿉니다.\n각 구성에서 Audit 없음과 Audit 있음을 측정합니다.\n\n**배분 방법**\n입력 총량은 입력 Note 수에 맞춰 균등 분할합니다.\n정수 나눗셈의 잔여분은 입력 1에 둡니다.\n출력별 질량을 먼저 정합니다.\n출력이 2개이면 질량 비율은 2:1입니다.\n출력이 3개이면 질량 비율은 3:2:1입니다.\n재활용 질량과 탄소는 각 출력의 질량에 비례해 배분합니다.\n출력 2 이후 값을 내림한 뒤 잔여분은 출력 1에 둡니다."
		if metric.id == "gas" {
			note = "**Gas**\n각 구성에서 Process transaction 한 번을 실제로 실행한 gasUsed입니다.\n준비용 Entry와 배포 비용은 제외합니다."
		}
		if metric.id == "time" {
			note = "**E2E**\n각 구성의 Process witness 준비부터 성공 receipt 확인까지입니다.\n시간은 Constraints·gas와 함께 보는 보조 지표입니다."
		}
		w.table("Process 입출력 Note 수 증가 — "+metric.title, note, []string{"입력→출력", "Audit 없음", "Audit 있음"}, rows, 1)
	}
	w.p("**domain 변화**\nProcess 2→2에서 Audit을 추가하면 domain이 65,536에서 131,072로 커집니다.\n이 값은 PLONK가 사용하는 계산 영역의 크기입니다.\n따라서 Constraints 증가율이 실제 시간 증가율과 같을 필요는 없습니다.\n\n**결과의 출처**\n위 세 표는 입출력 확장 실험에서 별도로 측정한 결과입니다.\n1장의 기본 성능 표와 같은 실행을 다시 집계한 값이 아닙니다.")

	w.section("4. 암호화 세부 비용")
	cryptoGroups := func(exp int) (map[string][]float64, error) {
		var rs []CryptoRow
		if e := json.Unmarshal(env[exp].Rows, &rs); e != nil {
			return nil, e
		}
		g := map[string][]float64{}
		for _, r := range rs {
			if r.Run > 0 {
				g[r.Case] = append(g[r.Case], r.TotalMS/float64(r.InnerIterations))
			}
		}
		return g, nil
	}
	cg, e := cryptoGroups(7)
	if e != nil {
		return e
	}
	kg, e := cryptoGroups(8)
	if e != nil {
		return e
	}
	var counts, fields, ks, ns [][]string
	for _, n := range []int{1, 10, 100, 1000} {
		counts = append(counts, []string{fmt.Sprint(n), stat(cg[fmt.Sprintf("ciphertexts-%d-fields-3", n)])})
	}
	for _, n := range []int{1, 2, 3, 5, 10, 20} {
		fields = append(fields, []string{fmt.Sprint(n), stat(cg[fmt.Sprintf("ciphertexts-1-fields-%d", n)])})
	}
	w.table("복호화할 AuditRecord 수 증가", "**고정 조건**\n암호문 한 개에 Field 3개를 넣습니다.\nMaster Key 복구와 암호문 생성은 측정 전에 끝냅니다.\n\n**변경 조건**\n복호화할 암호문 수를 1·10·100·1,000개로 늘립니다.\n\n**전체 복호화 시간**\nAuditor가 해당 행의 암호문을 모두 복호화한 시간입니다.\n암호문 한 개의 평균 시간이 아닙니다.\nRPC 조회 시간은 포함하지 않습니다.", []string{"암호문 수", "전체 복호화 (ms)"}, counts, 0)
	w.table("AuditRecord 한 건의 암호화 정보량 증가", "**고정 조건**\n복호화할 암호문은 한 개입니다.\nMaster Key는 이미 복구한 상태입니다.\n\n**변경 조건**\n암호문에 들어 있는 Field 수를 1·2·3·5·10·20개로 늘립니다.\n\n**복호화 시간**\n입력을 검사한 뒤 공유점을 계산합니다.\n키와 위치별 마스크를 유도해 모든 Field의 원문을 복원합니다.\n이 과정 전체를 측정합니다.", []string{"Field 수", "복호화 (ms)"}, fields, 0)
	for _, k := range []int{2, 3, 5, 7, 10} {
		ks = append(ks, []string{fmt.Sprint(k), stat(kg[fmt.Sprintf("%d-of-10", k)])})
	}
	for _, n := range []int{3, 5, 10, 20} {
		ns = append(ns, []string{fmt.Sprint(n), stat(kg[fmt.Sprintf("2-of-%d", n)])})
	}
	w.table("Master Key 복구에 사용하는 share 수 증가", "**고정 조건**\n전체 Committee 수 n은 10입니다.\n위원별 share와 공개 설정을 담은 package는 측정 전에 준비합니다.\n\n**변경 조건**\n복구에 사용하는 share 수 k를 2·3·5·7·10으로 바꿉니다.\n\n**복구 시간**\npackage와 k개 share를 검사합니다.\n보간으로 Master Key를 복구한 뒤 공개키와 일치하는지 확인합니다.\n이 로컬 계산 전체를 측정합니다.\nDKG·위원 통신·승인 대기는 제외합니다.", []string{"share k", "복구 (ms)"}, ks, 0)
	w.table("동일한 share 수로 복구할 때 전체 Committee 수 증가", "**고정 조건**\n복구에는 항상 share 2개를 사용합니다.\n\n**변경 조건**\n전체 Committee 수 n을 3·5·10·20으로 늘립니다.\nn이 늘어나면 package에서 검사할 위원 정보도 많아집니다.\n\n**복구 시간**\npackage 검사부터 Master Key 복구·공개키 대조까지 측정합니다.\n복구에 사용한 share 수가 늘어난 결과는 아닙니다.\nDKG·위원 통신·승인 대기는 제외합니다.", []string{"Committee n", "복구 (ms)"}, ns, 0)
	w.p("**짧은 작업의 반복 방법**\n각 공식 run에서 해당 workload를 1,000회 실행합니다.\n반복 전체 시간을 1,000으로 나누어 workload 한 번의 시간을 구합니다.\n이렇게 얻은 공식 run 5개의 median을 표에 표시합니다.\n\n**중복된 조건**\n두 복호화 표의 암호문 1개·3 Field 조건은 같은 측정값입니다.\n두 키 복구 표의 2-of-10 조건도 같은 측정값입니다.")

	w.section("환경·한계·원자료")
	w.p("**증명 환경**\nGo 1.25.7, gnark 0.15.0, gnark-crypto 0.20.1을 사용합니다.\nProof system은 PLONK-KZG/BLS12-381입니다.\n암호화에는 Jubjub·Poseidon2를 사용합니다.\nGOMAXPROCS는 8입니다.\n\n**온체인 환경**\nAnvil 1.7.1, Solidity 0.8.30, Prague를 사용합니다.\nchain ID는 31337이고 gas limit은 30M입니다.\n\n**State 표기**\nState 순서는 질량·재활용 질량·탄소입니다.\n구현에서는 각 값을 10^9 scale 정수로 처리합니다.")
	w.p(fmt.Sprintf("**준비 비용**\ndevelopment SRS %s points를 생성하는 데 %.3f ms가 걸렸습니다.\n%d개 relation의 PK·VK를 생성했습니다.\nSetup과 배포는 Event E2E에 포함하지 않습니다.\n준비 작업은 공식 case처럼 5회 반복한 값이 아닙니다.", reportNumber(fmt.Sprint(setup.CanonicalPoints)), setup.SRSMS, len(setup.Relations)))
	w.p("**실패와 재실행**\n최초 Tracing 실행은 Docker 메모리 부족(OOM, exit 137)으로 실패했습니다.\n실패 기록은 그대로 보존했습니다.\n재실행에서는 step tracing을 끄고 graph마다 새 노드를 사용했습니다.\n다른 실험과 계측 설정이 다르므로 절대시간을 직접 비교하지 않습니다.")
	w.p("**해석 범위**\n이 결과는 Anvil에서 구성한 graph의 실험값입니다.\nProduction finality나 동시 처리량을 입증하지 않습니다.\nDKG와 안전한 키 폐기도 이번 측정 대상이 아닙니다.\nPrivacy 보장을 성능값으로 증명하지 않습니다.\n기존 M1-HF와 v1 결과는 재측정하지 않았습니다.")
	w.p("**상세값**\nexperiment-results-details.md에서 공식 5회·min–max·세부 시간·Setup 표를 확인할 수 있습니다.\nRaw는 output/experiments/experiment-1.json부터 experiment-8.json에 있습니다.\nSetup Raw는 같은 폴더의 setup.json입니다.\n\n**재현**\nmake check-experiments로 저장된 결과를 검증합니다.\nmake report-experiments로 보고서를 생성합니다.\n보고서를 만들기 위해 benchmark를 다시 실행하지는 않습니다.")
	w.tex.WriteString("\\end{document}\n")
	details := reportDetails(env) + "\n## Circuit별 Setup\n\n준비 작업 1회입니다. SRS 생성과 Proof 생성은 포함하지 않습니다.\n\n| Relation | Constraints | Domain | Setup ms |\n|---|---:|---:|---:|\n"
	for _, r := range setup.Relations {
		details += fmt.Sprintf("| %s | %d | %d | %.3f |\n", r.Profile.Name, r.Constraints, r.Domain, r.SetupMS)
	}
	details += "\n## Audit E2E 보조 비교\n\n단위 ms. 각 Event의 witness 준비부터 성공 receipt까지의 median입니다.\n\n| Event | Audit 없음 | Audit 있음 | 차이 |\n|---|---:|---:|---:|\n"
	for _, r := range tt {
		details += "| " + strings.Join(r, " | ") + " |\n"
	}
	for _, f := range []struct{ name, body string }{{"experiment-results.md", w.md.String()}, {"experiment-results.tex", w.tex.String()}, {"experiment-results-details.md", details}} {
		p := filepath.Join(root, "docs", f.name)
		file, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e != nil {
			return e
		}
		_, e = file.WriteString(f.body)
		ce := file.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return nil
}
