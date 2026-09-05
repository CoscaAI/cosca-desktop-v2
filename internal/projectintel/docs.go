package projectintel

// CATEGORY: docs.go — detecção de evidência documental.
// README/CONTRIBUTING/ARCHITECTURE/CHANGELOG/SECURITY, diretório docs/ e
// contagem de arquivos .md.

import (
	"fmt"
	"strings"
)

// DetectDocumentation returns documentation evidence (READMEs, docs/ dir, .md count).
func DetectDocumentation(root string) []Tech { return detectDocumentation(root, inspect(root)) }

func detectDocumentation(root string, insp *inspection) []Tech {
	out := []Tech{}
	if insp.rootFileSet["README.md"] || insp.rootFileSet["readme.md"] {
		name := "README.md"
		if insp.rootFileSet["readme.md"] && !insp.rootFileSet["README.md"] {
			name = "readme.md"
		}
		out = append(out, tech("README", "documentation", confHigh, []Signal{fileSig(name, name)}, ""))
	}
	for _, f := range []string{"CONTRIBUTING.md", "ARCHITECTURE.md", "CHANGELOG.md", "SECURITY.md"} {
		if insp.rootFileSet[f] {
			out = append(out, tech(strings.TrimSuffix(f, ".md"), "documentation", confMedium, []Signal{fileSig(f, f)}, ""))
		}
	}
	if insp.rootDirs["docs"] {
		out = append(out, tech("docs", "documentation", confMedium, []Signal{fileSig("docs", "docs")}, ""))
	}
	if insp.mdCount > 0 {
		out = append(out, tech("markdown", "documentation", confLow, []Signal{countSig(fmt.Sprintf("%d .md file(s)", insp.mdCount))}, ""))
	}
	return out
}
