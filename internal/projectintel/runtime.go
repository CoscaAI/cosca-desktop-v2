package projectintel

// CATEGORY: runtime.go — detecção de runtimes de linguagem.
// Evidência por manifests (package.json/.nvmrc, deno.json, bun.lock, go.mod,
// pyproject.toml/requirements.txt, Cargo.toml, pom.xml).

// DetectRuntime returns language runtimes evidenced by manifests.
func DetectRuntime(root string) []Tech { return detectRuntime(root, inspect(root)) }

func detectRuntime(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "runtime", conf, ev, "")) }
	if insp.rootFileSet["package.json"] || insp.rootFileSet[".nvmrc"] || insp.rootFileSet[".node-version"] {
		add("Node.js", confMedium, fileSig("package.json", "package.json"))
	}
	if insp.rootFileSet["deno.json"] {
		add("Deno", confMedium, fileSig("deno.json", "deno.json"))
	}
	if insp.rootFileSet["bun.lockb"] || insp.rootFileSet["bun.lock"] {
		add("Bun", confMedium, fileSig("bun.lock", "bun.lock"))
	}
	if insp.rootFileSet["go.mod"] {
		add("Go", confMedium, fileSig("go.mod", "go.mod"))
	}
	if insp.rootFileSet["pyproject.toml"] || insp.rootFileSet["requirements.txt"] {
		add("Python", confMedium, fileSig("pyproject.toml", "pyproject.toml"))
	}
	if insp.rootFileSet["Cargo.toml"] {
		add("Rust", confMedium, fileSig("Cargo.toml", "Cargo.toml"))
	}
	if insp.rootFileSet["pom.xml"] {
		add("JVM", confMedium, fileSig("pom.xml", "pom.xml"))
	}
	return out
}
