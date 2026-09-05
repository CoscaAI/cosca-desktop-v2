package projectintel

// Testes do cache INCREMENTAL (missão §27): o Desktop NÃO deve re-scanear o
// projeto a cada análise; recalcula SÓ quando arquivos relevantes mudam. O cache
// vive em memória (nunca .cosca/memory/knowledge/family) e é read-only.

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// TestCache_HitShortCircuits prova o cache hit: cacheando um root (go.mod+src),
// a 2ª chamada devolve a MESMA instância do perfil — provando que NÃO reanalisou.
// Para provar de forma determinística, substituímos a relógio global TimeNow por
// um valor fixo: se a 2ª chamada reanalisasse, DetectedAt mudaria; no hit, ele
// permanece o original (detectado na 1ª vez).
func TestCache_HitShortCircuits(t *testing.T) {
	root := fixture(t, map[string]string{
		"go.mod":       "module example.com/foo\n\ngo 1.25\n",
		"src/main.go":  "package main\n\nfunc main(){}\n",
	})
	c := NewCache()

	p1, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("GetOrAnalyze: %v", err)
	}
	if p1 == nil {
		t.Fatal("perfil não deveria ser nil")
	}

	// Troca o relógio: un perfil re-analisado teria DetectedAt = 2030-...;
	// um cache hit preserva o DetectedAt original (mesma instância).
	restore := TimeNow
	TimeNow = func() string { return "2030-01-01T00:00:00Z" }
	defer func() { TimeNow = restore }()

	p2, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("GetOrAnalyze (2ª): %v", err)
	}
	if p2 != p1 {
		t.Errorf("cache hit deveria retornar a MESMA instância do perfil (re-analisou?)")
	}
	if p2.DetectedAt != p1.DetectedAt {
		t.Errorf("cache hit deveria preservar DetectedAt original %q, got %q (re-analisou)", p1.DetectedAt, p2.DetectedAt)
	}
}

// TestCache_Invalidate prova que Invalidate(root) força uma re-análise: após
// invalidar, a próxima GetOrAnalyze devolve um perfil DISTINTO com DetectedAt
// novo (o relógio fixo de 2030-... entra na re-análise).
func TestCache_Invalidate(t *testing.T) {
	root := fixture(t, map[string]string{
		"go.mod":      "module example.com/foo\n\ngo 1.25\n",
		"main.go":     "package main\n\nfunc main(){}\n",
	})
	c := NewCache()
	p1, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("GetOrAnalyze: %v", err)
	}

	restore := TimeNow
	TimeNow = func() string { return "2030-01-01T00:00:00Z" }
	defer func() { TimeNow = restore }()

	c.Invalidate(root)
	p2, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("GetOrAnalyze após invalidar: %v", err)
	}
	if p2 == p1 {
		t.Errorf("após Invalidate, a instância deveria ser NOVA (re-analisou)")
	}
	if p2.DetectedAt != "2030-01-01T00:00:00Z" {
		t.Errorf("após Invalidate deveria re-analisar com o novo relógio; DetectedAt=%q, want 2030-...", p2.DetectedAt)
	}
}

// TestCache_RelevantFiles prova que relevantFiles detecta os arquivos que importam
// p/ detecção (package.json, go.mod, Dockerfile) e IGNORA node_modules e .vscode
// (e não lista arquivos-fonte irrelevantes como main.go).
func TestCache_RelevantFiles(t *testing.T) {
	root := fixture(t, map[string]string{
		"package.json":              `{"name":"x"}`,
		"go.mod":                    "module x\n\ngo 1.25\n",
		"Dockerfile":                "FROM node:20\n",
		"src/main.go":               "package main\n\nfunc main(){}\n",
		"node_modules/foo/index.js": "console.log(1)",
		".vscode/settings.json":     `{}`,
	})
	files := relevantFiles(root)

	for _, want := range []string{"package.json", "go.mod", "Dockerfile"} {
		if _, ok := files[want]; !ok {
			t.Errorf("relevantFiles deveria conter %q; got %v", want, keysOf(files))
		}
	}
	// node_modules e .vscode NÃO podem aparecer (dirs excluídos/ignorados).
	for _, no := range []string{"node_modules/foo/index.js", ".vscode/settings.json"} {
		if _, ok := files[no]; ok {
			t.Errorf("relevantFiles NÃO deveria conter %q; got %v", no, keysOf(files))
		}
	}
	// Arquivo-fonte irrelevante não entra no fingerprint.
	if _, ok := files["src/main.go"]; ok {
		t.Errorf("relevantFiles NÃO deveria conter arquivo-fonte src/main.go; got %v", keysOf(files))
	}
}

