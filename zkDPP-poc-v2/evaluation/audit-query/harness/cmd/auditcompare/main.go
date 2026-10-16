// Bounded client experiment. The immutable fixtures are synthetic, not proved
// transactions. Native crypto/hash source copies are kept byte-identical.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/auditcrypto"
	zkhash "github.com/bighim/zkDPP/zkDPP-poc-v2/internal/core/hash"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

const fixtureSeed = "zkdpp-audit-client-compare-2026-10-11-v1"
const batchSize = 32

// Public deterministic test RNG. Do not use for production secrets.
type fixtureReader struct {
	seed    string
	counter uint64
	pending []byte
}

func (r *fixtureReader) Read(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		if len(r.pending) == 0 {
			var b [8]byte
			binary.BigEndian.PutUint64(b[:], r.counter)
			r.counter++
			h := sha256.Sum256(append([]byte(r.seed), b[:]...))
			r.pending = h[:]
		}
		k := copy(p, r.pending)
		p = p[k:]
		r.pending = r.pending[k:]
	}
	return n, nil
}
func enc(f fr.Element) string { b := f.Bytes(); return hex.EncodeToString(b[:]) }
func dec(s string) fr.Element {
	b, e := hex.DecodeString(s)
	must(e)
	var f fr.Element
	must(f.SetBytesCanonical(b))
	return f
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}

type WireCipher struct {
	X, Y string
	Data []string
}

func wire(ct auditcrypto.Ciphertext) WireCipher {
	w := WireCipher{X: enc(ct.R1.X), Y: enc(ct.R1.Y)}
	for _, v := range ct.Data {
		w.Data = append(w.Data, enc(v))
	}
	return w
}
func (w WireCipher) native() auditcrypto.Ciphertext {
	ct := auditcrypto.Ciphertext{R1: auditcrypto.Point{X: dec(w.X), Y: dec(w.Y)}}
	for _, s := range w.Data {
		ct.Data = append(ct.Data, dec(s))
	}
	return ct
}

type Record struct {
	ID      int
	Kind    string
	Outputs []string
	InputNF string
	Cipher  WireCipher
}
type Meta struct {
	ID   int
	Kind string
}
type Ref struct {
	CM string
	ID int
}
type Fixture struct {
	P, O             []Record
	Catalogue        []Meta
	Producers        map[string]int
	Parents          map[string]string // benchmark oracle only; never passed to an engine
	Seeds            []string
	Master           *big.Int
	FixtureHash      string
	KeyRecoveryNS    int64
	ParentFieldBytes int
}

