package projectintel

// CATEGORY: monorepo.go — detecção de monorepo tooling.
// Evidência por manifests de workspace (pnpm-workspace.yaml, package.json
// workspaces, turbo.json, nx.json, lerna.json, go.work, [workspace] em
// Cargo.toml).

import (
	"path/filepath"
	"strings"
)

// DetectMonorepo returns monorepo tooling evidenced by workspace manifests.
func DetectMonorepo(root string) []Tech { return detectMonorepo(root, inspect(root)) }

func detectMonorepo(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "monorepo", conf, ev, "")) }

	if insp.rootFileSet["pnpm-workspace.yaml"] {
		add("pnpm workspace", confHigh, fileSig("pnpm-workspace.yaml", "pnpm-workspace.yaml"))
	}
	if pkg := readPackage(root); pkg != nil && hasWorkspaces(pkg) {
		add("npm/yarn workspace", confMedium, contentSig("workspaces em package.json"))
	}
	if insp.rootFileSet["turbo.json"] {
		add("Turborepo", confHigh, fileSig("turbo.json", "turbo.json"))
	}
	if insp.rootFileSet["nx.json"] {
		add("Nx", confHigh, fileSig("nx.json", "nx.json"))
	}
	if insp.rootFileSet["lerna.json"] {
		add("Lerna", confHigh, fileSig("lerna.json", "lerna.json"))
	}
	if insp.rootFileSet["go.work"] {
		add("Go workspace", confHigh, fileSig("go.work", "go.work"))
	}
	if content, err := readSmallFile(filepath.Join(root, "Cargo.toml")); err == nil && strings.Contains(content, "[workspace]") {
		add("Cargo workspace", confHigh, contentSig("[workspace] em Cargo.toml"))
	}
	return out
}
