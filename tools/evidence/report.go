package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Evidence to cała zawartość pakietu poza surowymi wynikami przebiegów. Dla tego samego commitu i tych samych
// danych testowych różni się tylko polem Generated (EVD-004).
type Evidence struct {
	Generated   string         `json:"generated"`
	Commit      string         `json:"commit"`
	Dirty       bool           `json:"dirty"`
	GoVersion   string         `json:"go_version"`
	Platform    string         `json:"platform"`
	Python      string         `json:"python"`
	StreamMB    int            `json:"stream_mb,omitempty"`
	Candidate   string         `json:"candidate"`
	Criteria    []Criterion    `json:"criteria"`
	Runs        []*TestRun     `json:"runs"`
	PythonTests PythonRun      `json:"python_tests"`
	Traces      []Trace        `json:"traces"`
	UnknownRefs []Ref          `json:"unknown_refs,omitempty"`
	W3C         []W3CRow       `json:"w3c"`
	W3CExcluded []W3CExclusion `json:"w3c_excluded"`
	W3CFailing  []NamedCase    `json:"w3c_failing"`
	Ratchets    []Ratchet      `json:"ratchets"`
	Fuzz        []FuzzRun      `json:"fuzz"`
	FuzzHours   float64        `json:"fuzz_hours_counted"`
	Sources     []SourceSet    `json:"sources"`
	SourcesSHA  string         `json:"sources_json_sha256"`
	FetchVerify FetchVerify    `json:"fetch_verify"`
	ADRs        []ADR          `json:"adrs"`
	Differences []DiffSection  `json:"differences"`
	Files       []string       `json:"files"`
}

// PythonRun to wynik testów tools/fetch w Pythonie.
type PythonRun struct {
	Command string `json:"command"`
	Ran     int    `json:"ran"`
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
}

// FetchVerify to wynik `tools/fetch -verify`: czy pobrane dane mają sumy z manifestu.
type FetchVerify struct {
	Command string   `json:"command"`
	OK      bool     `json:"ok"`
	Lines   []string `json:"lines"`
}

// Criterion to jeden wiersz tabeli K1–K9.
type Criterion struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Target string `json:"target"`
	// Status: met, not_met, pending.
	Status string `json:"status"`
	Source string `json:"source"`
}

// Inputs to dane, z których liczymy kryteria.
type Inputs struct {
	Runs         []*TestRun
	PythonTests  PythonRun
	Traces       []Trace
	UnknownRefs  []Ref
	W3C          []W3CRow
	W3CFailing   []NamedCase
	W3CRatchet   *Ratchet
	FuzzHours    time.Duration
	Measurements *Measurements
	DiffTotal    int
	DiffUncat    int
	DiffStamp    string
	DiffFound    bool
	FetchVerify  FetchVerify
	StreamMB     int
}

const (
	pkgXSD    = "github.com/polestorm/goxsdgo/xsd"
	pkgSchema = "github.com/polestorm/goxsdgo/internal/schema"
)

var (
	mfSchemasRe   = regexp.MustCompile(`MF schemas compiled: (\d+)/(\d+)`)
	mfInstancesRe = regexp.MustCompile(`with the expected verdict(?: and errors)?: (\d+)/(\d+)`)
	reqCoveredRe  = regexp.MustCompile(`(\d+) requirement\(s\), (\d+) referenced`)
	streamPeakRe  = regexp.MustCompile(`validated (\d+) MB .*peak heap\+stack in use ([0-9.]+) MB`)
)

func (in Inputs) result(key string) string {
	for _, r := range in.Runs {
		if a := r.Result(key); a != "" {
			return a
		}
	}
	return ""
}

func (in Inputs) output(key string) []string {
	for _, r := range in.Runs {
		if o := r.Output(key); len(o) > 0 {
			return o
		}
	}
	return nil
}

