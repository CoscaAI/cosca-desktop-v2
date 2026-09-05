package projectintel

// CATEGORY: detector.go — orquestração e infraestrutura compartilhada.
// Contém o ponto de entrada Analyze, a passagem read-only única (inspect),
// helpers genéricos de evidência (Signal/Tech), leitura read-only de
// manifests (package.json, Makefile, Taskfile) e os cálculos derivados
// (config files, project types, capabilities). Não contém regras de detecção
// de nenhuma categoria específica — essas vivem nos seus próprios arquivos
// (language.go, framework.go, ...).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Constants / configuration for the read-only scanner
// ---------------------------------------------------------------------------

// excludedDirs are large or non-project directories that the extension-count
// walker must NEVER descend into (node_modules, .git, vendor, build outputs,
// language caches, venv, etc.). This keeps the scan cheap and deterministic and
// prevents "inflated" counts from third-party sources.
var excludedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"bin":          true,
	".next":        true,
	".nuxt":        true,
	".svelte-kit":  true,
	".cosca":       true, // includes .cosca/fallback and the institutional data
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".cache":       true,
	".pytest_cache": true,
	".mypy_cache":  true,
	".ruff_cache":  true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
	".turbo":       true,
	".serverless":  true,
	".terraform":   true,
	"coverage":     true,
	"tmp":          true,
	"temp":         true,
	".bundle":      true,
	"pods":         true,
	"Carthage":     true,
	"DerivedData":  true,
}

// maxScanDepth bounds the recursive walk so pathological nesting cannot cause a
// runaway scan.
const maxScanDepth = 14

// Confidence thresholds for extension-count based language detection.
const (
	highFileCount   = 5  // config present AND >= this many files => high
	mediumFileCount = 10 // no config but >= this many files => medium
)

// Confidence levels (string values used through the codebase).
const (
	confHigh   = "high"
	confMedium = "medium"
	confLow    = "low"
)

// maxReadSize caps any file content read. Detection is static, so only small
// config/manifest files are ever read; source files are only counted by
// extension (never read in full).
const maxReadSize = 1 << 20 // 1 MiB

// ---------------------------------------------------------------------------
// inference-time data (single lightweight pass over the root)
// ---------------------------------------------------------------------------

// inspection bundles the results of a single read-only pass over the root so
// each Detect function can answer "is X here?" cheaply and consistently.
type inspection struct {
	counts     map[string]int   // lowercase extension -> # of files under root (excluded dirs skipped)
	rootFiles  []string         // root-level file basenames
	rootFileSet map[string]bool // set form of rootFiles
	rootDirs   map[string]bool  // root-level directory basenames
	testGo     int              // # of *_test.go files
	testPy     int              // # of test_*.py files
	tfCount    int              // # of *.tf files
	mdCount    int              // # of .md files
}

// inspect performs one cheap, read-only walk over root. It never descends into
// excludedDirs and never reads file contents.
func inspect(root string) *inspection {
	i := &inspection{
		counts:     map[string]int{},
		rootFileSet: map[string]bool{},
		rootDirs:   map[string]bool{},
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return i
	}
	for _, e := range entries {
		if e.IsDir() {
			i.rootDirs[e.Name()] = true
		} else {
			i.rootFiles = append(i.rootFiles, e.Name())
			i.rootFileSet[e.Name()] = true
		}
	}

	var rec func(dir string, depth int)
	rec = func(dir string, depth int) {
		if depth > maxScanDepth {
			return
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			if e.IsDir() {
				// Exclude known-heavy dirs; also skip hidden dirs for source
				// counting (CI/docs dirs are handled by dedicated detect funcs).
				if excludedDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				rec(filepath.Join(dir, e.Name()), depth+1)
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != "" {
				i.counts[ext]++
			}
			switch {
			case strings.HasSuffix(name, "_test.go"):
				i.testGo++
			case (strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py")) && ext == ".py":
				i.testPy++
			case ext == ".tf":
				i.tfCount++
			case ext == ".md":
				i.mdCount++
			}
		}
	}
	rec(root, 0)
	return i
}

// globAt returns the set of BASE NAMES under root matching the given glob
// patterns (e.g. "vite.config.*", "tsconfig.*.json"). No error is fatal.
func globAt(root string, patterns ...string) map[string]bool {
	out := map[string]bool{}
	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, p))
		if err != nil {
			continue
		}
		for _, m := range matches {
			out[filepath.Base(m)] = true
		}
	}
	return out
}