// TestGetOrAnalyze_DifferentRoots prova que o cache é por root: duas raízes
// distintas não colidem (perfis independentes e cada uma com seu cache hit).
func TestGetOrAnalyze_DifferentRoots(t *testing.T) {
	rootA := fixture(t, map[string]string{
		"go.mod":    "module example.com/a\n\ngo 1.25\n",
		"main.go":   "package main\n\nfunc main(){}\n",
	})
	rootB := fixture(t, map[string]string{
		"pyproject.toml": "[project]\nname='b'\n",
		"app/main.py":    "print('b')\n",
	})
	c := NewCache()
	pa, err := c.GetOrAnalyze(rootA)
	if err != nil {
		t.Fatalf("rootA: %v", err)
	}
	pb, err := c.GetOrAnalyze(rootB)
	if err != nil {
		t.Fatalf("rootB: %v", err)
	}
	if pa == pb {
		t.Errorf("raízes distintas NÃO podem compartilhar a mesma instância de perfil")
	}
	if pa.Root != rootA || pb.Root != rootB {
		t.Errorf("cada perfil deveria apontar para a SUA raiz; pa.Root=%q pb.Root=%q", pa.Root, pb.Root)
	}
	// Cache hit em A continua retornando a instância de A.
	pa2, err := c.GetOrAnalyze(rootA)
	if err != nil {
		t.Fatalf("rootA (hit): %v", err)
	}
	if pa2 != pa {
		t.Errorf("cache hit em A deveria retornar a mesma instância de A")
	}
}

// keysOf devolve as chaves ordenadas de um mapa (para mensagens de erro legíveis).
func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestCache_Invalidate_MissingProofToDisco garante que o cache é apenas memória:
// Invalidate em um root inexistente não cria estado, e GetOrAnalyze sobre um
// path inválido propaga o erro do Analyze (de forma honesta) sem cachear.
func TestCache_Invalidate_MissingRoot(t *testing.T) {
	c := NewCache()
	gone := filepath.Join(t.TempDir(), "nao-existe")
	c.Invalidate(gone) // não deve panicar nem criar estado

	if _, err := c.GetOrAnalyze(gone); err == nil {
		t.Errorf("GetOrAnalyze em root inexistente deveria retornar erro (não inventar perfil)")
	}
	// Invalidate com cache vazio é no-op (não panic).
	c.Invalidate(gone)
	_ = os.RemoveAll(gone)
}

// TestCache_RelevantChange_Invalidates prova o caso C: alterar um arquivo que
// INFLUENCIA a detecção (go.mod) muda o fingerprint → invalida → reanalisa.
// Usamos o relógio fixo para detectar a re-análise: se DetectedAt mudar, houve
// reanálise (prova que a mudança relevante invalidou o cache).
func TestCache_RelevantChange_Invalidates(t *testing.T) {
	root := fixture(t, map[string]string{
		"go.mod":   "module example.com/foo\n\ngo 1.25\n",
		"main.go":  "package main\n\nfunc main(){}\n",
	})
	c := NewCache()
	p1, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("1ª: %v", err)
	}
	if p1 == nil {
		t.Fatal("perfil nil")
	}

	restore := TimeNow
	TimeNow = func() string { return "2031-01-01T00:00:00Z" }
	defer func() { TimeNow = restore }()

	// Muda um arquivo RELEVANTE: adiciona uma dependência no go.mod.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/foo\n\nrequire github.com/gin-gonic/gin v1.9.0\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	p2, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("2ª: %v", err)
	}
	if p2 == p1 {
		t.Errorf("mudança relevante (go.mod) deveria invalidar o cache e reanalisar (instância deveria ser nova)")
	}
	if p2.DetectedAt != "2031-01-01T00:00:00Z" {
		t.Errorf("mudança relevante deveria reanalisar com o novo relógio; DetectedAt=%q", p2.DetectedAt)
	}
}

