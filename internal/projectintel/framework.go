package projectintel

// CATEGORY: framework.go — detecção de frameworks (frontend + backend).
// Evidência por config file (frameConfig = forte/alta), por dependência
// declarada (frameDep = média) e por conteúdo do manifest host
// (backendFrames, ex.: FastAPI/Django em pyproject.toml, Gin/Echo em go.mod).

import (
	"path/filepath"
	"sort"
	"strings"
)

// frameDep maps a dependency key to a framework + category.
var frameDep = map[string]struct{ name, cat string }{
	"react":           {"React", "frontend"},
	"react-dom":       {"React", "frontend"},
	"next":            {"Next.js", "frontend"},
	"vue":             {"Vue", "frontend"},
	"nuxt":            {"Nuxt", "frontend"},
	"@angular/core":   {"Angular", "frontend"},
	"svelte":          {"Svelte", "frontend"},
	"@sveltejs/kit":   {"SvelteKit", "frontend"},
	"remix":           {"Remix", "frontend"},
	"vite":            {"Vite", "frontend"},
	"astro":           {"Astro", "frontend"},
	"express":         {"Express", "backend"},
	"fastify":         {"Fastify", "backend"},
	"koa":             {"Koa", "backend"},
	"@nestjs/core":    {"NestJS", "backend"},
	"hapi":            {"Hapi", "backend"},
	"restify":         {"Restify", "backend"},
	"@loopback/core":  {"LoopBack", "backend"},
	"@tauri-apps/api": {"Tauri", "frontend"},
}

// frameConfig maps a glob pattern to a framework + category (config file = strong evidence).
var frameConfig = []struct {
	pattern    string
	name, category string
}{
	{"next.config.*", "Next.js", "frontend"},
	{"vite.config.*", "Vite", "frontend"},
	{"nuxt.config.*", "Nuxt", "frontend"},
	{"angular.json", "Angular", "frontend"},
	{"svelte.config.*", "Svelte", "frontend"},
	{"remix.config.*", "Remix", "frontend"},
	{"astro.config.*", "Astro", "frontend"},
	{"nest-cli.json", "NestJS", "backend"},
	{"tauri.conf.json", "Tauri", "frontend"},
}

// backendFrames maps known backend framework names to their host manifest file
// and the substring that marks them (evidence-based content scan).
var backendFrames = []struct {
	name, file, marker, category string
}{
	{"FastAPI", "pyproject.toml", "fastapi", "backend"},
	{"Django", "pyproject.toml", "django", "backend"},
	{"Flask", "pyproject.toml", "flask", "backend"},
	{"Actix", "Cargo.toml", "actix-web", "backend"},
	{"Axum", "Cargo.toml", "axum", "backend"},
	{"Rocket", "Cargo.toml", "rocket", "backend"},
	{"Gin", "go.mod", "github.com/gin-gonic/gin", "backend"},
	{"Echo", "go.mod", "github.com/labstack/echo", "backend"},
	{"Fiber", "go.mod", "github.com/gofiber/fiber", "backend"},
	{"Spring Boot", "pom.xml", "spring-boot", "backend"},
}

// DetectFrameworks returns frontend + backend frameworks detected by config
// files and declared dependencies. It NEVER infers a framework from a folder
// name (anti-false-positive rule).
func DetectFrameworks(root string) []Tech { return detectFrameworks(root, inspect(root)) }

func detectFrameworks(root string, insp *inspection) []Tech {
	agg := map[string]*Tech{}

	add := func(name, cat, conf string, ev ...Signal) {
		if t, ok := agg[name]; ok {
			t.Evidence = append(t.Evidence, ev...)
			if conf == confHigh {
				t.Confidence = confHigh
			}
			return
		}
		agg[name] = &Tech{Name: name, Category: cat, Confidence: conf, Evidence: ev}
	}

	// Config files = strong (high) evidence.
	for _, fc := range frameConfig {
		for name := range globAt(root, fc.pattern) {
			add(fc.name, fc.category, confHigh, fileSig(name, name))
		}
	}

	// Declared dependencies = medium evidence.
	if pkg := readPackage(root); pkg != nil {
		deps := packageDeps(pkg)
		// iterate deterministically
		keys := make([]string, 0, len(deps))
		for k := range deps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if fr, ok := frameDep[k]; ok {
				if _, exists := agg[fr.name]; !exists || agg[fr.name].Confidence != confHigh {
					add(fr.name, fr.cat, confMedium, depSig(k))
				}
			}
		}
	}

	// Backend frameworks from host manifest content (static, read-only).
	for _, bf := range backendFrames {
		content, err := readSmallFile(filepath.Join(root, bf.file))
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(content), strings.ToLower(bf.marker)) {
			add(bf.name, bf.category, confMedium, contentSig("dependência "+bf.marker))
		}
	}

	names := make([]string, 0, len(agg))
	for n := range agg {
		names = append(names, n)
	}
	sort.Strings(names)
	out := []Tech{}
	for _, n := range names {
		out = append(out, *agg[n])
	}
	// Auto-install "frontend"/"backend" categories are left as-is.
	return out
}