// buildCriteria liczy tabelę K1–K9 (EVD-003). Wartości spoza `go test` biorą się z docs/measurements.json,
// z dziennika fuzzingu i z docs/differences-libxml2.md.
func buildCriteria(in Inputs) []Criterion {
	m := in.Measurements
	var out []Criterion

	// K1
	k1 := Criterion{ID: "K1", Name: "zestaw W3C XSD 1.0", Target: "100% przypadków w zakresie; każde wyłączenie i różnica opisane w docs/differences.md",
		Source: "TestW3CSuite (xsd), testdata/w3c/xsdtests.ratchet, internal/w3c.ExcludedGroups, docs/differences.md"}
	var all *W3CRow
	for i := range in.W3C {
		if in.W3C[i].Contributor == "all" {
			all = &in.W3C[i]
		}
	}
	switch res := in.result(pkgXSD + ".TestW3CSuite"); {
	case res == "skip" || all == nil:
		k1.Value, k1.Status = "brak pobranego zestawu W3C", "pending"
	default:
		failing := all.TotalIn - all.TotalOK
		named := 0
		for _, c := range in.W3CFailing {
			if c.Named {
				named++
			}
		}
		k1.Value = fmt.Sprintf("%d / %d (testy schematów %d / %d, testy dokumentów %d / %d); niezaliczone %d, nazwane w differences.md %d z %d",
			all.TotalOK, all.TotalIn, all.SchemaOK, all.SchemaIn, all.InstanceOK, all.InstanceIn, failing, named, len(in.W3CFailing))
		switch {
		case res != "pass":
			k1.Status = "not_met"
		case failing == 0 || (len(in.W3CFailing) == failing && named == failing):
			k1.Status = "met"
		default:
			k1.Status = "pending"
		}
	}
	out = append(out, k1)

	// K2
	sOK, sIn, sFound := findCount(in.output(pkgXSD+".TestSchemaCorpus"), mfSchemasRe)
	iOK, iIn, iFound := findCount(in.output(pkgXSD+".TestInstanceCorpus"), mfInstancesRe)
	k2 := Criterion{ID: "K2", Name: "korpus MF", Target: "wszystkie schematy się kompilują; każdy dokument z werdyktem z expected.json",
		Source: "TestSchemaCorpus, TestInstanceCorpus (xsd)"}
	switch {
	case !sFound || !iFound || in.result(pkgXSD+".TestSchemaCorpus") == "skip" || in.result(pkgXSD+".TestInstanceCorpus") == "skip":
		k2.Value, k2.Status = "brak pobranych danych MF (np. CI bez dostępu do epuap.gov.pl)", "pending"
	default:
		k2.Value = fmt.Sprintf("schematy %d / %d, dokumenty %d / %d", sOK, sIn, iOK, iIn)
		k2.Status = "not_met"
		if sOK == sIn && iOK == iIn && in.result(pkgXSD+".TestSchemaCorpus") == "pass" && in.result(pkgXSD+".TestInstanceCorpus") == "pass" {
			k2.Status = "met"
		}
	}
	out = append(out, k2)

	// K3
	k3 := Criterion{ID: "K3", Name: "zgodność z libxml2", Target: fmt.Sprintf("ten sam werdykt ≥ %.1f%%, ten sam wiersz pierwszego błędu ≥ %.0f%%, każda różnica werdyktu opisana",
		m.K3.VerdictThresholdPercent, m.K3.LineThresholdPercent),
		Source: fmt.Sprintf("docs/measurements.json (pomiar %s, commit %s: %s), %s", m.K3.Date, m.K3.Commit, m.K3.Method, m.K3.DifferencesFile)}
	k3.Value = fmt.Sprintf("testy dokumentów W3C: werdykt %.2f%% z %d, wiersz %.1f%% z %d; testy schematów: werdykt %.1f%% z %d; korpus MF %s; korpus prywatny: werdykt %s, wiersz %s",
		m.K3.InstanceVerdictSamePercent, m.K3.InstanceComparable, m.K3.FirstErrorLineSamePercent, m.K3.FirstErrorLineCompared,
		m.K3.SchemaVerdictSamePercent, m.K3.SchemaComparable, m.K3.CorpusVerdictSame, m.K3.PrivateCorpusVerdictSame, m.K3.PrivateCorpusFirstErrorLineSame)
	if in.DiffFound {
		k3.Value += fmt.Sprintf("; rozbieżności opisane: %d, bez kategorii: %d (%s)", in.DiffTotal, in.DiffUncat, in.DiffStamp)
	}
	switch {
	case !in.DiffFound:
		k3.Status = "pending"
	case m.K3.InstanceVerdictSamePercent >= m.K3.VerdictThresholdPercent && m.K3.FirstErrorLineSamePercent >= m.K3.LineThresholdPercent && in.DiffUncat == 0:
		k3.Status = "met"
	default:
		k3.Status = "not_met"
	}
	out = append(out, k3)

	// K4
	k4 := Criterion{ID: "K4", Name: "wymagania", Target: "każde wymaganie ma test; testy przechodzą",
		Source: "requirements/*.md, odwołania REQ: w testach, TestRequirementsCovered (xsd), macierz śledzenia w tym pakiecie"}
	counts := map[string]int{}
	for _, t := range in.Traces {
		counts[t.Status]++
	}
	k4.Value = fmt.Sprintf("%d wymagań: przeszło %d, na żądanie %d, pominięte %d, bez testu %d, nieprzypisane %d, nie przeszło %d; odwołania do nieznanych wymagań: %d",
		len(in.Traces), counts["pass"], counts["manual"], counts["skip"], counts["missing"], counts["unresolved"], counts["fail"], len(in.UnknownRefs))
	switch {
	case counts["fail"] > 0 || counts["missing"] > 0 || len(in.UnknownRefs) > 0 || in.result(pkgXSD+".TestRequirementsCovered") == "fail":
		k4.Status = "not_met"
	case counts["skip"] > 0 || counts["unresolved"] > 0:
		k4.Status = "pending"
	default:
		k4.Status = "met"
	}
	out = append(out, k4)

	// K5
	hours := in.FuzzHours.Hours()
	path1, path2 := in.result(pkgXSD+".TestPathological"), in.result(pkgSchema+".TestPathological")
	k5 := Criterion{ID: "K5", Name: "odporność", Target: "fuzzing ≥ 24 h łącznie na kandydacie bez błędu; przypadki patologiczne liniowe",
		Source: fmt.Sprintf("docs/fuzzing.md (kandydat %s i commity bez zmian biblioteki), TestPathological (xsd, internal/schema)", m.Candidate.Commit)}
	k5.Value = fmt.Sprintf("fuzzing %s z 24 h; TestPathological: xsd %s, internal/schema %s", fmtHours(in.FuzzHours), orNone(path1), orNone(path2))
	switch {
	case path1 == "fail" || path2 == "fail":
		k5.Status = "not_met"
	case hours >= 24 && path1 == "pass" && path2 == "pass":
		k5.Status = "met"
	default:
		k5.Status = "pending"
	}
	out = append(out, k5)

	// K6
	safe, stream := in.result(pkgXSD+".TestSafeByDefault"), in.result(pkgXSD+".TestStreamingMemory")
	k6 := Criterion{ID: "K6", Name: "bezpieczeństwo i pamięć", Target: fmt.Sprintf("bez sieci i encji zewnętrznych; plik %.0f MB w < %.0f MB pamięci", m.K6.SizeMB, m.K6.LimitMB),
		Source: fmt.Sprintf("TestSafeByDefault, TestStreamingMemory (xsd); docs/measurements.json (pomiar %s, commit %s: %s)", m.K6.Date, m.K6.Commit, m.K6.Method)}
	k6.Value = fmt.Sprintf("TestSafeByDefault %s; TestStreamingMemory %s", orNone(safe), orNone(stream))
	peak, size := m.K6.PeakMB, m.K6.SizeMB
	if in.StreamMB > 0 {
		for _, l := range in.output(pkgXSD + ".TestStreamingMemory") {
			if mm := streamPeakRe.FindStringSubmatch(l); mm != nil {
				fmt.Sscan(mm[1], &size)
				fmt.Sscan(mm[2], &peak)
				k6.Source += "; ten przebieg: GOXSDGO_STREAM_MB=" + fmt.Sprint(in.StreamMB)
			}
		}
	}
	k6.Value += fmt.Sprintf("; plik %.0f MB: szczyt %.1f MB", size, peak)
	switch {
	case safe == "fail" || stream == "fail" || peak >= m.K6.LimitMB:
		k6.Status = "not_met"
	case safe == "pass" && stream == "pass" && size >= 1000:
		k6.Status = "met"
	default:
		k6.Status = "pending"
	}
	out = append(out, k6)

	// K7
	k7 := Criterion{ID: "K7", Name: "wydajność", Target: fmt.Sprintf("nie wolniej niż %.0f× libxml2", m.K7.LimitRatio),
		Source: fmt.Sprintf("docs/measurements.json (pomiar %s, commit %s: %s)", m.K7.Date, m.K7.Commit, m.K7.Method)}
	k7.Value = fmt.Sprintf("syntetyczny JPK %.0f MB: %.2f×; korpus prywatny (%d plików, %.1f MB): %.2f×, największy plik %.2f×",
		m.K7.SyntheticSizeMB, m.K7.SyntheticRatio, m.K7.PrivateCorpusFiles, m.K7.PrivateCorpusSizeMB, m.K7.PrivateCorpusRatio, m.K7.PrivateCorpusLargestRatio)
	k7.Status = "not_met"
	worst := m.K7.SyntheticRatio
	for _, r := range []float64{m.K7.PrivateCorpusRatio, m.K7.PrivateCorpusLargestRatio} {
		if r > worst {
			worst = r
		}
	}
	if worst > 0 && worst <= m.K7.LimitRatio {
		k7.Status = "met"
	}
	out = append(out, k7)

	// K8
	k8 := Criterion{ID: "K8", Name: "odbiorca", Target: "testy odbiorcy przechodzą po przepięciu; każdy kod ma odpowiednik w compat",
		Source: "docs/measurements.json (k8), TestCompatTable (xsd), docs/consumer-migration.md"}
	compat := in.result(pkgXSD + ".TestCompatTable")
	k8.Value = fmt.Sprintf("TestCompatTable %s; testy %s: %s", orNone(compat), orDash(m.K8.Consumer), orDash(m.K8.Result))
	if m.K8.Branch != "" {
		k8.Value += fmt.Sprintf(" (gałąź %s, commit %s, %s)", m.K8.Branch, orDash(m.K8.Commit), orDash(m.K8.Date))
	}
	k8.Status = m.K8.Status
	if compat == "fail" {
		k8.Status = "not_met"
	}
	if k8.Status != "met" && k8.Status != "not_met" {
		k8.Status = "pending"
	}
	out = append(out, k8)

	// K9
	failed := 0
	for _, r := range in.Runs {
		failed += r.Failed + len(r.BuildFailed)
	}
	k9 := Criterion{ID: "K9", Name: "pakiet dowodów", Target: "wymagania → testy → wynik ostatniego przebiegu; dane testowe zgodne z manifestem",
		Source: "ten pakiet (tools/evidence, docs/evidence.md)"}
	k9.Value = fmt.Sprintf("przebiegi go test: %d nieudanych; testy Pythona: %s; tools/fetch -verify: %s; macierz: %d wymagań",
		failed, okWord(in.PythonTests.OK), okWord(in.FetchVerify.OK), len(in.Traces))
	switch {
	case failed > 0 || !in.PythonTests.OK || counts["fail"] > 0 || counts["missing"] > 0:
		k9.Status = "not_met"
	case !in.FetchVerify.OK:
		k9.Status = "pending"
	default:
		k9.Status = "met"
	}
	out = append(out, k9)
	return out
}