// TestCache_IrrelevantChange_NoReanalyze prova o caso D: mudar um arquivo que
// NÃO participa da detecção (um .txt de notas) NÃO muda os arquivos relevantes,
// então o cache hit deve persistir (mesma instância, sem reanálise).
func TestCache_IrrelevantChange_NoReanalyze(t *testing.T) {
	root := fixture(t, map[string]string{
		"go.mod":     "module example.com/foo\n\ngo 1.25\n",
		"main.go":    "package main\n\nfunc main(){}\n",
		"notes.txt":  "not relevant",
	})
	c := NewCache()
	p1, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("1ª: %v", err)
	}

	restore := TimeNow
	TimeNow = func() string { return "2032-01-01T00:00:00Z" }
	defer func() { TimeNow = restore }()

	// Muda apenas um arquivo IRRELEVANTE (.txt de notas).
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("changed but irrelevant\nlonger content\n"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}
	// Acrescenta um arquivo fonte (não é "relevante" p/ fingerprint).
	if err := os.WriteFile(filepath.Join(root, "other.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write other.go: %v", err)
	}
	p2, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("2ª: %v", err)
	}
	if p2 != p1 {
		t.Errorf("mudança irrelevante NÃO deveria reanalisar (deveria manter a mesma instância)")
	}
	if p2.DetectedAt != p1.DetectedAt {
		t.Errorf("mudança irrelevante deveria preservar DetectedAt; got %q want %q", p2.DetectedAt, p1.DetectedAt)
	}
}

// TestCache_ConcurrentAccess_Safe prova o caso G: chamadas simultâneas em
// vários roots não produzem estado inconsistente nem panic. O cache é
// thread-safe (mutex). Concorrência com leitura+escrita no mesmo root também
// deve ser segura (nunca retornar perfil corrompido/parcial).
func TestCache_ConcurrentAccess_Safe(t *testing.T) {
	roots := make([]string, 6)
	for i := range roots {
		goMod := "module example.com/x" + string(rune('a'+i)) + "\n\ngo 1.25\n"
		roots[i] = fixture(t, map[string]string{
			"go.mod":  goMod,
			"main.go": "package main\n\nfunc main(){}\n",
		})
	}
	c := NewCache()

	var wg sync.WaitGroup
	var errCount int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				root := roots[(i+j)%len(roots)]
				p, err := c.GetOrAnalyze(root)
				if err != nil {
					atomic.AddInt32(&errCount, 1)
					return
				}
				if p == nil || p.Root != root {
					atomic.AddInt32(&errCount, 1)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	if errCount != 0 {
		t.Errorf("concorrência produziu %d erro(s)/inconsistência(s)", errCount)
	}
}

// TestCache_FirstAnalyze_RegistersFingerprint prova o caso A (primeira análise):
// numa chamada inédita o cache executa Analyze, registra o fingerprint e produz
// um perfil com DetectedAt preenchido. A 2ª chamada (sem mudança) reaproveita.
func TestCache_FirstAnalyze_RegistersFingerprint(t *testing.T) {
	root := fixture(t, map[string]string{
		"go.mod":  "module example.com/foo\n\ngo 1.25\n",
		"main.go": "package main\n\nfunc main(){}\n",
	})
	c := NewCache()
	p1, err := c.GetOrAnalyze(root)
	if err != nil {
		t.Fatalf("1ª: %v", err)
	}
	if p1 == nil || p1.DetectedAt == "" {
		t.Fatalf("1ª análise deveria produzir perfil com DetectedAt preenchido")
	}
	// Verifica que o fingerprint de fato foi registrado (entrada de cache existe).
	c.mu.Lock()
	e, ok := c.entries[root]
	c.mu.Unlock()
	if !ok || e == nil {
		t.Fatalf("fingerprint/entrada de cache não foi registrada após 1ª análise")
	}
	if e.files == nil {
		t.Errorf("entrada de cache deveria guardar o conjunto de arquivos relevantes")
	}
}
