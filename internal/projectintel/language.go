package projectintel

// CATEGORY: language.go — detecção de linguagens.
// Regras de extensão + config files definitivos (tsconfig.json, go.mod,
// Cargo.toml, pyproject.toml, ...). As regras vivem em languageRules; a
// confiança é gradeada por evidência (config + contagem de arquivos).

import (
	"fmt"
	"strings"
)

type langRule struct {
	name     string
	exts     []string
	configs  []string // root-level file names that are definitive
	patterns []string // glob relative to root that are definitive
	notes    string
}

var languageRules = []langRule{
	{name: "TypeScript", exts: []string{".ts", ".tsx", ".mts", ".cts"}, configs: []string{"tsconfig.json"}, patterns: []string{"tsconfig.*.json"}, notes: "tsconfig.json + arquivos .ts/.tsx"},
	{name: "Go", exts: []string{".go"}, configs: []string{"go.mod", "go.work"}, notes: "go.mod + arquivos .go"},
	{name: "Rust", exts: []string{".rs"}, configs: []string{"Cargo.toml"}, notes: "Cargo.toml + arquivos .rs"},
	{name: "Python", exts: []string{".py"}, configs: []string{"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", "setup.cfg", "Pipfile.lock"}, notes: "pyproject.toml/setup.py + arquivos .py"},
	{name: "JavaScript", exts: []string{".js", ".jsx", ".mjs", ".cjs"}, notes: "arquivos .js/.jsx (sem config JS-definitivo)"},
	{name: "Java", exts: []string{".java"}, configs: []string{"pom.xml"}, patterns: []string{"gradle.properties"}, notes: "pom.xml + arquivos .java"},
	{name: "Kotlin", exts: []string{".kt", ".kts"}, patterns: []string{"build.gradle.kts"}, notes: "build.gradle.kts + arquivos .kt"},
	{name: "C#", exts: []string{".cs"}, patterns: []string{"*.csproj"}, notes: "*.csproj + arquivos .cs"},
	{name: "C/C++", exts: []string{".c", ".cpp", ".cc", ".h", ".hpp", ".cxx"}, configs: []string{"CMakeLists.txt"}, notes: "CMakeLists.txt + arquivos .c/.cpp"},
	{name: "PHP", exts: []string{".php"}, configs: []string{"composer.json"}, notes: "composer.json + arquivos .php"},
	{name: "Ruby", exts: []string{".rb"}, configs: []string{"Gemfile"}, notes: "Gemfile + arquivos .rb"},
	{name: "Swift", exts: []string{".swift"}, configs: []string{"Package.swift"}, notes: "Package.swift + arquivos .swift"},
}

// DetectLanguages returns the languages detected in root by file evidence.
func DetectLanguages(root string) []Tech { return detectLanguages(root, inspect(root)) }

func detectLanguages(root string, insp *inspection) []Tech {
	out := []Tech{}
	for _, r := range languageRules {
		var ev []Signal
		cfg := false
		for _, c := range r.configs {
			if insp.rootFileSet[c] {
				cfg = true
				ev = append(ev, fileSig(c, c))
			}
		}
		for _, pat := range r.patterns {
			for name := range globAt(root, pat) {
				cfg = true
				ev = append(ev, fileSig(name, name))
			}
		}
		cnt := 0
		for _, e := range r.exts {
			cnt += insp.counts[e]
		}
		if cnt > 0 {
			ev = append(ev, countSig(fmt.Sprintf("%d %s file(s)", cnt, strings.Join(r.exts, "/"))))
		}

		var conf string
		switch {
		case cfg && cnt >= highFileCount:
			conf = confHigh
		case cfg:
			conf = confMedium
		case cnt >= mediumFileCount:
			conf = confMedium
		case cnt > 0:
			conf = confLow
		}
		if conf != "" {
			out = append(out, tech(r.name, "language", conf, ev, r.notes))
		}
	}
	return out
}
