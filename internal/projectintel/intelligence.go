package projectintel

// CATEGORY: intelligence.go — camada de SURFÍCIE sobre o ProjectProfile que
// alimenta o Agente/UI sem que ele pergunte nada. Três helpers read-only que
// derivam TUDO do perfil já detectado (nunca re-scanneiam, nunca executam):
//
//   - AgentStack(profile)               → string "Go" / "TypeScript · React · Vite"
//   - AgentSkillMatch(profile)          → []string de skills que CASAM com o perfil
//   - AgentPipeline(profile)            → []{step,tool,status} do pipeline de QA/build
//
// REGRAS (ADR-0007):
//   - NÃO duplica kernel/memória do Cosca: apenas NOMEIA as skills que casam
//     (strings), nunca carrega conteúdo.
//   - NÃO inventa tecnologia: tudo deriva das categorias que o PI de fato
//     detectou. Ferramenta ausente => status "not_applicable" (não erro).
//   - Determinístico: a ordem de skills é ordenada; o pipeline segue a base
//     fixa discover→…→review.
//
// É a camada de SURFÍCIE do PI — NÃO todas as skills/capacidades do sistema,
// apenas as que correspondem ao perfil real do projeto.

import (
	"sort"
	"strings"
)

// techSkillMap mapeia o NOME de uma tecnologia detectada pelo PI para o slug de
// skill correspondente. Vários nomes podem casar no MESMO slug (ex.: build tool
// "go" e runtime "Go" → "go"); o dedupe garante unicidade. Só entram aqui
// tecnologias reais que o detector produz — é a superfície do que foi detectado.
var techSkillMap = map[string]string{
	// Linguagens
	"TypeScript": "typescript",
	"JavaScript": "javascript",
	"Go":         "go",
	"Rust":       "rust",
	"Python":     "python",
	"Java":       "java",
	"Kotlin":     "kotlin",
	"C#":         "csharp",
	"C/C++":      "cpp",
	"PHP":        "php",
	"Ruby":       "ruby",
	"Swift":      "swift",

	// Frameworks (frontend)
	"React":     "react",
	"Next.js":   "nextjs",
	"Vue":       "vue",
	"Nuxt":      "nuxt",
	"Angular":   "angular",
	"Svelte":    "svelte",
	"SvelteKit": "sveltekit",
	"Remix":     "remix",
	"Vite":      "vite",
	"Astro":     "astro",
	"Tauri":     "tauri",

	// Frameworks (backend)
	"Express":     "express",
	"Fastify":     "fastify",
	"Koa":         "koa",
	"NestJS":      "nestjs",
	"Hapi":        "hapi",
	"Restify":     "restify",
	"LoopBack":    "loopback",
	"FastAPI":     "fastapi",
	"Django":      "django",
	"Flask":       "flask",
	"Actix":       "actix",
	"Axum":        "axum",
	"Rocket":      "rocket",
	"Gin":         "gin",
	"Echo":        "echo",
	"Fiber":       "fiber",
	"Spring Boot": "spring",

	// Build tools
	"go":      "go",
	"cargo":   "rust",
	"make":    "make",
	"cmake":   "cmake",
	"gradle":  "gradle",
	"maven":   "maven",
	"task":    "task",
	"next":    "nextjs",
	"vite":    "vite",
	"webpack": "webpack",
	"nuxt":    "nuxt",
	"tsc":     "typescript",
	"esbuild": "esbuild",
	"rollup":  "rollup",

	// Test tools
	"vitest":    "vitest",
	"jest":      "jest",
	"playwright": "playwright",
	"cypress":   "cypress",
	"mocha":     "mocha",
	"jasmine":   "jasmine",
	"ava":       "ava",
	"karma":     "karma",
	"pytest":    "pytest",

	// Formatters
	"prettier":    "prettier",
	"biome":       "biome",
	"gofmt":       "go",
	"rustfmt":     "rust",
	"black":       "black",
	"ruff":        "ruff",
	"editorconfig": "editorconfig",

	// Linters
	"eslint": "eslint",
	"go vet": "go",
	"clippy": "rust",
	"mypy":   "python",

	// Container / infra
	"Docker":       "docker",
	"Compose":      "docker",
	"devcontainer": "devcontainer",
	"Terraform":    "terraform",
	"Helm":         "helm",
	"Kustomize":    "kustomize",

	// Database
	"Prisma":     "prisma",
	"Drizzle":    "drizzle",
	"PostgreSQL": "postgres",
	"MySQL":      "mysql",
	"SQLite":     "sqlite",
	"MongoDB":    "mongodb",
	"Redis":      "redis",

	// Monorepo (todos -> monorepo)
	"pnpm workspace":     "monorepo",
	"npm/yarn workspace": "monorepo",
	"Turborepo":          "monorepo",
	"Nx":                 "monorepo",
	"Lerna":              "monorepo",
	"Go workspace":       "monorepo",
	"Cargo workspace":    "monorepo",

	// CI
	"GitHub Actions": "ci",
	"GitLab CI":      "ci",
	"Jenkins":        "ci",
	"Azure Pipelines": "ci",
	"CircleCI":       "ci",
	"Travis CI":      "ci",

	// VCS
	"Git": "git",
}

