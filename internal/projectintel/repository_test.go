package projectintel

// Testes da detecção de Git (repository.go). Ainda NÃO acoplada no Analyze —
// a função DetectRepository é testada de forma isolada, garantindo que a
// adição de version-control não quebrou o contrato público nem os fixtures.

import "testing"

func TestDetectRepository_GitDir(t *testing.T) {
	files := map[string]string{
		"go.mod":         "module x\n\ngo 1.25\n",
		".git/.keep":     "",
		".gitignore":     "node_modules\n",
		".gitattributes": "* text=auto\n",
	}
	root := fixture(t, files)

	got := DetectRepository(root)
	if !hasTechName(got, "Git") {
		t.Fatalf("esperava Git, got %v", names(got))
	}
	if confOf(got, "Git") != "high" {
		t.Errorf("Git deveria ser high (.git presente), got %s", confOf(got, "Git"))
	}
}

func TestDetectRepository_NoGit(t *testing.T) {
	files := map[string]string{
		"go.mod":  "module x\n\ngo 1.25\n",
		"main.go": "package main\nfunc main(){}\n",
	}
	root := fixture(t, files)

	got := DetectRepository(root)
	if hasTechName(got, "Git") {
		t.Fatalf("sem evidência de Git não deveria declarar Git, got %v", names(got))
	}
}

func TestDetectRepository_GitignoreOnly(t *testing.T) {
	files := map[string]string{
		"package.json": `{"name":"x","scripts":{"build":"vite build"}}`,
		".gitignore":   "node_modules\n",
	}
	root := fixture(t, files)

	got := DetectRepository(root)
	if !hasTechName(got, "Git") {
		t.Fatalf("esperava Git (via .gitignore), got %v", names(got))
	}
	if confOf(got, "Git") != "medium" {
		t.Errorf("Git via .gitignore apenas deveria ser medium, got %s", confOf(got, "Git"))
	}
}
