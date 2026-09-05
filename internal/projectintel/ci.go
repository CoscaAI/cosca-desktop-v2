package projectintel

// CATEGORY: ci.go — detecção de sistemas de CI.
// Evidência por arquivos de workflow (`.github/workflows`, .gitlab-ci.yml,
// Jenkinsfile, azure-pipelines.yml, .circleci/config.yml, .travis.yml).

import (
	"os"
	"path/filepath"
)

// DetectCI returns CI systems evidenced by workflow config files.
func DetectCI(root string) []Tech { return detectCI(root, inspect(root)) }

func detectCI(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "ci", conf, ev, "")) }

	if st, err := os.Stat(filepath.Join(root, ".github", "workflows")); err == nil && st.IsDir() {
		add("GitHub Actions", confHigh, fileSig(".github/workflows", ".github/workflows"))
	}
	if insp.rootFileSet[".gitlab-ci.yml"] {
		add("GitLab CI", confHigh, fileSig(".gitlab-ci.yml", ".gitlab-ci.yml"))
	}
	if insp.rootFileSet["Jenkinsfile"] {
		add("Jenkins", confHigh, fileSig("Jenkinsfile", "Jenkinsfile"))
	}
	if insp.rootFileSet["azure-pipelines.yml"] {
		add("Azure Pipelines", confHigh, fileSig("azure-pipelines.yml", "azure-pipelines.yml"))
	}
	if st, err := os.Stat(filepath.Join(root, ".circleci", "config.yml")); err == nil && !st.IsDir() {
		add("CircleCI", confHigh, fileSig(".circleci/config.yml", ".circleci/config.yml"))
	}
	if insp.rootFileSet[".travis.yml"] {
		add("Travis CI", confMedium, fileSig(".travis.yml", ".travis.yml"))
	}
	return out
}