// AgentSkillMatch devolve as skills que CASAM com o perfil detectado (ordenadas,
// sem duplicados). Carrega SOMENTE as skills aplicáveis ao perfil real — nunca
// todas as skills do sistema. Se nada casar, retorna [] (vazio).
func AgentSkillMatch(p *ProjectProfile) []string {
	if p == nil {
		return []string{}
	}
	seen := map[string]bool{}
	out := []string{}
	collect := func(list []Tech) {
		for _, t := range list {
			s, ok := techSkillMap[t.Name]
			if ok && s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	for _, l := range allTechLists(p) {
		collect(l)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// pipelineSteps é a sequência BASE do pipeline auto-descoberto. As etapas que o
// projeto NÃO tem viram status "not_applicable" (nunca erro); as que têm viram
// "available" com a ferramenta real.
var pipelineSteps = []string{"discover", "understand", "plan", "implement", "format", "lint", "typecheck", "test", "build", "diff", "review"}

// AgentPipeline devolve o pipeline auto-descoberto e ADAPTADO ao projeto: cada
// etapa traz {step, tool, status}. status ∈ available | not_applicable | unknown.
//
// A sequência é fixa (discover→…→review). As etapas de ferramenta (format/lint/
// typecheck/test/build) usam a categoria detectada; diff usa o VCS; as etapas de
// agentes (discover/understand/plan/implement/review) estão sempre disponíveis
// porque são do fluxo do agente. Ferramenta ausente => "NOT_APPLICABLE".
func AgentPipeline(p *ProjectProfile) []map[string]string {
	if p == nil {
		return []map[string]string{}
	}
	steps := make([]map[string]string, 0, len(pipelineSteps))
	add := func(step, tool, status string) {
		steps = append(steps, map[string]string{"step": step, "tool": tool, "status": status})
	}

	add("discover", "projectintel", "available")
	add("understand", "context", "available")
	add("plan", "cosca plan", "available")
	add("implement", "cosca exec", "available")

	addToolStep := func(step string, cat []Tech, cmdFn func(string) string) {
		if t := firstTech(cat); t != "" {
			add(step, cmdFn(t), "available")
		} else {
			add(step, "NOT_APPLICABLE", "not_applicable")
		}
	}
	addToolStep("format", p.Formatters, formatCmd)
	addToolStep("lint", p.Linters, lintCmd)
	addToolStep("typecheck", p.TypeCheckers, typecheckCmd)
	addToolStep("test", p.TestTools, testCmd)
	addToolStep("build", p.BuildTools, buildCmd)

	if hasTech(p.Repository, "Git") {
		add("diff", "git diff", "available")
	} else {
		add("diff", "NOT_APPLICABLE", "not_applicable")
	}

	add("review", "review", "available")
	return steps
}

// AgentStack devolve o "stack" textual do projeto: linguagem + frameworks top
// (até 3), com dedupe case-insensitive. Compacto e determinístico para a UI/
// Agente exibirem num único lugar (ex.: "TypeScript · React · Vite").
func AgentStack(p *ProjectProfile) string {
	if p == nil {
		return ""
	}
	seen := map[string]bool{}
	items := []string{}
	add := func(name string) {
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, name)
	}
	add(firstTech(p.Languages))
	for i, fw := range p.Frameworks {
		if i >= 3 {
			break
		}
		add(fw.Name)
	}
	if len(items) <= 1 {
		// sem frameworks: completa o stack com runtime/build/container reais
		add(firstTech(p.Runtime))
		add(firstTech(p.BuildTools))
		add(firstTech(p.Container))
	}
	return strings.Join(items, " · ")
}

// buildCmd normaliza a ferramenta de build para um comando descritivo (ex.: o
// detector reporta "go" e aqui vira "go build"; "next" permanece "next").
func buildCmd(tool string) string {
	switch tool {
	case "go":
		return "go build"
	case "cargo":
		return "cargo build"
	case "cmake":
		return "cmake"
	case "gradle":
		return "gradle"
	case "maven":
		return "maven"
	case "task":
		return "task"
	case "make":
		return "make"
	case "next":
		return "next"
	case "vite":
		return "vite build"
	case "webpack":
		return "webpack"
	case "nuxt":
		return "nuxt"
	case "tsc":
		return "tsc"
	case "esbuild":
		return "esbuild"
	case "rollup":
		return "rollup"
	default:
		return tool
	}
}

// testCmd normaliza a ferramenta de teste para um comando descritivo.
func testCmd(tool string) string {
	switch tool {
	case "go":
		return "go test"
	case "cargo":
		return "cargo test"
	default:
		return tool
	}
}

func formatCmd(tool string) string    { return tool }
func lintCmd(tool string) string      { return tool }
func typecheckCmd(tool string) string { return tool }
