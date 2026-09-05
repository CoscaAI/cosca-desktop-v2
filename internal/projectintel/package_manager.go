package projectintel

// CATEGORY: package_manager.go — detecção de gerenciadores de pacote.
// Evidência por lockfile (package-lock.json, pnpm-lock.yaml, yarn.lock,
// bun.lock, Cargo.lock, poetry.lock, uv.lock, Go.sum, requirements.txt, ...)
// e por package.json + node_modules instalado (fallback npm médio).

// DetectPackageManagers returns package managers evidenced by lockfiles.
func DetectPackageManagers(root string) []Tech { return detectPackageManagers(root, inspect(root)) }

func detectPackageManagers(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) {
		out = appendTech(out, tech(name, "packagemanager", conf, ev, ""))
	}
	if insp.rootFileSet["package-lock.json"] {
		add("npm", confHigh, fileSig("package-lock.json", "package-lock.json"))
	}
	if insp.rootFileSet["npm-shrinkwrap.json"] {
		add("npm", confHigh, fileSig("npm-shrinkwrap.json", "npm-shrinkwrap.json"))
	}
	if insp.rootFileSet["pnpm-lock.yaml"] {
		add("pnpm", confHigh, fileSig("pnpm-lock.yaml", "pnpm-lock.yaml"))
	}
	if insp.rootFileSet["yarn.lock"] {
		add("yarn", confHigh, fileSig("yarn.lock", "yarn.lock"))
	}
	for _, b := range []string{"bun.lock", "bun.lockb", "bun.lock.json"} {
		if insp.rootFileSet[b] {
			add("bun", confHigh, fileSig(b, b))
		}
	}
	if insp.rootFileSet["Cargo.lock"] {
		add("cargo", confHigh, fileSig("Cargo.lock", "Cargo.lock"))
	}
	if insp.rootFileSet["poetry.lock"] {
		add("poetry", confHigh, fileSig("poetry.lock", "poetry.lock"))
	}
	if insp.rootFileSet["uv.lock"] {
		add("uv", confHigh, fileSig("uv.lock", "uv.lock"))
	}
	if insp.rootFileSet["Go.sum"] || insp.rootFileSet["go.sum"] {
		add("go modules", confMedium, fileSig("go.sum", "go.sum"))
	}
	if insp.rootFileSet["requirements.txt"] {
		add("pip", confMedium, fileSig("requirements.txt", "requirements.txt"))
	}
	if insp.rootFileSet["Pipfile"] {
		add("pipenv", confMedium, fileSig("Pipfile", "Pipfile"))
	}
	if insp.rootFileSet["composer.lock"] {
		add("composer", confHigh, fileSig("composer.lock", "composer.lock"))
	}
	if insp.rootFileSet["Gemfile.lock"] {
		add("bundler", confMedium, fileSig("Gemfile.lock", "Gemfile.lock"))
	}
	// package.json + installed node_modules (no other lockfile) => npm (medium).
	pkg := readPackage(root)
	if pkg != nil && insp.rootDirs["node_modules"] && len(out) == 0 {
		add("npm", confMedium, fileSig("package.json", "package.json"))
	}
	return out
}