func fmtHours(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1f h", d.Hours())
}

func orNone(s string) string {
	if s == "" {
		return "brak w przebiegu"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func okWord(ok bool) string {
	if ok {
		return "ok"
	}
	return "błąd"
}

var statusPL = map[string]string{"met": "spełnione", "not_met": "**niespełnione**", "pending": "oczekuje",
	"pass": "przeszedł", "fail": "**nie przeszedł**", "skip": "pominięty (brak danych)", "missing": "**brak testu**",
	"unresolved": "**nieprzypisany**", "manual": "na żądanie (benchmark albo zmienna GOXSDGO_*)"}

// renderReport składa report.md (po polsku).
func renderReport(e *Evidence) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	dirty := ""
	if e.Dirty {
		dirty = " (ze zmianami w drzewie roboczym — pakiet nie dotyczy czystego commitu)"
	}
	p("# Pakiet dowodów goxsdgo\n\n")
	p("Commit `%s`%s; %s, %s; %s. Wygenerowano %s przez `go -C tools/evidence run . -root ../..` (docs/evidence.md).\n\n",
		e.Commit, dirty, e.GoVersion, e.Platform, e.Python, e.Generated)
	p("Kandydat do wydania: `%s`.\n\n", e.Candidate)

	p("## Kryteria zastąpienia (ADR 0001)\n\n| # | Kryterium | Wynik | Cel | Stan | Źródło |\n|---|---|---|---|---|---|\n")
	for _, c := range e.Criteria {
		p("| %s | %s | %s | %s | %s | %s |\n", c.ID, c.Name, cell(c.Value), cell(c.Target), statusPL[c.Status], cell(c.Source))
	}

	p("\n## Przebiegi testów\n\n| Przebieg | Polecenie | Przeszło | Nie przeszło | Pominięte |\n|---|---|---|---|---|\n")
	for _, r := range e.Runs {
		p("| %s | `%s` | %d | %d | %d |\n", r.Name, r.Command, r.Passed, r.Failed+len(r.BuildFailed), r.Skipped)
	}
	p("| Python | `%s` | %s | | |\n\n", e.PythonTests.Command, cell(e.PythonTests.Summary))
	for _, r := range e.Runs {
		for _, t := range r.FailedTests {
			p("- nie przeszedł: `%s`\n", t)
		}
		for _, t := range r.BuildFailed {
			p("- pakiet bez wyniku (błąd kompilacji?): `%s`\n", t)
		}
		for _, t := range r.SkippedTests {
			p("- pominięty: `%s`\n", t)
		}
	}

	p("\n## Macierz śledzenia (K4)\n\n")
	file := ""
	for _, t := range e.Traces {
		if t.File != file {
			file = t.File
			p("\n### %s\n\n| Wymaganie | Treść | Dowód | Wynik |\n|---|---|---|---|\n", file)
		}
		var ev []string
		for _, x := range t.Evidence {
			ev = append(ev, "`"+x+"`")
		}
		p("| %s | %s | %s | %s |\n", t.ID, cell(t.Text), strings.Join(ev, "<br>"), statusPL[t.Status])
	}
	if len(e.UnknownRefs) > 0 {
		p("\nOdwołania do nieistniejących wymagań:\n\n")
		for _, r := range e.UnknownRefs {
			p("- `%s` w `%s` (%s)\n", r.ID, r.File, orDash(r.Test))
		}
	}

	p("\n## Zestaw W3C (K1)\n\n| Dostawca | Testy schematów | Testy dokumentów | Razem |\n|---|---|---|---|\n")
	for _, r := range e.W3C {
		p("| %s | %d / %d | %d / %d | %d / %d |\n", r.Contributor, r.SchemaOK, r.SchemaIn, r.InstanceOK, r.InstanceIn, r.TotalOK, r.TotalIn)
	}
	if len(e.W3CExcluded) > 0 {
		p("\n| Wyłączone | Powód |\n|---|---|\n")
		for _, x := range e.W3CExcluded {
			p("| %d | %s |\n", x.Count, cell(x.Reason))
		}
	}
	if len(e.W3CFailing) > 0 {
		p("\nNiezaliczone przypadki w zakresie (z `testdata/w3c/xsdtests.ratchet`) i to, czy `docs/differences.md` je nazywa:\n\n| Przypadek | Nazwany w differences.md |\n|---|---|\n")
		for _, c := range e.W3CFailing {
			named := "tak"
			if !c.Named {
				named = "**nie**"
			}
			p("| `%s` | %s |\n", c.ID, named)
		}
	}

	p("\n## Zapadki\n\n| Plik | W zakresie | Przechodzi | Tryb | Na liście |\n|---|---|---|---|---|\n")
	for _, r := range e.Ratchets {
		p("| `%s` | %d | %d | %s | %d |\n", r.File, r.InScope, r.Passing, r.Mode, r.Listed)
	}

	p("\n## Fuzzing (K5)\n\nLiczone do K5: %s (kandydat `%s` i commity bez zmian kodu biblioteki; reguły w docs/fuzzing.md).\n\n", fmtHours(time.Duration(e.FuzzHours*float64(time.Hour))), e.Candidate)
	p("| Data | Commit | Pakiet | Fuzzer | Czas | Wykonania | Wynik | Liczony |\n|---|---|---|---|---|---|---|---|\n")
	for _, f := range e.Fuzz {
		counted := "nie"
		if f.Counted {
			counted = "tak"
		}
		p("| %s | `%s` | %s | `%s` | %s | %s | %s | %s |\n", f.Date, f.Commit, f.Package, f.Fuzzer, f.Duration, f.Execs, cell(f.Result), counted)
	}

	p("\n## Dane testowe\n\nManifest `testdata/sources.json`, SHA-256 `%s`.\n\n| Zestaw | Rodzaj | Katalog | Pliki | SHA-256 archiwum | SHA-256 drzewa | Tylko lokalnie |\n|---|---|---|---|---|---|---|\n", e.SourcesSHA)
	for _, s := range e.Sources {
		p("| %s | %s | `%s` | %d | %s | %s | %d |\n", s.Name, s.Kind, s.Dest, s.Files, orDash(s.SHA256), orDash(s.TreeSHA256), s.LocalOnly)
	}
	p("\n`%s`: %s\n\n```\n%s\n```\n", e.FetchVerify.Command, okWord(e.FetchVerify.OK), strings.Join(e.FetchVerify.Lines, "\n"))

	p("\n## Decyzje (docs/adr)\n\n| Plik | Tytuł | Status |\n|---|---|---|\n")
	for _, a := range e.ADRs {
		p("| `%s` | %s | %s |\n", a.File, cell(a.Title), cell(a.Status))
	}

	p("\n## Różnice (docs/differences.md)\n\n| Sekcja | Wiersze |\n|---|---|\n")
	for _, d := range e.Differences {
		p("| %s | %d |\n", cell(d.Title), d.Rows)
	}

	p("\n## Pliki pakietu\n\n")
	for _, f := range e.Files {
		p("- `%s`\n", f)
	}
	return b.String()
}

func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", "\\|")
}