// readSmallFile reads a small config file. Files larger than maxReadSize are
// skipped (static detection never needs a huge file's content).
func readSmallFile(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.Size() > maxReadSize {
		return "", fmt.Errorf("file too large: %s", path)
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// ---------------------------------------------------------------------------
// small constructors / helpers
// ---------------------------------------------------------------------------

func fileSig(name, rel string) Signal { return Signal{Kind: "file", Value: name, File: rel} }
func countSig(value string) Signal    { return Signal{Kind: "count", Value: value} }
func depSig(name string) Signal       { return Signal{Kind: "dependency", Value: name} }
func contentSig(value string) Signal  { return Signal{Kind: "content", Value: value} }

func tech(name, category, confidence string, ev []Signal, notes string) Tech {
	if ev == nil {
		ev = []Signal{}
	}
	return Tech{Name: name, Category: category, Confidence: confidence, Evidence: ev, Notes: notes}
}

func hasTech(list []Tech, name string) bool {
	for _, t := range list {
		if t.Name == name {
			return true
		}
	}
	return false
}

func firstTech(list []Tech) string {
	if len(list) == 0 {
		return ""
	}
	return list[0].Name
}

// appendTech appends t to list, merging evidence and upgrading confidence if t
// carries stronger evidence than an existing entry with the same name. It
// prevents duplicate Tech entries for the same technology.
func appendTech(list []Tech, t Tech) []Tech {
	for i := range list {
		if list[i].Name == t.Name {
			list[i].Evidence = append(list[i].Evidence, t.Evidence...)
			if t.Confidence == confHigh && list[i].Confidence != confHigh {
				list[i].Confidence = confHigh
			}
			return list
		}
	}
	return append(list, t)
}

// npmPackage is the subset of package.json we read (read-only).
type npmPackage struct {
	Name            string            `json:"name"`
	Bin             any               `json:"bin"`
	Main            string            `json:"main"`
	Module          string            `json:"module"`
	Exports         any               `json:"exports"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
}

// readPackage reads and parses package.json at root. Returns nil if absent or
// invalid (never a fatal error for detection).
func readPackage(root string) *npmPackage {
	path := filepath.Join(root, "package.json")
	content, err := readSmallFile(path)
	if err != nil {
		return nil
	}
	var pkg npmPackage
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}
	return &pkg
}

// packageDeps merges dependencies + devDependencies names into a set.
func packageDeps(pkg *npmPackage) map[string]bool {
	if pkg == nil {
		return nil
	}
	set := map[string]bool{}
	for k := range pkg.Dependencies {
		set[k] = true
	}
	for k := range pkg.DevDependencies {
		set[k] = true
	}
	return set
}

// depsContain reports whether any of the dependency names are present.
func depsContain(pkg *npmPackage, names ...string) string {
	if pkg == nil {
		return ""
	}
	set := packageDeps(pkg)
	for _, n := range names {
		if set[n] {
			return n
		}
	}
	return ""
}

// hasWorkspaces reports whether package.json declares a workspaces field
// (array or object). We only look for evidence; we do not attempt to parse it
// deeply.
func hasWorkspaces(pkg *npmPackage) bool {
	if pkg == nil || len(pkg.Workspaces) == 0 {
		return false
	}
	return true
}

// toolFromScript returns the first known build/test/lint tool referenced by a
// script string (evidence-based; never inferred from a folder name).
func toolFromScript(script string, tools []string) string {
	for _, t := range tools {
		if strings.Contains(script, t) {
			return t
		}
	}
	return ""
}

// readMakeTargets extracts target names from a Makefile (lines of the form
// "target: prereq"). It ignores phony dot-targets, pattern rules and
// assignments.
func readMakeTargets(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".") || strings.Contains(line, "=") {
			continue
		}
		if strings.Contains(line, ":") {
			target := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
			if target != "" && !strings.Contains(target, "$") {
				out = append(out, target)
			}
		}
	}
	return out
}

// readTaskNames extracts task keys from a Taskfile.yml (top-level indented
// keys). Light heuristic, evidence-only.
func readTaskNames(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSuffix(trimmed, ":")
			if name != "" && name == strings.TrimSpace(name) && !strings.Contains(name, " ") {
				out = append(out, name)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Analyze — the entry point that assembles a full ProjectProfile
// ---------------------------------------------------------------------------

// Analyze inspects the project root (read-only) and returns a ProjectProfile
// produced purely by file evidence. It never executes commands and never
// writes to the root. Errors are only reported for an invalid root path.
func Analyze(root string) (*ProjectProfile, error) {
	if root == "" {
		return nil, fmt.Errorf("root vazio")
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s não é um diretório", root)
	}

	insp := inspect(root)
	p := &ProjectProfile{
		Root:           root,
		Name:           filepath.Base(root),
		DetectedAt:     TimeNow(),
		Capabilities:   map[string]string{},
		Languages:      []Tech{},
		Frameworks:     []Tech{},
		PackageManagers: []Tech{},
		BuildTools:     []Tech{},
		TestTools:      []Tech{},
		Formatters:     []Tech{},
		Linters:        []Tech{},
		TypeCheckers:   []Tech{},
		Runtime:        []Tech{},
		Container:      []Tech{},
		Infrastructure: []Tech{},
		Database:       []Tech{},
		Monorepo:       []Tech{},
		CI:             []Tech{},
		Documentation:  []Tech{},
		Repository:     []Tech{},
		ConfigFiles:    []string{},
		Commands:       []DetectedCommand{},
		Evidence:       []Signal{},
	}

	p.Languages = detectLanguages(root, insp)
	p.Frameworks = detectFrameworks(root, insp)
	p.PackageManagers = detectPackageManagers(root, insp)
	p.BuildTools = detectBuildTools(root, insp)
	p.TestTools = detectTestTools(root, insp)
	p.Formatters = detectFormatters(root, insp)
	p.Linters = detectLinters(root, insp)
	p.TypeCheckers = detectTypeCheckers(root, insp)
	p.Runtime = detectRuntime(root, insp)
	p.Container = detectContainer(root, insp)
	p.Infrastructure = detectInfrastructure(root, insp)
	p.Database = detectDatabases(root, insp)
	p.Monorepo = detectMonorepo(root, insp)
	p.CI = detectCI(root, insp)
	p.Documentation = detectDocumentation(root, insp)
	p.Repository = detectRepository(root, insp)
	p.Commands = detectCommands(root, insp)

	p.ConfigFiles = detectConfigFiles(p)
	p.ProjectTypes = computeProjectTypes(root, insp, p)
	p.Capabilities = computeCapabilities(p)

	// consolidate the evidence for observability ("why detected").
	ev := []Signal{}
	collect := func(list []Tech) {
		for _, t := range list {
			ev = append(ev, t.Evidence...)
		}
	}
	for _, l := range allTechLists(p) {
		collect(l)
	}
	for _, c := range p.Commands {
		ev = append(ev, Signal{Kind: "command", Value: c.Name + " -> " + c.Cmd, File: c.Source})
	}
	p.Evidence = dedupeSignals(ev)

	return p, nil
}

// allTechLists returns every tech category slice on the profile (used for
// evidence consolidation and config-file collection).
func allTechLists(p *ProjectProfile) [][]Tech {
	return [][]Tech{
		p.Languages, p.Frameworks, p.PackageManagers, p.BuildTools, p.TestTools,
		p.Formatters, p.Linters, p.TypeCheckers, p.Runtime, p.Container,
		p.Infrastructure, p.Database, p.Monorepo, p.CI, p.Documentation,
		p.Repository,
	}
}

// dedupeSignals removes duplicate (kind+value+file) signals preserving order.
func dedupeSignals(ev []Signal) []Signal {
	seen := map[string]bool{}
	out := []Signal{}
	for _, s := range ev {
		key := s.Kind + "\x00" + s.Value + "\x00" + s.File
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// ---------------------------------------------------------------------------
// ConfigFiles / ProjectTypes / Capabilities / Evidence
// ---------------------------------------------------------------------------

// detectConfigFiles collects the config/manifest file basenames found across all
// detected Tech (file-kind evidence that is not a markdown doc).
func detectConfigFiles(p *ProjectProfile) []string {
	set := map[string]bool{}
	for _, list := range allTechLists(p) {
		for _, t := range list {
			for _, s := range t.Evidence {
				if s.Kind != "file" {
					continue
				}
				if strings.HasSuffix(strings.ToLower(s.Value), ".md") {
					continue
				}
				set[s.Value] = true
			}
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// computeProjectTypes derives coarse project shapes from what was detected.
func computeProjectTypes(root string, insp *inspection, p *ProjectProfile) []string {
	// Empty = truly empty root (no files, no commands, no tech).
	if len(insp.rootFiles) == 0 && len(p.Commands) == 0 && totalTech(p) == 0 {
		return []string{"empty"}
	}

	var types []string
	if hasTechAny(p.Monorepo, "pnpm workspace", "Turborepo", "Nx", "Lerna", "Go workspace", "Cargo workspace") {
		types = append(types, "monorepo")
	}
	for _, fw := range p.Frameworks {
		if fw.Category == "frontend" {
			types = append(types, "frontend")
			break
		}
	}
	for _, fw := range p.Frameworks {
		if fw.Category == "backend" {
			types = append(types, "backend")
			break
		}
	}
	if hasTech(p.Container, "Docker") || hasTech(p.Container, "Compose") {
		types = append(types, "service")
	}
	if isCLI(root, insp, p) {
		types = append(types, "cli")
	}
	if isLibrary(root, p, insp) {
		types = append(types, "library")
	}
	// docs-only: no language, but documentation present.
	if len(p.Languages) == 0 && len(p.Documentation) > 0 {
		types = append(types, "docs-only")
	}
	if len(types) == 0 && len(p.Languages) > 0 {
		types = append(types, "module")
	}
	return types
}

// isCLI reports CLI-evidence: a package.json "bin" field, or a cmd/ dir (Go),
// or a Rust binary target. Folder names are never the sole reason.
func isCLI(root string, insp *inspection, p *ProjectProfile) bool {
	if pkg := readPackage(root); pkg != nil {
		if pkg.Bin != nil {
			switch pkg.Bin.(type) {
			case string, map[string]any:
				return true
			}
		}
	}
	if insp.rootDirs["cmd"] {
		return true
	}
	if insp.rootDirs["src"] {
		// a src/ dir with Rust main.rs binary boundary supports CLI count, but
		// only combined with a Rust main binary declaration.
		if insp.rootFileSet["Cargo.toml"] {
			if content, err := readSmallFile(filepath.Join(root, "Cargo.toml")); err == nil {
				if strings.Contains(content, "[[bin]]") {
					return true
				}
			}
		}
	}
	return false
}

// isLibrary reports library-evidence: a package.json with main/module/exports
// and no bin, or a Go module without a cmd/ dir.
func isLibrary(root string, p *ProjectProfile, insp *inspection) bool {
	if pkg := readPackage(root); pkg != nil {
		hasBin := pkg.Bin != nil
		if (pkg.Main != "" || pkg.Module != "" || pkg.Exports != nil) && !hasBin {
			return true
		}
	}
	if insp.rootFileSet["go.mod"] && !insp.rootDirs["cmd"] {
		return true
	}
	if insp.rootFileSet["Cargo.toml"] {
		if c, err := readSmallFile(filepath.Join(root, "Cargo.toml")); err == nil {
			if strings.Contains(c, "[lib]") && !strings.Contains(c, "[[bin]]") {
				return true
			}
		}
	}
	return false
}

// computeCapabilities derives the tool map from detected tech.
func computeCapabilities(p *ProjectProfile) map[string]string {
	caps := map[string]string{}
	if t := firstTech(p.BuildTools); t != "" {
		caps["build"] = t
	}
	if t := firstTech(p.TestTools); t != "" {
		caps["test"] = t
	}
	if t := firstTech(p.Linters); t != "" {
		caps["lint"] = t
	}
	if t := firstTech(p.Formatters); t != "" {
		caps["format"] = t
	}
	if t := firstTech(p.TypeCheckers); t != "" {
		caps["typecheck"] = t
	}
	if t := firstTech(p.PackageManagers); t != "" {
		caps["package_manager"] = t
	}
	if hasTech(p.Container, "Docker") {
		caps["docker"] = "YES"
	}
	for _, fw := range p.Frameworks {
		if fw.Category == "frontend" {
			caps["framework"] = strings.ToLower(fw.Name)
			break
		}
	}
	return caps
}

// ---------------------------------------------------------------------------
// small utilities used above
// ---------------------------------------------------------------------------

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedBoolKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedFirst(m map[string]bool) string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func contentContains(file, marker string) bool {
	if marker == "" {
		return false
	}
	content, err := readSmallFile(file)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(content), strings.ToLower(marker))
}

func hasTechAny(list []Tech, names ...string) bool {
	for _, t := range list {
		for _, n := range names {
			if t.Name == n {
				return true
			}
		}
	}
	return false
}

func totalTech(p *ProjectProfile) int {
	n := 0
	for _, l := range allTechLists(p) {
		n += len(l)
	}
	return n
}
