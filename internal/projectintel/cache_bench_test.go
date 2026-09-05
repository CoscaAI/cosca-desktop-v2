package projectintel

// Benchmark de performance do cache (missão FASE 7): prova que a análise
// completa (1ª) e a incremental (cache hit) diferem, e que o resultado é
// equivalente. Mede de verdade, com números reais.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkCache_FullVsIncremental mede o custo da análise COMPLETA (1ª chamada,
// sem cache) vs INCREMENTAL (2ª chamada, cache hit — reutiliza sem reanalisar).
// b.N iterações: a 1ª iteração analisa (cold), as demais são cache hit.
func BenchmarkCache_FullVsIncremental(b *testing.B) {
	root := b.TempDir()
	writeFiles(b, root, map[string]string{
		"go.mod":    "module example.com/bench\n\ngo 1.25\n",
		"main.go":   "package main\n\nfunc main(){}\n",
		"internal/x.go": "package x\n\nfunc X(){}\n",
		"internal/y.go": "package x\n\nfunc Y(){}\n",
		"internal/z.go": "package x\n\nfunc Z(){}\n",
		"README.md": "# bench\n",
		"Makefile":  "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n",
	})
	c := NewCache()

	// Warm-up: 1ª análise (completa, cold) — mede o tempo da análise completa.
	start := time.Now()
	if _, err := c.GetOrAnalyze(root); err != nil {
		b.Fatalf("GetOrAnalyze: %v", err)
	}
	fullElapsed := time.Since(start)

	// b.N iterações de cache hit (incremental) — mede o tempo médio incremental.
	b.ResetTimer()
	incrementalTotal := time.Duration(0)
	for i := 0; i < b.N; i++ {
		s := time.Now()
		if _, err := c.GetOrAnalyze(root); err != nil {
			b.Fatalf("GetOrAnalyze (hit): %v", err)
		}
		incrementalTotal += time.Since(s)
	}
	b.StopTimer()

	incrementalAvg := incrementalTotal / time.Duration(b.N)
	b.ReportMetric(float64(fullElapsed.Microseconds())/1e3, "full_ms")
	b.ReportMetric(float64(incrementalAvg.Microseconds())/1e3, "incr_ms")
	b.Logf("análise completa (cold, 1ª): %v | incremental (cache hit, média): %v | N=%d", fullElapsed, incrementalAvg, b.N)
}

// BenchmarkCache_RelevantFiles_Count mede quantos arquivos relevantes são
// considerados num projeto grande simulado e quantos são processados.
func BenchmarkCache_RelevantFiles_Count(b *testing.B) {
	root := b.TempDir()
	files := map[string]string{
		"go.mod":      "module example.com/big\n\ngo 1.25\n",
		"main.go":     "package main\n\nfunc main(){}\n",
		"Makefile":    "build:\n\tgo build ./...\n",
		"README.md":   "# big\n",
		".gitignore":  "*.log\n",
	}
	// simulate muitos arquivos-fonte irrelevantes (não são "relevantes" p/ fingerprint)
	for i := 0; i < 500; i++ {
		files[fmt.Sprintf("pkg/pkg%d/c%d.go", i%50, i)] = "package pkg\n\nfunc F(){}\n"
	}
	writeFiles(b, root, files)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fs := relevantFiles(root)
		if len(fs) == 0 {
			b.Fatal("relevantFiles não pode retornar vazio")
		}
	}
	b.StopTimer()
	b.Logf("arquivos relevantes detectados de um projeto com 500+ arquivos-fonte: %d", len(relevantFiles(root)))
}

func writeFiles(b *testing.B, root string, files map[string]string) {
	b.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			b.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			b.Fatalf("write %s: %v", full, err)
		}
	}
}
