package projectintel

// Testes de fixtures do Project Intelligence (detecção por evidência).
// MISSÃO: provar que o engine detecta por MÚLTIPLOS sinais, nunca por nome de
// pasta, ignora node_modules/vendor, é read-only e extensível.
//
// Detecção é por ARQUIVO estático → os fixtures não dependem de toolchain
// instalada (node/go/python) — apenas criam arquivos em t.TempDir() e verificam
// o ProjectProfile. Os testes são determinísticos e não executam comandos.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// helpers: cria estrutura de arquivos a partir de um map[path]conteúdo.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// mkdir cria um diretório vazio (apenas um placeholder, sem arquivo).
func mkdirFile(list map[string]string, path string) map[string]string {
	list[path+"/.keep"] = ""
	return list
}

func hasTechName(list []Tech, name string) bool {
	for _, t := range list {
		if t.Name == name {
			return true
		}
	}
	return false
}

func confOf(list []Tech, name string) string {
	for _, t := range list {
		if t.Name == name {
			return t.Confidence
		}
	}
	return ""
}

func TestDetect_ReactViteTS(t *testing.T) {
	files := map[string]string{
		"package.json": `{
			"name":"react-vite-app","scripts":{"dev":"vite","build":"vite build","test":"vitest"},
			"dependencies":{"react":"^18","react-dom":"^18"},"devDependencies":{"vite":"^5","vitest":"^1","typescript":"^5"}
		}`,
		"tsconfig.json":    `{"compilerOptions":{"jsx":"react-jsx"}}`,
		"vite.config.ts":   `export default {}`,
		"package-lock.json": `{}`,
		"src/App.tsx":      `export default function App(){return <div/>}`,
		"src/main.tsx":     `import App from './App'`,
		"src/Index.tsx":    `import App from './App'`,
		"src/Other.tsx":    `import App from './App'`,
		"src/Util.tsx":     `export const a=1`,
		"src/Helper.tsx":   `export const b=2`,
		"src/Card.tsx":     `export const c=3`,
		"src/Logo.tsx":     `export const d=4`,
		"src/Header.tsx":   `export const e=5`,
		"src/Footer.tsx":   `export const f=6`,
	}
	root := fixture(t, files)
	p, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if !hasTechName(p.Languages, "TypeScript") {
		t.Errorf("esperava TypeScript, got %v", names(p.Languages))
	}
	if confOf(p.Languages, "TypeScript") != "high" {
		t.Errorf("TypeScript deveria ser high (tsconfig+n .tsx), got %s", confOf(p.Languages, "TypeScript"))
	}
	if !hasTechName(p.Frameworks, "React") {
		t.Errorf("esperava React, got %v", names(p.Frameworks))
	}
	if !hasTechName(p.Frameworks, "Vite") {
		t.Errorf("esperava Vite (vite.config), got %v", names(p.Frameworks))
	}
	if !hasTechName(p.PackageManagers, "npm") {
		t.Errorf("esperava npm (package-lock), got %v", names(p.PackageManagers))
	}
	if !hasTechName(p.TypeCheckers, "tsc") {
		t.Errorf("esperava tsc (tsconfig), got %v", names(p.TypeCheckers))
	}
	if !hasTechName(p.TestTools, "vitest") {
		t.Errorf("esperava vitest, got %v", names(p.TestTools))
	}
	if c := p.Capabilities["build"]; c != "vite" {
		t.Errorf("capability build deveria ser vite, got %q", c)
	}
	if !contains(commandsNames(p.Commands), "dev") {
		t.Errorf("esperava command dev, got %v", commandsNames(p.Commands))
	}
	if !contains(p.ProjectTypes, "frontend") {
		t.Errorf("esperava project type frontend, got %v", p.ProjectTypes)
	}
}

