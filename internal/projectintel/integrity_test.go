package projectintel

// Teste de INTEGRIDADE do Project Intelligence (missão FASE 3): confirma que
// TODOS os campos esperados do ProjectProfile são entregues e que a detecção é
// por evidência (nenhuma invenção). Roda sobre a RAIZ REAL do cosca
// (C:\Users\Henrique\Documents\cosca), que é um projeto Go rico (go.mod,
// Makefile, docs/, cmd/, internal/, .cosca), para validar um caso real.

import (
	"os"
	"testing"
)

// TestPI_Integrity_RealCoscaRoot analisa o projeto real do COSCA (raiz) e
// confirma que o perfil é populado com evidência e que nenhum campo essencial
// está ausente. Este é um teste de integridade end-to-end.
func TestPI_Integrity_RealCoscaRoot(t *testing.T) {
	root := "C:\\Users\\Henrique\\Documents\\cosca"
	if _, err := os.Stat(root); err != nil {
		t.Skipf("raiz do cosca não acessível neste ambiente: %v", err)
	}
	p, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze(cosca root) erro: %v", err)
	}
	if p == nil {
		t.Fatal("perfil nil")
	}

	// Campos estruturais não podem ser vazios.
	if p.Root == "" || p.Name == "" || p.DetectedAt == "" {
		t.Errorf("root/name/detected_at não podem ser vazios: root=%q name=%q at=%q", p.Root, p.Name, p.DetectedAt)
	}
	if p.ProjectTypes == nil || len(p.ProjectTypes) == 0 {
		t.Errorf("ProjectTypes não deveria ser vazio num projeto Go real")
	}
	for _, ft := range p.ConfigFiles {
		if ft == "" {
			t.Errorf("ConfigFiles contém item vazio")
		}
	}

	// Detecção por evidência: toda Tech precisa ter almeno 1 evidência e a
	// Language deve incluir Go (é um projeto Go real) — mas SÓ se houver evidência.
	foundGo := false
	for _, l := range p.Languages {
		if l.Name != "Go" {
			continue
		}
		foundGo = true
		if l.Confidence == "" {
			t.Errorf("Go deveria ter confiança preenchida")
		}
		if len(l.Evidence) == 0 {
			t.Errorf("Go deveria ter evidência (não invenção)")
		}
	}
	if !foundGo {
		t.Errorf("projeto cosca é Go; deveria detectar Go. languages=%v", techNames(p.Languages))
	}

	// Nenhuma Tech sem evidência (regra: nada inventado).
	for _, cat := range allTechLists(p) {
		for _, tech := range cat {
			if len(tech.Evidence) == 0 {
				t.Errorf("Tech %q (cat %q) sem evidência — parece invenção", tech.Name, tech.Category)
			}
			if tech.Confidence == "" {
				t.Errorf("Tech %q sem confiança", tech.Name)
			}
		}
	}

	// Build tool: projeto Go tem Makefile/go.mod.
	if len(p.BuildTools) == 0 {
		t.Errorf("projeto Go com go.mod/Makefile deveria ter BuildTools")
	}
	// Documentation: o cosca tem README.md, docs/.
	if len(p.Documentation) == 0 {
		t.Errorf("projeto cosca tem README/docs; deveria ter Documentation")
	}
	// ConfigFiles deve incluir go.mod.
	if !hasStr(p.ConfigFiles, "go.mod") {
		t.Errorf("ConfigFiles deveria conter go.mod; got %v", p.ConfigFiles)
	}
}

// TestPI_Fields_AgentContextAndPipeline valida os métodos derivados
// (AgentSkillMatch, AgentPipeline) sobre um perfil Go real, garantindo que
// continuam coerentes (missão FASE 3).
func TestPI_Fields_AgentContextAndPipeline(t *testing.T) {
	p := &ProjectProfile{
		Languages:       []Tech{{Name: "Go", Category: "language", Confidence: "high", Evidence: []Signal{{Kind: "file", Value: "go.mod"}}}},
		BuildTools:      []Tech{{Name: "go", Category: "buildtool", Confidence: "high", Evidence: []Signal{{Kind: "file", Value: "go.mod"}}}},
		TestTools:       []Tech{{Name: "go", Category: "testtool", Confidence: "high", Evidence: []Signal{{Kind: "count", Value: "n"}}}},
		Formatters:      []Tech{{Name: "gofmt", Category: "formatter", Confidence: "medium", Evidence: []Signal{{Kind: "file", Value: "go.mod"}}}},
		Linters:         []Tech{{Name: "go vet", Category: "linter", Confidence: "medium", Evidence: []Signal{{Kind: "file", Value: "go.mod"}}}},
		TypeCheckers:    []Tech{},
		Repository:      []Tech{{Name: "Git", Category: "vcs", Confidence: "high", Evidence: []Signal{{Kind: "file", Value: ".git"}}}},
	}

	// Skill: Go deve casar com "go".
	skills := AgentSkillMatch(p)
	if !hasStr(skills, "go") {
		t.Errorf("AgentSkillMatch deveria conter 'go' para um perfil Go; got %v", skills)
	}

	// Pipeline: Go → format=gofmt, lint=go vet, typecheck=N/A, test=go test, build=go build.
	var fmtStep, lintStep, tcStep, testStep, buildStep string
	for _, st := range AgentPipeline(p) {
		switch st["step"] {
		case "format":
			fmtStep = st["tool"]
		case "lint":
			lintStep = st["tool"]
		case "typecheck":
			tcStep = st["tool"]
		case "test":
			testStep = st["tool"]
		case "build":
			buildStep = st["tool"]
		}
	}
	if fmtStep != "gofmt" {
		t.Errorf("pipeline Go format deveria ser gofmt, got %q", fmtStep)
	}
	if lintStep != "go vet" {
		t.Errorf("pipeline Go lint deveria ser go vet, got %q", lintStep)
	}
	if tcStep != "NOT_APPLICABLE" {
		t.Errorf("pipeline Go typecheck deveria ser NOT_APPLICABLE (sem typechecker), got %q", tcStep)
	}
	if testStep != "go test" {
		t.Errorf("pipeline Go test deveria ser go test, got %q", testStep)
	}
	if buildStep != "go build" {
		t.Errorf("pipeline Go build deveria ser go build, got %q", buildStep)
	}
}

func techNames(list []Tech) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, t.Name)
	}
	return out
}

func hasStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
