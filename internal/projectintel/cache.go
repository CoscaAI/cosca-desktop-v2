package projectintel

// CATEGORY: cache.go — cache INCREMENTAL em memória do Project Intelligence
// (missão §27). Evita re-scanear o projeto inteiro a cada abertura/troca de aba;
// recalcula SÓ quando os arquivos RELEVANTES mudam.
//
// REGRAS:
//   - Vive 100% na MEMÓRIA do processo (var/struct do Desktop), NUNCA em .cosca,
//     memory, knowledge, family. NADA institucional é persistido aqui.
//   - É uma camada OPcional que envolve Analyze — não altera a lógica de detecção.
//   - Read-only e thread-safe: nunca escreve no root.
//   - Cache hit devolve a MESMA instância do perfil (DetectedAt original = quando
//     foi detectado pela 1ª vez), o que serve de observabilidade (missão §29).

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// cacheEntry é o estado cacheado de um root: o conjunto de arquivos relevantes
// no momento do scan, o perfil resultante e o instante do último acesso (LRU).
type cacheEntry struct {
	files   map[string]string
	profile *ProjectProfile
	at      time.Time
}

// Cache é um cache em memória por root (incremental + LRU). É a camada que o
// Desktop usa via GetOrAnalyze/Invalidate — nunca passa pelo kernel/memória.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
	max     int
}

// NewCache cria um Cache com um limite de projetos (default 50).
func NewCache() *Cache {
	return &Cache{entries: map[string]*cacheEntry{}, max: 50}
}

// ---------------------------------------------------------------------------
// Arquivos RELEVANTES (missão §27) — o fingerprint do que importa p/ detecção
// ---------------------------------------------------------------------------

// relevantExact são basenames relevantes (case-insensitive) reconhecidos em
// QUALQUER nível da árvore (ex.: apps/web/package.json em monorepo).
var relevantExact = map[string]bool{
	"package.json": true,
	"go.mod":       true,
	"go.sum":       true,
	"Cargo.toml":   true,
	"pyproject.toml": true,
	"requirements.txt": true,
	"biome.json":       true,
	".editorconfig":    true,
	"pnpm-workspace.yaml": true,
	"turbo.json":          true,
	"nx.json":             true,
	"lerna.json":          true,
	"go.work":             true,
	".gitlab-ci.yml":      true,
	"Jenkinsfile":         true,
	".gitignore":          true,
	".nvmrc":              true,
	"Makefile":            true,
	"Justfile":            true,
	"Taskfile.yml":        true,
	"pom.xml":             true,
}

// relevantGlobs são padrões de basename relevantes (case-insensitive, via
// filepath.Match). Dockerfile*, compose.*, tsconfig.*, eslint.*, .prettierrc*,
// prettier.config.*, build.gradle*, *.csproj + docker-compose (variante comum).
var relevantGlobs = []string{
	"dockerfile*",
	"compose.*",
	"docker-compose.*",
	"tsconfig.*",
	"eslint.*",
	".prettierrc*",
	"prettier.config.*",
	"build.gradle*",
	"*.csproj",
}

// relevantFiles coleciona os arquivos RELEVANTES do root e devolve
// path(relativo, slash) -> fingerprint do conteúdo/tamanho+mtime. NÃO desce em
// node_modules/.git/vendor (e demais dirs excluídos) e NÃO toca .cosca.
func relevantFiles(root string) map[string]string {
	out := map[string]string{}
	var rec func(dir string, depth int)
	rec = func(dir string, depth int) {
		if depth > maxScanDepth {
			return
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			rel, err := filepath.Rel(root, filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if e.IsDir() {
				name := e.Name()
				if excludedDirs[name] {
					continue
				}
				// Só desce em dir oculto relevantes p/ CI (.github); os demais
				// ocultos (.vscode, .idea...) são ignorados por serem irrelevantes.
				if strings.HasPrefix(name, ".") && name != ".github" {
					continue
				}
				rec(filepath.Join(dir, name), depth+1)
				continue
			}
			if !isRelevantFile(rel, e.Name()) {
				continue
			}
			if v := fileFingerprint(filepath.Join(dir, e.Name())); v != "" {
				out[rel] = v
			}
		}
	}
	rec(root, 0)
	return out
}

// isRelevantFile decide se um arquivo (rel em slash + basename) é relevante.
func isRelevantFile(rel, base string) bool {
	lower := strings.ToLower(base)
	if relevantExact[lower] {
		return true
	}
	for _, g := range relevantGlobs {
		if ok, _ := filepath.Match(g, lower); ok {
			return true
		}
	}
	// CI: qualquer arquivo dentro de .github/workflows/.
	if strings.Contains(strings.ToLower(rel), ".github/workflows/") {
		return true
	}
	return false
}

// fileFingerprint gera um identificador de conteúdo: sha1 do conteúdo quando o
// arquivo é pequeno (manifests/config), senão "size:mtimeUnixNano".
func fileFingerprint(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if st.Size() > maxReadSize {
		return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
	}
	sum := sha1.Sum(b)
	return fmt.Sprintf("%x", sum[:])
}

// fingerprint devolve um hash determinístico (sha256) do conjunto ORDENADO de
// arquivos relevantes. "empty" quando não há nenhum arquivo relevante.
func fingerprint(files map[string]string) string {
	if len(files) == 0 {
		return "empty"
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(files[k]))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// sameFiles compara dois conjuntos de arquivos relevantes (mesma fingerprint).
func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// API do cache
// ---------------------------------------------------------------------------

// GetOrAnalyze devolve o perfil do root, usando o cache quando possível: se os
// arquivos relevantes NÃO mudaram desde o último scan, retorna a MESMA instância
// do perfil (DetectedAt original). Senão reanalisa (Analyze) e atualiza o cache
// (evict LRU se estourar o limite).
func (c *Cache) GetOrAnalyze(root string) (*ProjectProfile, error) {
	files := relevantFiles(root)
	c.mu.Lock()
	if e, ok := c.entries[root]; ok && sameFiles(e.files, files) {
		e.at = time.Now() // último acesso (LRU)
		prof := e.profile
		c.mu.Unlock()
		return prof, nil
	}
	c.mu.Unlock()

	prof, err := Analyze(root)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.entries[root] = &cacheEntry{files: files, profile: prof, at: time.Now()}
	c.evictLocked()
	c.mu.Unlock()
	return prof, nil
}

// Invalidate descarta o cache de um root — a próxima GetOrAnalyze reanalisa.
func (c *Cache) Invalidate(root string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, root)
}

// evictLocked remove o artigo menos recentemente usado quando o cache estoura o
// limite. Deve ser chamado com c.mu travado.
func (c *Cache) evictLocked() {
	if c.max <= 0 {
		return
	}
	for len(c.entries) > c.max {
		var oldRoot string
		var oldAt time.Time
		for r, e := range c.entries {
			if oldRoot == "" || e.at.Before(oldAt) {
				oldRoot = r
				oldAt = e.at
			}
		}
		if oldRoot == "" {
			return
		}
		delete(c.entries, oldRoot)
	}
}
