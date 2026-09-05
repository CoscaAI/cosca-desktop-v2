package projectintel

// CATEGORY: commands.go — descoberta de comandos reais.
// Lê scripts de package.json, targets de Makefile/Justfile e tasks de
// Taskfile.yml e os reporta como DetectedCommand (nunca inventa comandos).

import "path/filepath"

// DetectCommands returns real commands from package.json scripts, Makefile,
// Justfile and Taskfile.yml (never invents commands).
func DetectCommands(root string) []DetectedCommand { return detectCommands(root, inspect(root)) }

func detectCommands(root string, insp *inspection) []DetectedCommand {
	out := []DetectedCommand{}

	if pkg := readPackage(root); pkg != nil {
		names := sortedKeys(pkg.Scripts)
		for _, n := range names {
			if n == "" {
				continue
			}
			out = append(out, DetectedCommand{Name: n, Cmd: pkg.Scripts[n], Source: "package.json scripts"})
		}
	}
	if content, err := readSmallFile(filepath.Join(root, "Makefile")); err == nil {
		for _, t := range readMakeTargets(content) {
			out = append(out, DetectedCommand{Name: t, Cmd: "make " + t, Source: "Makefile"})
		}
	}
	if content, err := readSmallFile(filepath.Join(root, "Justfile")); err == nil {
		for _, t := range readMakeTargets(content) {
			out = append(out, DetectedCommand{Name: t, Cmd: "just " + t, Source: "Justfile"})
		}
	}
	if content, err := readSmallFile(filepath.Join(root, "Taskfile.yml")); err == nil {
		for _, t := range readTaskNames(content) {
			out = append(out, DetectedCommand{Name: t, Cmd: "task " + t, Source: "Taskfile"})
		}
	}
	return out
}