func makeFixture(n int, layout string) *Fixture {
	if n%100 != 0 {
		panic("size must be a multiple of 100")
	}
	flows := n / 100
	rng := &fixtureReader{seed: fixtureSeed + fmt.Sprint(n) + layout}
	pk, shares, e := auditcrypto.TrustedSetup(rng)
	must(e)
	pkg, e := auditcrypto.PackageFromShares("client-comparison", pk, shares)
	must(e)
	t := time.Now()
	master, e := auditcrypto.RecoverMasterKey(pkg, shares[:2])
	must(e)
	keyNS := time.Since(t).Nanoseconds()
	f := &Fixture{Producers: map[string]int{}, Parents: map[string]string{}, Seeds: make([]string, flows), Master: master, KeyRecoveryNS: keyNS}
	cmOf := func(flow, step, slot int) fr.Element {
		return zkhash.MustHashToField(fmt.Sprintf("%s/cm/%d/%d/%d", fixtureSeed, flow, step, slot))
	}
	secret := zkhash.MustHashToField(fixtureSeed + "/owner")
	nfOf := func(cm fr.Element) fr.Element { return zkhash.Hash(zkhash.NullifierTag, secret, cm) }
	add := func(flow, step int) {
		id := len(f.P) + 1
		kind := "Split"
		if step == 0 {
			kind = "Entry"
		}
		outputs := []fr.Element{cmOf(flow, step, 0)}
		if step > 0 {
			outputs = append(outputs, cmOf(flow, step, 1))
		}
		outputNFs := []fr.Element{}
		refs := []string{}
		for _, cm := range outputs {
			key := enc(cm)
			if f.Producers[key] != 0 {
				panic("duplicate CM")
			}
			f.Producers[key] = id
			refs = append(refs, key)
			outputNFs = append(outputNFs, nfOf(cm))
		}
		var parents []fr.Element
		inputNF := ""
		if step > 0 {
			parent := cmOf(flow, step-1, 0)
			parents = []fr.Element{parent}
			inputNF = enc(nfOf(parent))
			for _, cm := range refs {
				f.Parents[cm] = enc(parent)
			}
			f.ParentFieldBytes += 32
		}
		pMsg := append(append([]fr.Element{}, parents...), outputNFs...)
		r, e := auditcrypto.RandomScalar(rng)
		must(e)
		pCT, e := auditcrypto.EncryptWithMasterPublicKey(pk, pMsg, r)
		must(e)
		r, e = auditcrypto.RandomScalar(rng)
		must(e)
		oCT, e := auditcrypto.EncryptWithMasterPublicKey(pk, outputNFs, r)
		must(e)
		// Exact ciphertext-vector and public-input correspondence checks are outside timing.
		pPlain, e := auditcrypto.DecryptWithMasterKey(master, pCT)
		must(e)
		oPlain, e := auditcrypto.DecryptWithMasterKey(master, oCT)
		must(e)
		if len(pPlain) != len(pMsg) || len(oPlain) != len(outputNFs) {
			panic("shape")
		}
		for i := range pMsg {
			if !pPlain[i].Equal(&pMsg[i]) {
				panic("P encryption mismatch")
			}
		}
		for i := range outputNFs {
			if !oPlain[i].Equal(&outputNFs[i]) {
				panic("O encryption mismatch")
			}
		}
		f.P = append(f.P, Record{id, kind, refs, inputNF, wire(pCT)})
		f.O = append(f.O, Record{id, kind, refs, inputNF, wire(oCT)})
		f.Catalogue = append(f.Catalogue, Meta{id, kind})
		if step == 99 {
			f.Seeds[flow] = refs[0]
		}
	}
	if layout == "contiguous" {
		for flow := 0; flow < flows; flow++ {
			for step := 0; step < 100; step++ {
				add(flow, step)
			}
		}
	} else if layout == "interleaved" {
		for step := 0; step < 100; step++ {
			for flow := 0; flow < flows; flow++ {
				add(flow, step)
			}
		}
	} else {
		panic("layout")
	}
	// Verify causal, one-consumption history and unique future identifiers.
	nfs := map[string]string{}
	spent := map[string]bool{}
	for _, r := range f.O {
		plain, e := auditcrypto.DecryptWithMasterKey(master, r.Cipher.native())
		must(e)
		if r.Kind != "Entry" {
			parent, ok := nfs[r.InputNF]
			if !ok || spent[r.InputNF] || f.Producers[parent] >= r.ID {
				panic("invalid causal history")
			}
			spent[r.InputNF] = true
			for _, cm := range r.Outputs {
				if f.Parents[cm] != parent {
					panic("oracle/public input mismatch")
				}
			}
		}
		for j, token := range plain {
			k := enc(token)
			if _, ok := nfs[k]; ok {
				panic("duplicate NF")
			}
			nfs[k] = r.Outputs[j]
		}
	}
	b, e := json.Marshal(struct{ P, O []Record }{f.P, f.O})
	must(e)
	h := sha256.Sum256(b)
	f.FixtureHash = hex.EncodeToString(h[:])
	return f
}

// Both local and HTTP engines see exactly the same public interface. The oracle
// parent map and decryption key are never available to the record server.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
}
type Backend struct {
	Records   []Record
	Catalogue []Meta
	Producers map[string]int
	WireSizes []int
}