func TestDetect_NextTS_Pnpm(t *testing.T) {
	files := map[string]string{
		"package.json": `{"name":"next-app","scripts":{"dev":"next dev","build":"next build"},
			"dependencies":{"next":"^14","react":"^18"},"devDependencies":{"eslint":"^8","prettier":"^3","typescript":"^5"}}`,
		"tsconfig.json":    `{}`,
		"next.config.mjs":  `export default {}`,
		"pnpm-lock.yaml":   `lockfileVersion: '6.0'`,
		"eslint.config.mjs": `export default []`,
		".prettierrc":      `{"semi":false}`,
		"app/page.tsx":     `export default function Home(){return <h1>hi</h1>}`,
		"app/layout.tsx":   `export default function Layout(){return <html/>}`,
		"app/Index.tsx":    `export default function X(){return null}`,
		"app/About.tsx":    `export default function X(){return null}`,
		"app/Header.tsx":   `export default function X(){return null}`,
		"app/Footer.tsx":   `export default function X(){return null}`,
		"app/Main.tsx":     `export default function X(){return null}`,
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if !hasTechName(p.Frameworks, "Next.js") {
		t.Errorf("esperava Next.js, got %v", names(p.Frameworks))
	}
	if !hasTechName(p.PackageManagers, "pnpm") {
		t.Errorf("esperava pnpm (pnpm-lock), got %v", names(p.PackageManagers))
	}
	if !hasTechName(p.Linters, "eslint") {
		t.Errorf("esperava eslint, got %v", names(p.Linters))
	}
	if !hasTechName(p.Formatters, "prettier") {
		t.Errorf("esperava prettier, got %v", names(p.Formatters))
	}
	if !hasTechName(p.TypeCheckers, "tsc") {
		t.Errorf("esperava tsc, got %v", names(p.TypeCheckers))
	}
	if c := p.Capabilities["build"]; c != "next" {
		t.Errorf("capability build deveria ser next, got %q", c)
	}
}

func TestDetect_Go(t *testing.T) {
	files := map[string]string{
		"go.mod":     "module example.com/foo\n\ngo 1.25\n",
		"cmd/foo/main.go": `package main\nfunc main(){}\n`,
		"pkg/bar/bar.go":  `package bar\nfunc Bar() string { return "x" }`,
		"pkg/baz/baz.go":  `package baz`,
		"internal/one.go": `package internal`,
		"internal/two.go": `package internal`,
		"internal/three.go": `package internal`,
		"internal/four.go": `package internal`,
		"internal/bar_test.go": `package internal\nfunc TestX(t *testing.T){}\n`,
		"Makefile":        "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...",
		"README.md":       "# foo\n",
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if !hasTechName(p.Languages, "Go") {
		t.Errorf("esperava Go, got %v", names(p.Languages))
	}
	if confOf(p.Languages, "Go") != "high" {
		t.Errorf("Go deveria ser high (go.mod + .go), got %s", confOf(p.Languages, "Go"))
	}
	if !hasTechName(p.Formatters, "gofmt") {
		t.Errorf("esperava gofmt, got %v", names(p.Formatters))
	}
	if !hasTechName(p.Linters, "go vet") {
		t.Errorf("esperava go vet, got %v", names(p.Linters))
	}
	if !hasTechName(p.TestTools, "go") {
		t.Errorf("esperava go test (com *_test.go), got %v", names(p.TestTools))
	}
	if !hasTechName(p.BuildTools, "make") {
		t.Errorf("esperava make (Makefile), got %v", names(p.BuildTools))
	}
	if c := p.Capabilities["build"]; c != "go" {
		t.Errorf("capability build deveria ser go, got %q", c)
	}
}

func TestDetect_Python(t *testing.T) {
	files := map[string]string{
		"pyproject.toml": "[project]\nname='foo'\ndependencies=['fastapi']\n\n[tool.ruff]\nline-length=88\n[tool.black]\n",
		"app/main.py":    `from fastapi import FastAPI\napp=FastAPI()`,
		"app/models.py":  `class M: pass`,
		"app/routes.py":  `def r(): pass`,
		"tests/test_x.py": `def test_x(): pass`,
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if !hasTechName(p.Languages, "Python") {
		t.Errorf("esperava Python, got %v", names(p.Languages))
	}
	if !hasTechName(p.Frameworks, "FastAPI") {
		t.Errorf("esperava FastAPI, got %v", names(p.Frameworks))
	}
	if !hasTechName(p.Formatters, "ruff") {
		t.Errorf("esperava ruff, got %v", names(p.Formatters))
	}
	if !hasTechName(p.Linters, "ruff") {
		t.Errorf("esperava ruff linter, got %v", names(p.Linters))
	}
	if !hasTechName(p.TestTools, "pytest") {
		t.Errorf("esperava pytest (test_*.py), got %v", names(p.TestTools))
	}
}

func TestDetect_RustMock(t *testing.T) {
	files := map[string]string{
		"Cargo.toml": "[package]\nname='foo'\n\n[dependencies]\nactix-web='4'\n\n[workspace]\n",
		"src/main.rs": `fn main(){}`,
		"src/lib.rs":  `pub fn x(){}`,
		"src/other.rs": `pub fn y(){}`,
		"src/one.rs":   `pub fn z(){}`,
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if !hasTechName(p.Languages, "Rust") {
		t.Errorf("esperava Rust, got %v", names(p.Languages))
	}
	if !hasTechName(p.BuildTools, "cargo") {
		t.Errorf("esperava cargo, got %v", names(p.BuildTools))
	}
	if !hasTechName(p.Formatters, "rustfmt") {
		t.Errorf("esperava rustfmt, got %v", names(p.Formatters))
	}
	if !hasTechName(p.Linters, "clippy") {
		t.Errorf("esperava clippy, got %v", names(p.Linters))
	}
	if !hasTechName(p.Monorepo, "Cargo workspace") {
		t.Errorf("esperava Cargo workspace, got %v", names(p.Monorepo))
	}
	if !hasTechName(p.Frameworks, "Actix") {
		t.Errorf("esperava Actix (actix-web dep), got %v", names(p.Frameworks))
	}
}

func TestDetect_DockerMonorepo(t *testing.T) {
	files := map[string]string{
		"Dockerfile":           "FROM node:20\n",
		"docker-compose.yml":   "services:\n  db:\n    image: postgres:16\n",
		"pnpm-workspace.yaml":  "packages:\n  - 'apps/*'\n  - 'packages/*'\n",
		"apps/web/package.json": `{"name":"web"}`,
		"packages/ui/package.json": `{"name":"ui"}`,
		"README.md": "# mono\n",
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if !hasTechName(p.Container, "Docker") {
		t.Errorf("esperava Docker, got %v", names(p.Container))
	}
	if !hasTechName(p.Container, "Compose") {
		t.Errorf("esperava Compose, got %v", names(p.Container))
	}
	if !hasTechName(p.Monorepo, "pnpm workspace") {
		t.Errorf("esperava pnpm workspace, got %v", names(p.Monorepo))
	}
	if !hasTechName(p.Database, "PostgreSQL") {
		t.Errorf("esperava PostgreSQL (compose image), got %v", names(p.Database))
	}
	if !contains(p.ProjectTypes, "monorepo") {
		t.Errorf("esperava project type monorepo, got %v", p.ProjectTypes)
	}
}

func TestDetect_Empty(t *testing.T) {
	root := t.TempDir()
	p, _ := Analyze(root)

	if len(p.Languages) != 0 {
		t.Errorf("empty não deveria ter languages, got %v", names(p.Languages))
	}
	if len(p.Capabilities) != 0 {
		t.Errorf("empty não deveria ter capabilities, got %v", p.Capabilities)
	}
	if !contains(p.ProjectTypes, "empty") {
		t.Errorf("esperava project type empty, got %v", p.ProjectTypes)
	}
}

func TestDetect_NeverByFolderName(t *testing.T) {
	// Pasta chamada "react" mas que NÃO tem evidência de React → NÃO declara React.
	files := map[string]string{
		"react/foo.txt": "not react at all",
		"react/bar.txt": "still not react",
		"src/main.js":   "console.log('x')",
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	if hasTechName(p.Frameworks, "React") {
		t.Errorf("NÃO deveria detectar React por pasta 'react/' sem evidência, got %v", names(p.Frameworks))
	}
	if len(p.Frameworks) != 0 {
		t.Errorf("sem evidência, frameworks deveria ser vazio, got %v", names(p.Frameworks))
	}
}

func TestDetect_IgnoresNodeModules(t *testing.T) {
	// node_modules é um diretório gigante que NUNCA deve inflar a contagem.
	files := map[string]string{
		"package.json": `{"name":"x","dependencies":{"react":"^18"}}`,
		"tsconfig.json": `{}`,
		"src/App.tsx":   `export default function A(){return null}`,
		"src/Main.tsx":  `export default function M(){return null}`,
		"src/One.tsx":   `export default function O(){return null}`,
		"src/Two.tsx":   `export default function T(){return null}`,
		"src/Three.tsx": `export default function T(){return null}`,
		"src/Four.tsx":  `export default function F(){return null}`,
		"src/Five.tsx":  `export default function F(){return null}`,
		"src/Six.tsx":   `export default function S(){return null}`,
	}
	// injeta muitos .tsx falsos em node_modules — não devem contar como TS do projeto.
	for i := 0; i < 50; i++ {
		files["node_modules/pkg"+string(rune('a'+i%26))+string(rune('0'+i/26))+"/index.tsx"] = `export const x=1`
	}
	root := fixture(t, files)
	p, _ := Analyze(root)

	for _, lang := range p.Languages {
		if lang.Name == "TypeScript" {
			// confirma que node_modules NÃO inflou para "many files"
			for _, s := range lang.Evidence {
				if s.Kind == "count" && strings.Contains(s.Value, "100") {
					t.Errorf("node_modules inflou a contagem (.tsx de dentro de node_modules não deve contar): %s", s.Value)
				}
			}
		}
	}
}

func TestAnalyze_DSL(t *testing.T) {
	// Analyze deve retornar perfil coerente: DetectedAt set, Name set, Evidence consolidada.
	files := map[string]string{
		"go.mod":   "module x\n\ngo 1.25\n",
		"main.go":  "package main\nfunc main(){}\n",
		"README.md": "# x\n",
	}
	root := fixture(t, files)
	p, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if p.Name != filepath.Base(root) {
		t.Errorf("Name deveria ser base do root, got %q", p.Name)
	}
	if p.DetectedAt == "" {
		t.Errorf("DetectedAt deveria ser setado")
	}
	if len(p.Evidence) == 0 {
		t.Errorf("Evidence consolidada não deveria ser vazia (tem go.mod + README)")
	}
	if p.Root != root {
		t.Errorf("Root deveria ser %q", root)
	}
}

// helpers de assert/coleta
func names(list []Tech) []string {
	var out []string
	for _, t := range list {
		out = append(out, t.Name)
	}
	sort.Strings(out)
	return out
}

func commandsNames(c []DetectedCommand) []string {
	var out []string
	for _, x := range c {
		out = append(out, x.Name)
	}
	sort.Strings(out)
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
