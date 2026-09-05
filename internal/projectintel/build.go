package projectintel

// CATEGORY: build.go — detecção de ferramentas de build.
// Evidência por manifests/arquivos de build (go.mod/go.work, Cargo.toml,
// Makefile, CMakeLists.txt, build.gradle*, pom.xml, Taskfile.yml) e, para JS,
// pelo conteúdo do script "build" em package.json.

// DetectBuildTools returns build systems evidenced by manifests/scripts.
func DetectBuildTools(root string) []Tech { return detectBuildTools(root, inspect(root)) }

func detectBuildTools(root string, insp *inspection) []Tech {
	out := []Tech{}
	if insp.rootFileSet["go.mod"] || insp.rootFileSet["go.work"] {
		f := "go.mod"
		if insp.rootFileSet["go.work"] {
			f = "go.work"
		}
		out = append(out, tech("go", "buildtool", confHigh, []Signal{fileSig(f, f)}, "go build"))
	}
	if insp.rootFileSet["Cargo.toml"] {
		out = append(out, tech("cargo", "buildtool", confHigh, []Signal{fileSig("Cargo.toml", "Cargo.toml")}, "cargo build"))
	}
	if insp.rootFileSet["Makefile"] {
		out = append(out, tech("make", "buildtool", confHigh, []Signal{fileSig("Makefile", "Makefile")}, "make"))
	}
	if insp.rootFileSet["CMakeLists.txt"] {
		out = append(out, tech("cmake", "buildtool", confHigh, []Signal{fileSig("CMakeLists.txt", "CMakeLists.txt")}, "cmake"))
	}
	if m := globAt(root, "build.gradle*"); len(m) > 0 {
		name := sortedFirst(m)
		out = append(out, tech("gradle", "buildtool", confHigh, []Signal{fileSig(name, name)}, "gradle"))
	}
	if insp.rootFileSet["pom.xml"] {
		out = append(out, tech("maven", "buildtool", confHigh, []Signal{fileSig("pom.xml", "pom.xml")}, "maven"))
	}
	if insp.rootFileSet["Taskfile.yml"] {
		out = append(out, tech("task", "buildtool", confHigh, []Signal{fileSig("Taskfile.yml", "Taskfile.yml")}, "task"))
	}
	// JS build tool from package.json "build" script (content evidence).
	if pkg := readPackage(root); pkg != nil {
		if s, ok := pkg.Scripts["build"]; ok {
			if tool := toolFromScript(s, []string{"next", "vite", "webpack", "nuxt", "tsc", "esbuild", "rollup"}); tool != "" {
				out = append(out, tech(tool, "buildtool", confMedium, []Signal{contentSig("build: " + s)}, "script build"))
			}
		}
	}
	return out
}