func newBackend(records []Record, f *Fixture) *Backend {
	b := &Backend{Records: records, Catalogue: f.Catalogue, Producers: f.Producers}
	for _, r := range records {
		data, e := json.Marshal(r)
		must(e)
		b.WireSizes = append(b.WireSizes, len(data))
	}
	return b
}
func envelopeBytes(resultBytes int) int {
	return resultBytes + len(`{"jsonrpc":"2.0","id":1,"result":}`)
}
func (b *Backend) dispatch(q Request) Response {
	var result any
	switch q.Method {
	case "catalogue":
		result = b.Catalogue
	case "producerOf":
		var key string
		must(json.Unmarshal(q.Params, &key))
		result = b.Producers[key]
	case "getAuditRecords":
		var ids []int
		must(json.Unmarshal(q.Params, &ids))
		records := make([]Record, 0, len(ids))
		for _, id := range ids {
			if id < 1 || id > len(b.Records) {
				panic("id")
			}
			records = append(records, b.Records[id-1])
		}
		result = records
	default:
		panic("unknown method")
	}
	data, e := json.Marshal(result)
	must(e)
	return Response{"2.0", q.ID, data}
}
func (b *Backend) serve(w http.ResponseWriter, r *http.Request) {
	data, e := io.ReadAll(r.Body)
	must(e)
	var q Request
	must(json.Unmarshal(data, &q))
	response := b.dispatch(q)
	encoded, e := json.Marshal(response)
	must(e)
	w.Header().Set("Content-Type", "application/json")
	_, e = w.Write(encoded)
	must(e)
}

type Metrics struct{ Decryptions, Fields, RecordReads, ProducerReads, Requests, ResponseBytes int }

func (m Metrics) minus(a Metrics) Metrics {
	return Metrics{m.Decryptions - a.Decryptions, m.Fields - a.Fields, m.RecordReads - a.RecordReads, m.ProducerReads - a.ProducerReads, m.Requests - a.Requests, m.ResponseBytes - a.ResponseBytes}
}

type Source struct {
	Backend *Backend
	URL     string
	Client  *http.Client
	M       Metrics
}

func (s *Source) call(method string, params any, out any) {
	data, e := json.Marshal(params)
	must(e)
	q := Request{"2.0", 1, method, data}
	s.M.Requests++
	if s.URL == "" {
		response := s.Backend.dispatch(q)
		s.M.ResponseBytes += envelopeBytes(len(response.Result))
		must(json.Unmarshal(response.Result, out))
		return
	}
	body, e := json.Marshal(q)
	must(e)
	response, e := s.Client.Post(s.URL, "application/json", bytes.NewReader(body))
	must(e)
	received, e := io.ReadAll(response.Body)
	must(e)
	must(response.Body.Close())
	if response.StatusCode != 200 {
		panic("HTTP status")
	}
	s.M.ResponseBytes += len(received)
	var rpc Response
	must(json.Unmarshal(received, &rpc))
	must(json.Unmarshal(rpc.Result, out))
}
func (s *Source) catalogue() []Meta { var out []Meta; s.call("catalogue", nil, &out); return out }
func (s *Source) producer(cm string) int {
	var id int
	if s.URL == "" {
		id = s.Backend.Producers[cm]
		s.M.Requests++
		s.M.ResponseBytes += envelopeBytes(len(strconv.Itoa(id)))
	} else {
		s.call("producerOf", cm, &id)
	}
	s.M.ProducerReads++
	if id == 0 {
		panic("missing producer")
	}
	return id
}
func (s *Source) records(ids []int) []Record {
	var out []Record
	if s.URL == "" {
		resultBytes := 2
		for j, id := range ids {
			out = append(out, s.Backend.Records[id-1])
			resultBytes += s.Backend.WireSizes[id-1]
			if j > 0 {
				resultBytes++
			}
		}
		s.M.Requests++
		s.M.ResponseBytes += envelopeBytes(resultBytes)
	} else {
		s.call("getAuditRecords", ids, &out)
	}
	s.M.RecordReads += len(ids)
	return out
}

type Engine struct {
	Strategy  string
	Src       *Source
	Catalogue []Meta
	Master    *big.Int
	Records   map[int]Record
	Producer  map[string]int
	Plain     map[int][]fr.Element
	Reverse   map[string]Ref
	ParentCM  map[int]string
}

