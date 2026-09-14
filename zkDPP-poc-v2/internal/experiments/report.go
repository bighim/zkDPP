package experiments

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func escapeTex(s string) string {
	r := strings.NewReplacer("10^9", "$10^{9}$", "\\", "\\textbackslash{}", "_", "\\_", "%", "\\%", "&", "\\&", "#", "\\#", "$", "\\$", "{", "\\{", "}", "\\}", "^", "\\textasciicircum{}", "→", "$\\rightarrow$", "·", " / ")
	return r.Replace(s)
}
func reportNumber(s string) string {
	if strings.Trim(s, "0123456789.-") != "" {
		return s
	}
	parts := strings.SplitN(s, ".", 2)
	a := parts[0]
	start := 0
	if strings.HasPrefix(a, "-") {
		start = 1
	}
	for i := len(a) - 3; i > start; i -= 3 {
		a = a[:i] + "," + a[i:]
	}
	if len(parts) == 2 {
		return a + "." + parts[1]
	}
	return a
}
func stat(v []float64) string {
	if len(v) == 0 {
		return "미측정"
	}
	x := append([]float64{}, v...)
	sort.Float64s(x)
	return fmt.Sprintf("%.3f", x[len(x)/2])
}
func median(v []float64) float64 {
	x := append([]float64{}, v...)
	sort.Float64s(x)
	if len(x) == 0 {
		return 0
	}
	return x[len(x)/2]
}
func measureRows(env Envelope) []Measurement {
	var rows []Measurement
	_ = json.Unmarshal(env.Rows, &rows)
	return rows
}
func values(rows []Measurement, caseID string, fn func(Measurement) float64) []float64 {
	v := []float64{}
	for _, r := range rows {
		if r.Run > 0 && (caseID == "" || r.Case == caseID) {
			v = append(v, fn(r))
		}
	}
	return v
}

// 공식 run 순서를 유지하며 대표값의 근거를 별도 문서에 보존합니다.
func reportDetails(envs map[int]Envelope) string {
	var out strings.Builder
	out.WriteString("# 실험 결과 상세값\n\n본문은 median을 비교합니다. 이 부록은 공식 run 1~5와 min–max를 보존합니다. 시간 단위는 ms이며 warm-up은 제외합니다. 암호화 실험의 값은 1,000회 묶음을 나눈 workload 1회 비용입니다. RPC·방문 수·gas·메모리·receipt는 원본 output/experiments/experiment-N.json에 있습니다.\n\n")
	type sample struct {
		run   int
		value float64
	}
	for exp := 1; exp <= 8; exp++ {
		groups := map[string][]sample{}
		add := func(key string, run int, value float64) {
			if run > 0 {
				groups[key] = append(groups[key], sample{run, value})
			}
		}
		for _, r := range measureRows(envs[exp]) {
			// Measurement 이외의 JSON shape는 각 실험의 typed rows로 처리합니다.
			if exp == 4 || exp == 7 || exp == 8 {
				break
			}
			for _, m := range []struct {
				name  string
				value float64
			}{{"E2E", r.E2EMS}, {"witness", r.WitnessMS}, {"Prove", r.ProveMS}, {"transaction", r.TransactionMS}} {
				add(r.Case+" / "+r.Profile+" / "+m.name, r.Run, m.value)
			}
		}
		for _, r := range envs[exp].Routes {
			add(r.Route+" / 경로 E2E", r.Run, r.TotalMS)
		}
		for _, r := range envs[exp].DPP {
			add("DPPClaim / 전체", r.Run, r.TotalMS)
			add("DPPClaim / RPC", r.Run, r.RPCMS)
			add("DPPClaim / 로컬", r.Run, r.LocalMS)
		}
		if exp == 4 {
			var rows []AuditRow
			json.Unmarshal(envs[exp].Rows, &rows)
			for _, r := range rows {
				add(r.Case+" / "+r.Operation, r.Run, r.TotalMS)
			}
		}
		if exp == 7 || exp == 8 {
			var rows []CryptoRow
			json.Unmarshal(envs[exp].Rows, &rows)
			for _, r := range rows {
				add(r.Case, r.Run, r.TotalMS/float64(r.InnerIterations))
			}
		}
		fmt.Fprintf(&out, "## 실험 %d\n\n| case / 측정 | Run 1 | Run 2 | Run 3 | Run 4 | Run 5 | Median | Min | Max |\n|---|---:|---:|---:|---:|---:|---:|---:|---:|\n", exp)
		for _, key := range sortedKeys(groups) {
			rs := groups[key]
			sort.Slice(rs, func(i, j int) bool { return rs[i].run < rs[j].run })
			fmt.Fprintf(&out, "| %s ", key)
			var values []float64
			for _, r := range rs {
				fmt.Fprintf(&out, "| %.6f ", r.value)
				values = append(values, r.value)
			}
			sort.Float64s(values)
			fmt.Fprintf(&out, "| %.6f | %.6f | %.6f |\n", median(values), values[0], values[len(values)-1])
		}
		out.WriteString("\n")
	}
	return out.String()
}
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return naturalKey(keys[i]) < naturalKey(keys[j]) })
	return keys
}
func naturalKey(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] >= '0' && s[i] <= '9' {
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(s[i:j])
			fmt.Fprintf(&b, "%09d", n)
			i = j
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
