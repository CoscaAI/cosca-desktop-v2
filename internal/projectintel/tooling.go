package projectintel

// CATEGORY: tooling.go — ferramentas de qualificação: test runners, formatters,
// linters e type checkers. Detectadas por config file, dependência declarada,
// contagem de arquivos de teste e conteúdo do manifest.

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Test tools
// ---------------------------------------------------------------------------

// DetectTestTools returns test runners evidenced by config/deps/files.
func DetectTestTools(root string) []Tech { return detectTestTools(root, inspect(root)) }

func detectTestTools(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) {
		out = appendTech(out, tech(name, "testtool", conf, ev, ""))
	}

	cfgGlobal := map[string]string{
		"vitest.config.*":     "vitest",
		"jest.config.*":       "jest",
		"playwright.config.*": "playwright",
		"cypress.config.*":    "cypress",
		"cypress.json":        "cypress",
	}
	// deterministic order
	for _, pat := range []string{"vitest.config.*", "jest.config.*", "playwright.config.*", "cypress.config.*", "cypress.json"} {
		for name := range globAt(root, pat) {
			add(cfgGlobal[pat], confHigh, fileSig(name, name))
		}
	}

	if pkg := readPackage(root); pkg != nil {
		for _, dep := range []string{"vitest", "jest", "@playwright/test", "cypress", "mocha", "jasmine", "ava", "karma"} {
			if d := depsContain(pkg, dep); d != "" {
				name := dep
				if dep == "@playwright/test" {
					name = "playwright"
				}
				add(name, confMedium, depSig(d))
			}
		}
	}

	if insp.rootFileSet["go.mod"] && insp.testGo > 0 {
		add("go", confHigh, []Signal{fileSig("go.mod", "go.mod"), countSig(fmt.Sprintf("%d *_test.go", insp.testGo))}...)
	}
	if insp.testPy > 0 {
		add("pytest", confMedium, countSig(fmt.Sprintf("%d test_*.py", insp.testPy)))
	}
	if insp.rootFileSet["Cargo.toml"] {
		add("cargo", confMedium, fileSig("Cargo.toml", "Cargo.toml"))
	}
	return out
}

// ---------------------------------------------------------------------------
// Formatters
// ---------------------------------------------------------------------------

// DetectFormatters returns code formatters evidenced by config/deps.
func DetectFormatters(root string) []Tech { return detectFormatters(root, inspect(root)) }

func detectFormatters(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "formatter", conf, ev, "")) }

	if m := globAt(root, ".prettierrc*", "prettier.config.*"); len(m) > 0 {
		add("prettier", confHigh, fileSig(sortedFirst(m), sortedFirst(m)))
	}
	if insp.rootFileSet["biome.json"] {
		add("biome", confHigh, fileSig("biome.json", "biome.json"))
	}
	if insp.rootFileSet["go.mod"] {
		add("gofmt", confMedium, fileSig("go.mod", "go.mod"))
	}
	if insp.rootFileSet["Cargo.toml"] {
		add("rustfmt", confMedium, fileSig("Cargo.toml", "Cargo.toml"))
	}
	if py, err := readSmallFile(filepath.Join(root, "pyproject.toml")); err == nil {
		if strings.Contains(py, "black") {
			add("black", confHigh, contentSig("black em pyproject.toml"))
		}
		if strings.Contains(py, "ruff") {
			add("ruff", confHigh, contentSig("ruff em pyproject.toml"))
		}
	}
	if insp.rootFileSet[".editorconfig"] {
		add("editorconfig", confLow, fileSig(".editorconfig", ".editorconfig"))
	}
	if pkg := readPackage(root); pkg != nil {
		if d := depsContain(pkg, "prettier"); d != "" {
			if !hasTech(out, "prettier") {
				add("prettier", confMedium, depSig(d))
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Linters
// ---------------------------------------------------------------------------

// DetectLinters returns linters evidenced by config/deps.
func DetectLinters(root string) []Tech { return detectLinters(root, inspect(root)) }

func detectLinters(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "linter", conf, ev, "")) }

	if m := globAt(root, "eslint.config.*", ".eslintrc*"); len(m) > 0 {
		add("eslint", confHigh, fileSig(sortedFirst(m), sortedFirst(m)))
	}
	if insp.rootFileSet["biome.json"] {
		add("biome", confHigh, fileSig("biome.json", "biome.json"))
	}
	if insp.rootFileSet["go.mod"] {
		add("go vet", confMedium, fileSig("go.mod", "go.mod"))
	}
	if insp.rootFileSet["Cargo.toml"] {
		add("clippy", confMedium, fileSig("Cargo.toml", "Cargo.toml"))
	}
	if py, err := readSmallFile(filepath.Join(root, "pyproject.toml")); err == nil {
		if strings.Contains(py, "ruff") {
			add("ruff", confHigh, contentSig("ruff em pyproject.toml"))
		}
		if strings.Contains(py, "mypy") {
			add("mypy", confHigh, contentSig("mypy em pyproject.toml"))
		}
	}
	if pkg := readPackage(root); pkg != nil {
		if d := depsContain(pkg, "eslint", "@biomejs/biome"); d != "" {
			name := "eslint"
			if d == "@biomejs/biome" {
				name = "biome"
			}
			if !hasTech(out, name) {
				add(name, confMedium, depSig(d))
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Type checkers
// ---------------------------------------------------------------------------

// DetectTypeCheckers returns type checkers evidenced by config/deps.
func DetectTypeCheckers(root string) []Tech { return detectTypeCheckers(root, inspect(root)) }

func detectTypeCheckers(root string, insp *inspection) []Tech {
	out := []Tech{}
	if m := globAt(root, "tsconfig.json", "tsconfig.*.json"); len(m) > 0 {
		out = append(out, tech("tsc", "typechecker", confHigh, []Signal{fileSig(sortedFirst(m), sortedFirst(m))}, "tsc"))
	}
	if insp.rootFileSet["mypy.ini"] || contentContains(filepath.Join(root, "pyproject.toml"), "mypy") {
		ev := []Signal{fileSig("mypy.ini", "mypy.ini")}
		if !insp.rootFileSet["mypy.ini"] {
			ev = []Signal{contentSig("mypy em pyproject.toml")}
		}
		out = append(out, tech("mypy", "typechecker", confHigh, ev, "mypy"))
	}
	if pkg := readPackage(root); pkg != nil {
		if d := depsContain(pkg, "typescript"); d != "" {
			if !hasTech(out, "tsc") {
				out = append(out, tech("tsc", "typechecker", confMedium, []Signal{depSig(d)}, "typescript"))
			}
		}
	}
	return out
}