func newEngine(strategy string, src *Source, cat []Meta, master *big.Int) *Engine {
	return &Engine{Strategy: strategy, Src: src, Catalogue: cat, Master: master, Records: map[int]Record{}, Producer: map[string]int{}, Plain: map[int][]fr.Element{}, Reverse: map[string]Ref{}, ParentCM: map[int]string{}}
}
func (e *Engine) load(ids []int) {
	missing := []int{}
	for _, id := range ids {
		if _, ok := e.Records[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return
	}
	for _, r := range e.Src.records(missing) {
		want := 2
		if r.Kind == "Entry" {
			want = 1
		}
		fields := want
		if e.Strategy == "parents" && r.Kind != "Entry" {
			fields++
		}
		if len(r.Outputs) != want || len(r.Cipher.Data) != fields || r.ID < 1 {
			panic("record shape")
		}
		e.Records[r.ID] = r
		for _, cm := range r.Outputs {
			if old := e.Producer[cm]; old != 0 && old != r.ID {
				panic("producer conflict")
			}
			e.Producer[cm] = r.ID
		}
	}
}
func (e *Engine) record(id int) Record { e.load([]int{id}); return e.Records[id] }
func (e *Engine) producer(cm string) int {
	if id := e.Producer[cm]; id != 0 {
		return id
	}
	id := e.Src.producer(cm)
	e.Producer[cm] = id
	return id
}
func (e *Engine) decrypt(id int) []fr.Element {
	if v, ok := e.Plain[id]; ok {
		return v
	}
	r := e.record(id)
	v, err := auditcrypto.DecryptWithMasterKey(e.Master, r.Cipher.native())
	must(err)
	e.Src.M.Decryptions++
	e.Src.M.Fields += len(v)
	e.Plain[id] = v
	if e.Strategy == "parents" && r.Kind != "Entry" {
		e.ParentCM[id] = enc(v[0])
	} else if e.Strategy != "parents" {
		for j, nf := range v {
			key := enc(nf)
			ref := Ref{r.Outputs[j], r.ID}
			if old, ok := e.Reverse[key]; ok && old != ref {
				panic("NF collision")
			}
			e.Reverse[key] = ref
		}
	}
	return v
}
func (e *Engine) prepare() {
	if e.Strategy != "full_map" {
		return
	}
	for start := 0; start < len(e.Catalogue); start += batchSize {
		stop := start + batchSize
		if stop > len(e.Catalogue) {
			stop = len(e.Catalogue)
		}
		ids := []int{}
		for _, m := range e.Catalogue[start:stop] {
			ids = append(ids, m.ID)
		}
		e.load(ids)
		for _, id := range ids {
			e.decrypt(id)
		}
	}
}
func (e *Engine) resolve(nf string, before int) Ref {
	if ref, ok := e.Reverse[nf]; ok {
		if ref.ID >= before {
			panic("noncausal map")
		}
		return ref
	}
	if e.Strategy == "full_map" {
		panic("prepared map missing NF")
	}
	candidates := []int{}
	for i := len(e.Catalogue) - 1; i >= 0; i-- {
		id := e.Catalogue[i].ID
		if id < before {
			if _, done := e.Plain[id]; !done {
				candidates = append(candidates, id)
			}
		}
	}
	for start := 0; start < len(candidates); start += batchSize {
		stop := start + batchSize
		if stop > len(candidates) {
			stop = len(candidates)
		}
		ids := candidates[start:stop]
		e.load(ids)
		for _, id := range ids {
			e.decrypt(id)
			if ref, ok := e.Reverse[nf]; ok {
				return ref
			}
		}
	}
	panic("NF not found")
}
func (e *Engine) trace(seed string) []string {
	current := Ref{seed, e.producer(seed)}
	visited := map[string]bool{}
	out := []string{}
	for {
		if visited[current.CM] {
			panic("cycle")
		}
		visited[current.CM] = true
		out = append(out, current.CM)
		r := e.record(current.ID)
		found := false
		for _, cm := range r.Outputs {
			if cm == current.CM {
				found = true
			}
		}
		if !found {
			panic("producer/output mismatch")
		}
		if r.Kind == "Entry" {
			break
		}
		var parent Ref
		if e.Strategy == "parents" {
			e.decrypt(r.ID)
			cm := e.ParentCM[r.ID]
			parent = Ref{cm, e.producer(cm)}
		} else {
			parent = e.resolve(r.InputNF, r.ID)
		}
		if parent.ID >= r.ID {
			panic("noncausal parent")
		}
		current = parent
	}
	sort.Strings(out)
	return out
}

// Payload/accounting proxy excludes Go map bucket and allocator overhead.
func (e *Engine) logicalBytes() int {
	total := 0
	for _, r := range e.Records {
		total += 8 + len(r.Kind) + len(r.InputNF) + len(r.Outputs)*64 + 128 + len(r.Cipher.Data)*64
	}
	for _, v := range e.Plain {
		total += 8 + 32*len(v)
	}
	for k := range e.Producer {
		total += len(k) + 8
	}
	for k, v := range e.Reverse {
		total += len(k) + len(v.CM) + 8
	}
	for _, v := range e.ParentCM {
		total += 8 + len(v)
	}
	return total
}

type Row struct {
	Size              int     `json:"size"`
	Layout            string  `json:"layout"`
	Transport         string  `json:"transport"`
	Strategy          string  `json:"strategy"`
	Run               int     `json:"run"`
	Phase             string  `json:"phase"`
	Query             int     `json:"query"`
	DistinctSeeds     int     `json:"distinct_seeds"`
	SeedFlow          int     `json:"seed_flow"`
	NS                int64   `json:"ns"`
	CumulativeNS      int64   `json:"cumulative_ns"`
	CatalogueNS       int64   `json:"catalogue_ns"`
	Delta             Metrics `json:"delta"`
	Cumulative        Metrics `json:"cumulative"`
	CatalogueMetrics  Metrics `json:"catalogue_metrics"`
	Ancestors         int     `json:"ancestors"`
	Matched           bool    `json:"matched"`
	CacheRecords      int     `json:"cache_records"`
	CachePlain        int     `json:"cache_plain"`
	MapEntries        int     `json:"map_entries"`
	CacheLogicalBytes int     `json:"cache_logical_bytes"`
	RetainedHeapBytes int64   `json:"retained_heap_bytes,omitempty"`
}
type CaseInfo struct {
	Size                                 int
	Layout                               string
	FixtureSHA256                        string
	KeyRecoveryNS                        int64
	AddedParentFieldBytes                int
	PayloadPJSONBytes, PayloadOJSONBytes int
}
type Output struct {
	Created                          string
	GoVersion                        string
	GOOS                             string
	GOARCH                           string
	CPUs                             int
	FixtureSeed                      string
	BatchSize, MeasuredRuns, Warmups int
	Cases                            []CaseInfo
	Rows                             []Row
}

func truth(f *Fixture, seed string) []string {
	out := []string{}
	for seed != "" {
		out = append(out, seed)
		seed = f.Parents[seed]
	}
	sort.Strings(out)
	return out
}
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func runSession(f *Fixture, n int, layout, transport, strategy string, run int, client *http.Client, url string, backend *Backend) []Row {
	src := &Source{Backend: backend, Client: client, URL: url}
	t := time.Now()
	catalogue := src.catalogue()
	catalogueNS := time.Since(t).Nanoseconds()
	catalogueM := src.M
	src.M = Metrics{}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	e := newEngine(strategy, src, catalogue, f.Master)
	rows := []Row{}
	cumulative := int64(0)
	save := func(phase string, query, distinct, flow int, ns int64, delta Metrics, ancestors int, matched bool) {
		cumulative += ns
		rows = append(rows, Row{Size: n, Layout: layout, Transport: transport, Strategy: strategy, Run: run, Phase: phase, Query: query, DistinctSeeds: distinct, SeedFlow: flow, NS: ns, CumulativeNS: cumulative, CatalogueNS: catalogueNS, Delta: delta, Cumulative: src.M, CatalogueMetrics: catalogueM, Ancestors: ancestors, Matched: matched, CacheRecords: len(e.Records), CachePlain: len(e.Plain), MapEntries: len(e.Reverse), CacheLogicalBytes: e.logicalBytes()})
	}
	before := src.M
	t = time.Now()
	e.prepare()
	save("prepare", 0, 0, -1, time.Since(t).Nanoseconds(), src.M.minus(before), 0, true)
	sequence := []int{0, 0}
	for i := 1; i < len(f.Seeds); i++ {
		sequence = append(sequence, i)
	}
	sequence = append(sequence, len(f.Seeds)-1)
	distinct := 0
	for i, flow := range sequence {
		phase := "new_seed"
		if i == 0 {
			phase = "first"
			distinct++
		} else if i == 1 {
			phase = "repeat_same"
		} else if i == len(sequence)-1 {
			phase = "repeat_last"
		} else {
			distinct++
		}
		before = src.M
		t = time.Now()
		got := e.trace(f.Seeds[flow])
		ns := time.Since(t).Nanoseconds()
		matched := equal(got, truth(f, f.Seeds[flow]))
		if !matched || len(got) != 100 {
			panic("incorrect ancestor set")
		}
		save(phase, i+1, distinct, flow, ns, src.M.minus(before), len(got), matched)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	retained := int64(after.HeapAlloc) - int64(base.HeapAlloc)
	if retained < 0 {
		retained = 0
	}
	rows[len(rows)-1].RetainedHeapBytes = retained
	runtime.KeepAlive(e)
	return rows
}
func main() {
	path := flag.String("out", "raw.json", "JSON output")
	runs := flag.Int("runs", 5, "measured repetitions after 1 excluded warmup")
	sizes := flag.String("sizes", "100,1000", "100 or 100,1000")
	flag.Parse()
	counts := []int{100, 1000}
	if *sizes == "100" {
		counts = []int{100}
	} else if *sizes != "100,1000" {
		panic("bounded sizes only")
	}
	out := Output{Created: time.Now().Format(time.RFC3339), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPUs: runtime.NumCPU(), FixtureSeed: fixtureSeed, BatchSize: batchSize, MeasuredRuns: *runs, Warmups: 1}
	for _, n := range counts {
		for _, layout := range []string{"contiguous", "interleaved"} {
			f := makeFixture(n, layout)
			pJSON, e := json.Marshal(f.P)
			must(e)
			oJSON, e := json.Marshal(f.O)
			must(e)
			out.Cases = append(out.Cases, CaseInfo{n, layout, f.FixtureHash, f.KeyRecoveryNS, f.ParentFieldBytes, len(pJSON), len(oJSON)})
			pBackend := newBackend(f.P, f)
			oBackend := newBackend(f.O, f)
			pServer := httptest.NewServer(http.HandlerFunc(pBackend.serve))
			oServer := httptest.NewServer(http.HandlerFunc(oBackend.serve))
			for _, transport := range []string{"local_preloaded", "http_loopback"} {
				client := &http.Client{Transport: &http.Transport{MaxIdleConns: 4, MaxIdleConnsPerHost: 4}, Timeout: 30 * time.Second}
				// Warm both HTTP connections symmetrically, outside session timing.
				for _, server := range []*httptest.Server{pServer, oServer} {
					src := &Source{Client: client, URL: server.URL}
					_ = src.catalogue()
				}
				strategies := []string{"parents", "full_map", "lazy_map"}
				for run := 0; run <= *runs; run++ {
					for i := 0; i < len(strategies); i++ {
						strategy := strategies[(run+i)%len(strategies)]
						url := ""
						if transport == "http_loopback" {
							url = oServer.URL
							if strategy == "parents" {
								url = pServer.URL
							}
						}
						backend := oBackend
						if strategy == "parents" {
							backend = pBackend
						}
						rows := runSession(f, n, layout, transport, strategy, run, client, url, backend)
						out.Rows = append(out.Rows, rows...)
					}
					fmt.Fprintf(os.Stderr, "n=%d layout=%s transport=%s run=%d/%d validated\n", n, layout, transport, run, *runs)
				}
				client.CloseIdleConnections()
			}
			pServer.Close()
			oServer.Close()
		}
	}
	data, e := json.MarshalIndent(out, "", "  ")
	must(e)
	must(os.WriteFile(*path, append(data, '\n'), 0644))
	fmt.Fprintf(os.Stderr, "saved %s (%d phase rows, all matched)\n", *path, len(out.Rows))
}
