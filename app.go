package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"cosca-desktop/internal/projectintel"
	"cosca-desktop/internal/uiperception"
)

// App é o backend Wails do COSCA Desktop (Engineering Workspace).
//
// PROJECT-FIRST: a unidade operacional é o projeto. O primeiro vertical slice
// é: Home → Create Project → cosca init (REAL) → ProjectContext → Workspace.
// Serviços de agent/knowledge/memory são REQUIRED LATER (não criam estado
// antes de existir projeto).
type App struct {
	ctx             context.Context
	project         *Project // projeto ativo (nil = sem projeto, Home)
	runtime         string
	pendingApproval *Approval // aprovação local atual (opção B — ver abaixo)

	// Cache INCREMENTAL do Project Intelligence (missão §27): vive na MEMÓRIA do
	// processo, nunca em .cosca/memory/knowledge/family. Evita re-scanear o
	// projeto a cada análise; recalcula só quando arquivos relevantes mudam.
	piCache *projectintel.Cache

	// ── FASE A (ADR-0007): modo de operação definido por DETECÇÃO do root ──
	// Cosca Desktop é um produto independente com dois modos:
	//   - STANDALONE (fora do root): Agents + Skills, SEM kernel/memória da família.
	//   - FORGE (no root): entra o KERNEL e revela TUDO do root (agents, skills,
	//     memória da família, config, arquitetura, family chain, DNA).
	// A detecção é por LEITURA do disco (nunca cópia de conteúdo para o binário;
	// não carregamos memória da família para o estado — apenas sinalizamos).
	rootMode bool   // true = FORGE (no root do framework Cosca), false = STANDALONE
	rootDir  string // diretório ativo usado na detecção (projeto ativo ou cwd)

	// Buffer de eventos do runtime (canal Desktop→UI). O backend coleta/emite
	// eventos estruturados e a UI assina via Wails EventsOn (não polling).
	events   []RuntimeEvent
	eventsMu sync.Mutex
	eventSeq int

	// ── CHAT (FASE 3 — Kernel-First /v1/run/stream) ────────────────────────────
	// Mapa de cancelamento dos streams de chat EM ANDAMENTO (runID → cancel func).
	// O frontend chama CancelChatStream(runID) para abortar o SSE; o cancel
	// propaga ao context do request HTTP (mesmo padrão do cancelamento do daemon,
	// que reusa r.Context()). A UI assina `cosca:chat:event` para o streaming
	// token-a-token; este mapa é apenas o MAÇO de cancelamento — não carrega
	// lógica cognitiva nenhuma.
	chatCancelMu sync.Mutex
	chatCancel   map[string]context.CancelFunc
}

// Project é o contexto do projeto (raiz contextual da aplicação).
type Project struct {
	Name        string `json:"name"`
	Root        string `json:"root"`
	Initialized bool   `json:"initialized"`
	HasCosca    bool   `json:"has_cosca"`
	InitedAt    string `json:"inited_at"`
	Error       string `json:"error,omitempty"`
}

// NewApp cria a instância.
func NewApp() *App {
	return &App{
		runtime:    resolveRuntime(),
		piCache:    projectintel.NewCache(),
		chatCancel: make(map[string]context.CancelFunc),
	}
}

// startup é chamado pelo Wails.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.runtime = resolveRuntime()
	// Se executado dentro de um projeto (env), tenta descobrir.
	if p := os.Getenv("COSCA_PROJECT"); p != "" {
		a.project = describeProject(p)
		a.project.Root = p
	}
	// Detecção do root (FORGE vs STANDALONE) roda no startup do Wails.
	a.DetectRoot()
}

func resolveRuntime() string {
	if r := os.Getenv("COSCA_RUNTIME"); r != "" {
		return r
	}
	for _, name := range []string{"cosca.exe", "cosca"} {
		if b, err := exec.LookPath("cosca.exe"); err == nil && name == "cosca.exe" {
			return b
		}
		if b, err := exec.LookPath("cosca"); err == nil {
			return b
		}
	}
	// fallback: caminho conhecido do repo de dev
	if b, err := exec.LookPath("cosca"); err == nil {
		return b
	}
	return "cosca"
}

// ---------------------------------------------------------------------------
// PROJECT SERVICE — primeiro fluxo (Create Project → cosca init → context)
// ---------------------------------------------------------------------------

// DiscoverProjects descobre projetos existentes em uma localização.
// Retorna os que têm um diretório (.cosca ou git) — para a Home.
func (a *App) DiscoverProjects(location string) []Project {
	location = expandHome(location)
	out := []Project{}
	entries, err := os.ReadDir(location)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(location, e.Name())
		if isProjectDir(p) {
			out = append(out, *describeProject(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ValidateLocation confirma que a localização base existe (para criar projeto).
func (a *App) ValidateLocation(location string) (string, error) {
	location = expandHome(location)
	st, err := os.Stat(location)
	if err != nil {
		return "", fmt.Errorf("localização não existe: %s", location)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("localização não é um diretório: %s", location)
	}
	if !isWritable(location) {
		return "", fmt.Errorf("localização não é gravável: %s", location)
	}
	return location, nil
}

// CreateProject cria o projeto: diretório + identidade, e então executa o
// cosca init REAL. Retorna o ProjectContext.
func (a *App) CreateProject(name, location string) (Project, error) {
	name = strings.TrimSpace(name)
	// Validação do nome ANTES de qualquer criação de diretório: impede
	// path traversal ("../evil"), separadores, nomes reservados do Windows
	// e caracteres inválidos — nada é criado fora da base.
	if err := validateProjectName(name); err != nil {
		return Project{}, err
	}
	loc, err := a.ValidateLocation(location)
	if err != nil {
		return Project{}, err
	}
	root := filepath.Join(loc, name)
	if _, err := os.Stat(root); err == nil {
		return Project{}, fmt.Errorf("já existe: %s", root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Project{}, fmt.Errorf("criar diretório: %w", err)
	}
	// Ocupa o projeto ativo.
	a.project = &Project{Name: name, Root: root}
	// Executa o cosca init real.
	return a.InitProject()
}

// InitProject roda o `cosca init` real no diretório do projeto e valida.
//
// POLÍTICA (ADR-0003): `cosca init` é uma operação de SETUP/leitura (não executa
// trabalho de agente). Como no Windows o runtime não tem bwrap, o opt-in
// envAllowNoRoot é injetado apenas para o runtime não negar fail-closed — ele
// NÃO autoriza nada; quem decide o que o `init` cria é o próprio runtime. A
// autoridade permanece no Cosca (execpolicy/approval), nunca no Desktop.
func (a *App) InitProject() (Project, error) {
	if a.project == nil {
		return Project{}, fmt.Errorf("sem projeto ativo")
	}
	root := a.project.Root
	cmd := exec.CommandContext(context.Background(), a.runtime, "init")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), envAllowNoRoot+"="+policyAllowNoRoot)
	out, err := cmd.CombinedOutput()
	desc := describeProject(root)
	desc.Name = a.project.Name
	desc.Root = root
	if err != nil {
		desc.Error = strings.TrimSpace(string(out)) + " [" + err.Error() + "]"
		a.project = desc
		a.DetectRoot()
		return *desc, nil // não propaga erro fatal; relata no desc
	}
	desc.Initialized = true
	desc.HasCosca = hasCoscaDir(root)
	desc.InitedAt = time.Now().Format(time.RFC3339)
	a.project = desc
	a.DetectRoot()
	return *desc, nil
}

// OpenProject abre um projeto existente (a partir da Home) e valida.
func (a *App) OpenProject(root string) (Project, error) {
	root = expandHome(root)
	if !isProjectDir(root) {
		// tenta marcar como projeto não inicializado
		if _, err := os.Stat(root); err != nil {
			return Project{}, fmt.Errorf("projeto não encontrado: %s", root)
		}
	}
	desc := describeProject(root)
	desc.Initialized = hasCoscaDir(root)
	a.project = desc
	a.DetectRoot()
	return *desc, nil
}

// ProjectContext retorna o projeto ativo (ou nil => sem projeto).
func (a *App) ProjectContext() *Project { return a.project }

// HasProject indica se há projeto ativo.
func (a *App) HasProject() bool { return a.project != nil }

// CloseProject sai do projeto (volta para Home).
func (a *App) CloseProject() {
	a.project = nil
	a.DetectRoot() // re-detecta contra o diretório ativo (cwd) sem projeto
}

// ---------------------------------------------------------------------------
// helpers de projeto
// ---------------------------------------------------------------------------

// validateProjectName rejeita nomes de projeto inseguros/inválidos antes de
// qualquer criação de diretório. Regras (fail-closed):
//   - vazio
//   - contém ".." (path traversal)
//   - contém separador de caminho ("/" ou "\")
//   - é um nome reservado do Windows (CON, PRN, AUX, NUL, COM1-9, LPT1-9)
//   - termina com espaço ou ponto
//   - contém caracteres inválidos de arquivo do Windows (<>:"|?* ou \x00)
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("nome do projeto vazio")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("nome do projeto não pode conter \"..\": %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("nome do projeto não pode conter separadores de caminho: %q", name)
	}
	// Nomes reservados do Windows (case-insensitive, nomes de dispositivo).
	base := strings.ToUpper(name)
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("nome do projeto é um nome reservado do Windows: %q", name)
	}
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("nome do projeto não pode terminar com espaço ou ponto: %q", name)
	}
	if strings.ContainsAny(name, `<>:"|?*`) || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("nome do projeto contém caracteres inválidos: %q", name)
	}
	return nil
}

func expandHome(p string) string {
	if p == "~" {
		h, _ := os.UserHomeDir()
		return h
	}
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, strings.TrimPrefix(p, "~/"))
	}
	return p
}

func isWritable(p string) bool {
	f, err := os.CreateTemp(p, ".cosca-w-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func isProjectDir(p string) bool {
	return hasCoscaDir(p) || hasGitDir(p)
}
func hasCoscaDir(p string) bool {
	st, err := os.Stat(filepath.Join(p, ".cosca"))
	return err == nil && st.IsDir()
}
func hasGitDir(p string) bool {
	st, err := os.Stat(filepath.Join(p, ".git"))
	return err == nil && st.IsDir()
}

// describeProject descreve um diretório como projeto (best-effort).
func describeProject(p string) *Project {
	name := filepath.Base(p)
	return &Project{
		Name:        name,
		Root:        p,
		Initialized: hasCoscaDir(p),
		HasCosca:    hasCoscaDir(p),
	}
}

// ---------------------------------------------------------------------------
// Filesystem do projeto (usado no Workspace — exige projeto ativo)
// ---------------------------------------------------------------------------

// TreeEntry é um item da árvore do projeto.
type TreeEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Kind     string      `json:"kind"`
	Children []TreeEntry `json:"children,omitempty"`
}

// FileNode é o modelo semântico do File Explorer profissional (fonte de verdade
// do filesystem). Diferente do TreeEntry (recursivo, depth máx. 5, sem metadata),
// o FileNode representa UM nó do sistema de arquivos do projeto ativo com
// metadata REAL (isDir, isSymlink, size, modtime, linguagem) — preenchida por
// os.ReadDir/os.Stat no disco, nunca inferida do nome nem mascarada.
//
//   - Children é POPULADO APENAS no carregamento LAZY, por pasta (ReadDir retorna
//     apenas os filhos diretos de um diretório). Para um nó folha (não carregado)
//     Children é nil e Loaded é false — o frontend expande sob demanda.
//   - IsDir é a fonte de verdade: vem de e.IsDir()/os.Stat, nunca do nome.
//   - Language é resolvida por resolveLanguage (extensão composta + nome
//     especial). É um rótulo SEMÂNTICO para a UI; não é syntax-highlighting.
type FileNode struct {
	Name       string     `json:"name"`               // nome do diretório/arquivo
	Path       string     `json:"path"`               // projeto-relativo, slash, canônico
	Kind       string     `json:"kind"`               // "file" | "directory" | "symlink" | "unknown"
	IsDir      bool       `json:"is_dir"`             // fonte de verdade (não inferir do nome)
	IsSymlink  bool       `json:"is_symlink"`         // entrada é um symlink (os.ModeSymlink)
	Ext        string     `json:"ext"`                // extensão simples (ex: ".ts") OU "" se nenhuma
	Language   string     `json:"language"`           // "typescript" | "go" | ... | "plaintext"
	Size       int64      `json:"size"`               // bytes do arquivo (dirs: size do diretório)
	ModifiedAt string     `json:"modified_at"`        // RFC3339 (UTC)
	Children   []FileNode `json:"children,omitempty"` // preenchido apenas no lazy por pasta
	Loaded     bool       `json:"loaded"`             // se children foi carregado (para lazy)
}

var ignoreDirs = map[string]bool{
	"node_modules": true, "dist": true, ".git": true, ".next": true,
	".venv": true, "build": true, "target": true, "out": true, "bin": true,
}

// ─── Bloqueio de caminhos sensíveis (fail-closed) ────────────────────────────
//
// O Desktop NUNCA lê, lista ou expõe para a UI os caminhos abaixo (ADR-0002
// "nunca na UI"). Esta lista espelha a lista oficial do Rails do Cosca
// (internal/chat/sandbox/rails.go DefaultBlockedDirs: `.git`, `.cosca/data`,
// `node_modules`) e a estende com os segredos do projeto que vazariam
// credenciais para a WebView se a árvore ou o leitor de arquivos permitisse
// (ex.: `.cosca/serve.env`, `.cosca/keys/`, `.env`).
//
// Regra de casamento: por PREFIXO DE SEGMENTO, sobre o path RELATIVO à raiz do
// projeto normalizado para slash. Um caminho é bloqueado se for igual a um item
// ou estiver abaixo dele. A comparação é case-insensitive (NTFS é
// case-preserving mas case-insensitive), o que fecha a variante ".COSCA/DATA"
// que burlaria uma comparação lexical exata.
var blockedPathSuffixes = []string{
	".git",                    // metadados do repositório (oficial)
	".cosca/data",             // chaves/segredos encriptados (oficial)
	"node_modules",            // deps de terceiros (oficial)
	".cosca/serve.env",        // CRÍTICO: segredos do servidor (JWT, API keys)
	".cosca/audit.db",         // registros de auditoria
	".cosca/gate.db",          // motor de transições (decisões)
	".cosca/trace.db",         // ledger de trace
	".cosca/keys",             // chaves do projeto (Ed25519/AES)
	".cosca/jail-secrets.env", // segredos injetados na jaula do runtime
	".env",                    // segredos ad-hoc do projeto
}

// normalizeRel normaliza um path (relativo a uma raiz) para comparação de
// prefixo: troca "\" por "/", resolve ".." e "." lexicalmente (filepath.Clean),
// remove "./" inicial e converte para minúsculas (NTFS é case-insensitive no
// Windows). O Clean torna o guard imune a variantes "foo/../.cosca/serve.env"
// que burlariam um check lexical sobre o rel cru (defesa em profundidade; o
// safeJoin já limpa o caminho antes de delegar).
func normalizeRel(p string) string {
	// Clean resolve ".."/"." no formato nativo do SO (Windows: barra invertida)
	// e então ToSlash normaliza para "/" — garante comparação consistente de
	// prefixo independente da forma com que o path chegou (Rel, FromSlash, ...).
	p = filepath.Clean(p)
	p = filepath.ToSlash(p)
	if p == "." {
		return ""
	}
	p = strings.TrimPrefix(p, "./")
	return strings.ToLower(p)
}

// isSensitivePath reporta se um path relativo à raiz do projeto resolve para
// um diretório/arquivo sensível que o Desktop deve bloquear. É o guard
// fail-closed usado por safeJoin (ReadFile/WriteFile/CreateFile) e por
// tree/TreeDirs (listing): qualquer caminho que case com um segmento da lista
// de bloqueio é rejeitado, nunca apenas ocultado.
func isSensitivePath(rel string) bool {
	rel = normalizeRel(rel)
	for _, blocked := range blockedPathSuffixes {
		b := normalizeRel(blocked)
		if rel == b || strings.HasPrefix(rel, b+"/") {
			return true
		}
	}
	return false
}

// TreeDirs lista a árvore do projeto ativo.
func (a *App) TreeDirs() []TreeEntry {
	if a.project == nil {
		return nil
	}
	return a.tree(a.project.Root, 0)
}
func (a *App) tree(dir string, depth int) []TreeEntry {
	if depth > 5 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []TreeEntry{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && (ignoreDirs[name] || (strings.HasPrefix(name, ".") && name != ".cosca")) {
			continue
		}
		rel, _ := filepath.Rel(a.project.Root, filepath.Join(dir, name))
		rel = filepath.ToSlash(rel)
		// Bloqueio de caminhos sensíveis (fail-closed): nunca listar nem entrar
		// em diretórios/arquivos de segredo (.cosca/data, .cosca/serve.env,
		// .cosca/keys, .env, .git/*). Essa informação não pertence à árvore do
		// Workspace (ADR-0002) e vazaria credenciais para a WebView se exibida.
		if isSensitivePath(rel) {
			continue
		}
		if e.IsDir() {
			out = append(out, TreeEntry{Name: name, Path: rel, Kind: "dir", Children: a.tree(filepath.Join(dir, name), depth+1)})
		} else {
			out = append(out, TreeEntry{Name: name, Path: rel, Kind: "file"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == "dir"
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ─── File Explorer LAZY + metadata real (fonte de verdade do filesystem) ──────
//
// O TreeDirs()/tree() acima continua existindo por COMPATIBILIDADE, mas o File
// Explorer profissional do Desktop migra para o carregamento LAZY via ReadDir():
// um diretório por chamada, não-recursivo, com metadata REAL do disco (IsDir,
// IsSymlink, Size, ModifiedAt, Language). O frontend expande sob demanda.

// ReadDir lista os FILHOS DIRETOS de um diretório do projeto ativo (relativo à
// raiz, vazio/"" = raiz). É LAZY e NÃO-recursivo: retorna apenas os nós do
// diretório pedido, com Children=nil e Loaded=false — cada pasta é carregada
// quando o usuário a expande na árvore. Fonte de verdade = os.ReadDir + os.Stat:
//
//   - Sem projeto ativo            → erro "sem projeto ativo" (fail-closed).
//   - rel vazio/""                 → raiz do projeto.
//   - rel não existe / não é dir   → erro honesto "não é diretório: <rel>".
//   - Caminho sensível (.cosca/keys, .env, .git/*) → bloqueado via safeJoin +
//     isSensitivePath (fail-closed, ADR-0002) — nunca listado para a WebView.
//
// Cada filho carrega: Kind ("file"|"directory"|"symlink"), IsDir (fonte de
// verdade via e.IsDir()), IsSymlink (e.Type()&os.ModeSymlink), Size e ModifiedAt
// (os.Stat), Ext (filepath.Ext, lowercase) e Language (resolveLanguage).
func (a *App) ReadDir(rel string) ([]FileNode, error) {
	if a.project == nil || a.project.Root == "" {
		return nil, fmt.Errorf("sem projeto ativo")
	}
	root := a.project.Root

	// Resolve o diretório-alvo relativo à raiz do projeto. safeJoin aplica o
	// mesmo guard do ReadFile/WriteFile (rejeita escapes por "..", byte nulo,
	// symlink para fora e caminhos sensíveis) — o explorer nunca sai da raiz.
	targetDir := root
	if rel != "" {
		p, err := a.safeJoin(rel)
		if err != nil {
			return nil, err
		}
		targetDir = p
	}

	st, err := os.Stat(targetDir)
	if err != nil {
		return nil, fmt.Errorf("não é diretório: %s", rel)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("não é diretório: %s", rel)
	}

	dirEntries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, fmt.Errorf("ler diretório %s: %w", rel, err)
	}

	nodes := make([]FileNode, 0, len(dirEntries))
	for _, e := range dirEntries {
		name := e.Name()
		full := filepath.Join(targetDir, name)

		// Path projeto-relativo, normalizado para slash (canônico).
		childRel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		childRel = filepath.ToSlash(childRel)

		// Fail-closed (ADR-0002): nunca listar nem entrar em caminhos sensíveis
		// (.cosca/serve.env, .env, .git/config, .cosca/keys ...). Eles não
		// pertencem ao explorer e vazariam credenciais para a WebView.
		if isSensitivePath(childRel) {
			continue
		}
		// ignoreDirs: pula diretórios de dependência/artefatos conhecidos
		// (node_modules, dist, .git, build, etc.) — não desce neles.
		if e.IsDir() && ignoreDirs[name] {
			continue
		}

		// Metadata real, direto do disco (source of truth).
		isSymlink := e.Type()&os.ModeSymlink != 0
		isDir := e.IsDir()

		// os.Stat segue symlink → size/modtime do ALVO (metadata real). Se
		// falhar (ex.: symlink quebrado), cai para e.Info() (lstat da entrada).
		fi, err := os.Stat(full)
		if err != nil {
			if eif, eerr := e.Info(); eerr == nil {
				fi = eif
			}
		}
		if fi == nil {
			continue // não é possível obter metadata — omite honestamente
		}

		kind := "file"
		if isSymlink {
			kind = "symlink"
		} else if isDir {
			kind = "directory"
		}

		nodes = append(nodes, FileNode{
			Name:       name,
			Path:       childRel,
			Kind:       kind,
			IsDir:      isDir,
			IsSymlink:  isSymlink,
			Ext:        strings.ToLower(filepath.Ext(name)),
			Language:   resolveLanguage(childRel),
			Size:       fi.Size(),
			ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			Children:   nil, // LAZY: não recursa — o frontend carrega sob demanda
			Loaded:     false,
		})
	}

	// Ordena: diretórios primeiro (IsDir é a fonte de verdade), depois alfabético.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// resolveLanguage resolve a linguagem de um caminho (relativo, slash) para um
// rótulo SEMÂNTICO do File Explorer. É DETERMINÍSTICO e não-chuta: cobre nomes
// especiais (Dockerfile, Makefile, .gitignore, .env) e extensões compostas
// (.d.ts, .test.ts, .spec.ts, .module.css) antes da extensão simples. Qualquer
// extensão desconhecida (ou ausente) → "plaintext" (nunca adivinhar).
func resolveLanguage(rel string) string {
	base := filepath.Base(filepath.ToSlash(rel))
	lowerBase := strings.ToLower(base)

	// Nomes especiais (case-insensitive), checados antes da extensão.
	switch lowerBase {
	case ".gitignore":
		return "gitignore"
	case ".env":
		return "env"
	case "dockerfile":
		return "dockerfile"
	case "makefile":
		return "makefile"
	}

	// Extensões compostas: mapeiam ANTES da extensão simples para que
	// "file.d.ts" → typescript (e não "ts"), "file.test.ts"/"file.spec.ts" →
	// typescript e "file.module.css" → css (mais preciso que o extname sozinho).
	if strings.HasSuffix(lowerBase, ".d.ts") || strings.HasSuffix(lowerBase, ".test.ts") || strings.HasSuffix(lowerBase, ".spec.ts") {
		return "typescript"
	}
	if strings.HasSuffix(lowerBase, ".module.css") {
		return "css"
	}

	// Extensão simples (lowercase).
	switch strings.ToLower(filepath.Ext(base)) {
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".go":
		return "go"
	case ".rs":
		return "rust"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".less":
		return "css"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	case ".ps1":
		return "powershell"
	case ".sh":
		return "shell"
	case ".xml":
		return "xml"
	case ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	case ".mp4", ".mkv", ".webm":
		return "video"
	case ".mp3", ".wav":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".zip", ".tar", ".gz":
		return "archive"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".dart":
		return "dart"
	}
	return "plaintext"
}

// ReadFile / WriteFile / CreateFile (exigem projeto ativo, com check de path).
func (a *App) ReadFile(rel string) (string, error) {
	p, err := a.safeJoin(rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", rel, err)
	}
	return string(b), nil
}
func (a *App) WriteFile(rel string, content string) error {
	p, err := a.safeJoin(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}
func (a *App) CreateFile(rel string, content string) error {
	p, err := a.safeJoin(rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("já existe: %s", rel)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// safeJoin resolve um caminho relativo dentro do projeto ativo rejeitando
// escapes por qualquer via, incluindo symlinks/junctions que aponte para fora
// da raiz. Mesma estratégia do mecanismo oficial do Cosca (internal/chat/sandbox):
//
//  1. rejeita byte nulo (fail-closed);
//  2. filepath.Clean + resolve absoluto (defesa lexical contra "..");
//  3. resolve symlinks dos ancestrais EXISTENTES no disco (EvalSymlinks sobre o
//     prefixo que existe) para detectar escape via symlink dentro do workspace;
//  4. verifica que o caminho resolvido permanece dentro da raiz do projeto
//     (prefixo exato, considerando o separador);
//  5. para caminhos que ainda NÃO existem (escrita de arquivo novo), resolve o
//     ancestral existente e valida o restante reapendado ao alvo resolvido.
func (a *App) safeJoin(rel string) (string, error) {
	if a.project == nil {
		return "", fmt.Errorf("sem projeto ativo")
	}
	// 1. Byte nulo é sempre rejeitado (injeção de caminho).
	if strings.ContainsRune(rel, '\x00') {
		return "", fmt.Errorf("path contém byte nulo — acesso negado")
	}
	rootAbs, err := filepath.Abs(a.project.Root)
	if err != nil {
		return "", fmt.Errorf("resolve raiz: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	// 2. Clean + absoluto (defesa lexical).
	fullAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(rel))))
	if err != nil {
		return "", fmt.Errorf("resolve caminho: %w", err)
	}
	fullAbs = filepath.Clean(fullAbs)
	if !isWithinRoot(fullAbs, rootAbs) {
		return "", fmt.Errorf("path fora do projeto: %s", rel)
	}
	// Bloqueio de caminhos sensíveis (fail-closed) sobre o path RESOLVIDO
	// lexicalmente e relativo à raiz: fecha "foo/../.cosca/serve.env" e demais
	// variantes que burlariam um check sobre o rel cru. Nunca ler/escrever/
	// alterar .git, .cosca/data, .cosca/serve.env, .cosca/keys, .env etc. —
	// dados que violariam o ADR-0002 se fossem para a WebView.
	if relToRoot, err := filepath.Rel(rootAbs, fullAbs); err == nil && isSensitivePath(relToRoot) {
		return "", fmt.Errorf("path sensível bloqueado: %s", rel)
	}
	// A raiz do projeto é o limite; se a própria raiz for um symlink, o limite
	// passa a ser o alvo resolvido (mantém o projeto funcional sem abrir escape).
	rootReal := rootAbs
	if realRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootReal = realRoot
	}
	// 3. Resolve symlinks do ancestral existente mais próximo.
	existing, err := resolveExistingAncestor(fullAbs)
	if err != nil {
		return "", fmt.Errorf("path resolution: %w", err)
	}
	real, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("symlink resolution falhou — acesso negado: %w", err)
	}
	// 4. O alvo resolvido precisa permanecer dentro da raiz do projeto.
	if !isWithinRoot(real, rootReal) {
		return "", fmt.Errorf("path escapa do projeto via symlink: %s", rel)
	}
	// 5. Para caminhos ainda inexistentes, reapenda o restante não resolvido
	// ao ancestral resolvido e revalida o resultado completo.
	relTail, err := filepath.Rel(existing, fullAbs)
	if err != nil {
		return "", fmt.Errorf("resolve restante: %w", err)
	}
	resolved := filepath.Clean(filepath.Join(real, relTail))
	if !isWithinRoot(resolved, rootReal) {
		return "", fmt.Errorf("path escapa do projeto: %s", rel)
	}
	// Bloqueio de caminhos sensíveis sobre o path RESOLVIDO FINAL (após
	// EvalSymlinks): um symlink dentro do workspace apontando para `.cosca/data`
	// (chaves/segredos) passa no check "dentro da raiz" mas ainda vaza o alvo
	// se for lido. O rel é calculado contra a raiz real resolvida.
	if relToRoot, err := filepath.Rel(rootReal, resolved); err == nil && isSensitivePath(relToRoot) {
		return "", fmt.Errorf("path sensível bloqueado: %s", rel)
	}
	return resolved, nil
}

// resolveExistingAncestor devolve o prefixo de `path` que existe no disco (o
// ancestral mais próximo de `path`). Se o próprio path existe, devolve ele;
// senão sobe até achar um componente existente. Usa Lstat para que um symlink
// existente (mesmo quebrado) seja reconhecido — o EvalSymlinks subsequente
// decide se é legítimo.
func resolveExistingAncestor(path string) (string, error) {
	abs := filepath.Clean(path)
	if !filepath.IsAbs(abs) {
		a, err := filepath.Abs(abs)
		if err != nil {
			return "", fmt.Errorf("resolve absoluto: %w", err)
		}
		abs = a
	}
	cur := abs
	for {
		if _, err := os.Lstat(cur); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor for %q", path)
		}
		cur = parent
	}
}

// isWithinRoot verifica se `p` está dentro de `root` (igual ao root, ou abaixo
// dele com separador exato). Ambos são normalizados antes da comparação.
func isWithinRoot(p, root string) bool {
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	return p == root || strings.HasPrefix(p, root+string(filepath.Separator))
}

// ---------------------------------------------------------------------------
// Agent runtime (REQUIRED LATER — fora do primeiro vertical slice)
// Mantido, porém não é usado pelo fluxo Create Project.
// ---------------------------------------------------------------------------

// ChatMessage é uma mensagem da conversa com o runtime/agente.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	At      string `json:"at"`
}

var chatBuf []ChatMessage

// Chat executa o runtime cosca no projeto (REQUIRED LATER).
//
// POLÍTICA (ADR-0003): `Chat` aciona `cosca exec` diretamente — o caminho mais
// sensível. A Desktop delega a autoridade ao runtime (execpolicy + approval);
// envAllowNoRoot apenas declara o Windows sem bwrap e NÃO autoriza. A decisão
// de executar permanece no Cosca, nunca no Desktop.
func (a *App) Chat(message string) ChatMessage {
	if a.project == nil {
		return ChatMessage{Role: "assistant", Content: "Sem projeto ativo — crie/abra um projeto primeiro.", At: time.Now().Format("15:04:05")}
	}
	// GATE (opção B): `Chat` aciona `cosca exec` — escrita/execução. Sem uma
	// aprovação "approved" registrada NÃO dispara o runtime (fail-closed).
	req := strings.TrimSpace(message)
	if g := a.requireExecApproval(req); g != "" {
		return ChatMessage{Role: "assistant", Content: g, At: time.Now().Format("15:04:05")}
	}
	defer a.clearApproval() // o consentimento local é consumido ao executar
	out := a.runRuntime(req, "exec")
	resp := strings.TrimSpace(out)
	if resp == "" {
		resp = "(sem resposta — configure provider/modelo)"
	}
	msg := ChatMessage{Role: "assistant", Content: resp, At: time.Now().Format("15:04:05")}
	chatBuf = append(chatBuf, ChatMessage{Role: "user", Content: req, At: ""}, msg)
	return msg
}
func (a *App) ChatHistory() []ChatMessage { return chatBuf }

// ---------------------------------------------------------------------------
// CHAT (FASE 3 — Kernel-First /v1/run/stream) — a ponte Conversa ↔ Cérebro
// ---------------------------------------------------------------------------
//
// A UI NÃO tem lógica cognitiva: ela fala com o Desktop, que CONSUME o daemon
// `/v1/run` (Kernel-First, ADR-032) e REAPRESENTA o contrato canônico de
// eventos: `thinking → status:processing → progress* → response* →
// metadata → status:done → done` (| `error` | `cancelled`). Este bloco NÃO
// reimplementa o runtime nem duplica sessão — só cria a ponte HTTP/SSE→front
// com:
//   - `session_id` no request/resposta (resume/fork via runRequest);
//   - streaming token-a-token via `cosca:chat:event` (EventsEmit);
//   - cancelamento real (context cancel → SSE abort → estado "cancelled").
//
// POLÍTICA: o Chat é uma CONVERSA (raciocínio/responder), NÃO toca o
// filesystem por conta própria — logo NÃO exigimos o gate local de aprovação
// de escrita/execução (o runtime aplica a sua própria execpolicy se/houver
// tool call). A autoridade permanece no Cosca.

// chatRunStreamURL é o endpoint do daemon consumido (Kernel-First stream).
const chatRunStreamURL = "/v1/run/stream"

// ChatStreamResult é a resposta consolidada de um turno de conversa.
type ChatStreamResult struct {
	Text       string `json:"text"`       // texto final do agente (concatenado)
	SessionID  string `json:"session_id"` // id da conversa (resume/fork)
	Model      string `json:"model"`      // modelo que respondeu
	Provider   string `json:"provider"`   // provider do runtime
	Agent      string `json:"agent"`      // agente rolado pelo runtime
	DurationMs int64  `json:"duration_ms"`
	Ended      string `json:"ended"`               // "done" | "error" | "cancelled"
	Error      string `json:"error,omitempty"`     // mensagem honesta de erro (quando aplicável)
}

// ChatSessionSummary é um resumo leve de uma conversa persistida (sidebar).
type ChatSessionSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`      // derivado da 1ª msg do usuário (honesto)
	Model     string `json:"model"`
	Messages  int    `json:"messages"`
	UpdatedAt string `json:"updated_at"` // RFC3339 (meta.updated_at)
}

// chatFrame é o envelope SSE canônico do daemon (extraído de `data: {...}`).
// Como NÃO reimplementamos o protocolo do runtime, só capturamos os campos que
// o wire realmente entrega (JSON lax — ausentes ficam zero).
type chatFrame struct {
	Type       string         `json:"type"`
	Content    string         `json:"content"`
	Status     string         `json:"status"`
	Error      string         `json:"error"`
	SessionID  string         `json:"session_id"`
	Model      string         `json:"model"`
	Provider   string         `json:"provider"`
	Agent      string         `json:"agent"`
	DurationMs int64          `json:"duration_ms"`
	TokenUsage map[string]int `json:"token_usage"`
	Metadata   map[string]any `json:"metadata"`
}

// ChatStream executa UM turno de conversa pelo daemon Kernel-First
// (`/v1/run/stream`) com sessão (resume/fork) e streaming token-a-token para a
// UI via `cosca:chat:event`. Retorna o texto final + metadata.
//
//   - `runID` identifica o turno para o cancelamento (CancelChatStream). O
//     frontend gera um runID por envio; para cancelar chama CancelChatStream.
//   - `sessionID` "" → conversa NOVA (o daemon gera e devolve); passe o id do
//     turno anterior para RESUMIR (o daemon injeta o histórico persistido).
//   - `model` é o provider/modelo; "" → default "deepseek".
//
// Streaming (cosca:chat:event) — o MESMO shape do daemon:
//
//	{"type":"thinking","content":"...","session_id":"..."}
//	{"type":"status","status":"processing"}
//	{"type":"progress","content":"...","metadata":{...}}
//	{"type":"response","content":"<chunk>"}
//	{"type":"metadata","session_id":"...","model":"...","provider":"...","agent":"...","duration_ms":N,"token_usage":{...}}
//	{"type":"done","duration_ms":N,"session_id":"..."}
//	{"type":"error","content":"..."}
//	{"type":"cancelled"}
func (a *App) ChatStream(runID, message, sessionID, model string) *ChatStreamResult {
	res := &ChatStreamResult{Ended: "error"}
	msg := strings.TrimSpace(message)
	if msg == "" {
		res.Text = "mensagem vazia — nada a conversar"
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}
	if strings.TrimSpace(model) == "" {
		model = "deepseek"
	}
	res.Model = model

	// Daemon offline ⇒ fallback honesto (não fabricar thinking/response/done).
	if ok, why := a.serveHealth(); !ok {
		res.Text = "daemon offline — a conversa requer cosca serve (SSE); inicie-o e tente novamente. (" + why + ")"
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}

	// Contexto cancelável por runID — o frontend cancela via CancelChatStream.
	ctx, cancel := context.WithCancel(a.ctx)
	a.storeChatCancel(runID, cancel)
	defer a.deleteChatCancel(runID)

	payload, err := json.Marshal(map[string]any{
		"prompt":     msg,
		"provider":   model,
		"session_id": strings.TrimSpace(sessionID),
	})
	if err != nil {
		res.Text = "falha ao montar request: " + err.Error()
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.serveBaseURL()+chatRunStreamURL, bytes.NewReader(payload))
	if err != nil {
		res.Text = "falha ao criar request: " + err.Error()
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// Cancelamento explícito OU disconnect do cliente.
		if ctx.Err() != nil {
			res.Ended = "cancelled"
			res.Text = ""
			a.emitChatFrame(chatFrame{Type: "cancelled"})
			return res
		}
		res.Text = "daemon request falhou: " + err.Error()
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		res.Text = fmt.Sprintf("daemon respondeu %s: %s", resp.Status, strings.TrimSpace(string(b)))
		res.Error = res.Text
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Text})
		return res
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			res.Ended = "cancelled"
			a.emitChatFrame(chatFrame{Type: "cancelled"})
			return res
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
		if data == "" {
			continue
		}
		var fr chatFrame
		if err := json.Unmarshal([]byte(data), &fr); err != nil {
			continue // frame malformado — não inventamos conteúdo
		}

		// Re-apresenta o frame na íntegra para a UI (shape canônico do daemon).
		a.emitChatFrame(fr)

		switch fr.Type {
		case "response":
			if fr.Content != "" {
				sb.WriteString(fr.Content)
			}
		case "thinking":
			if fr.SessionID != "" { res.SessionID = fr.SessionID }
			if fr.Model != "" { res.Model = fr.Model }
			if fr.Provider != "" { res.Provider = fr.Provider }
			if fr.Agent != "" { res.Agent = fr.Agent }
		case "metadata":
			if fr.SessionID != "" { res.SessionID = fr.SessionID }
			if fr.Model != "" { res.Model = fr.Model }
			if fr.Provider != "" { res.Provider = fr.Provider }
			if fr.Agent != "" { res.Agent = fr.Agent }
			if fr.DurationMs > 0 { res.DurationMs = fr.DurationMs }
		case "done":
			res.Ended = "done"
			if fr.SessionID != "" { res.SessionID = fr.SessionID }
			if fr.DurationMs > 0 { res.DurationMs = fr.DurationMs }
		case "error":
			res.Ended = "error"
			if fr.Content != "" {
				res.Error = fr.Content
			} else {
				res.Error = "erro do runtime"
			}
		case "cancelled":
			res.Ended = "cancelled"
			return res
		}
		if res.Ended == "done" || res.Ended == "error" || res.Ended == "cancelled" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			res.Ended = "cancelled"
			a.emitChatFrame(chatFrame{Type: "cancelled"})
		} else {
			res.Ended = "error"
			res.Error = "sse read error: " + err.Error()
			a.emitChatFrame(chatFrame{Type: "error", Content: res.Error})
		}
		return res
	}
	// SSE encerrou sem terminal explícito → se houve resposta, é um "done"
	// honesto; caso contrário reportamos erro (nunca fabricamos sucesso).
	if res.Ended == "error" && res.Error == "" {
		res.Error = "stream encerrou sem estado terminal"
		a.emitChatFrame(chatFrame{Type: "error", Content: res.Error})
		return res
	}
	if res.Ended != "cancelled" && res.Ended != "error" {
		res.Ended = "done"
	}
	res.Text = strings.TrimSpace(sb.String())
	return res
}

// emitChatFrame emite o frame canônico do daemon para a UI via Wails
// (`cosca:chat:event`). É o canal Desktop→UI do chat — a UI assina por
// EventsOn e interpreta o `type` (thinking/response/progress/status/metadata/
// done/error/cancelled). Nenhuma lógica cognitiva aqui: só reapresentação.
func (a *App) emitChatFrame(fr chatFrame) {
	a.emitEvent("cosca:chat:event", fr)
}

func (a *App) storeChatCancel(runID string, cancel context.CancelFunc) {
	if runID == "" {
		return
	}
	a.chatCancelMu.Lock()
	defer a.chatCancelMu.Unlock()
	if a.chatCancel == nil {
		a.chatCancel = make(map[string]context.CancelFunc)
	}
	a.chatCancel[runID] = cancel
}

func (a *App) deleteChatCancel(runID string) {
	if runID == "" {
		return
	}
	a.chatCancelMu.Lock()
	defer a.chatCancelMu.Unlock()
	delete(a.chatCancel, runID)
}

// CancelChatStream cancela um stream de chat EM ANDAMENTO (runID). O cancel
// propaga ao context do request HTTP → o SSE aborta → o ChatStream emite
// {"type":"cancelled"} e termina com Ended="cancelled". Retorna true se havia
// um stream ativo para o runID (false = no-op honesto).
func (a *App) CancelChatStream(runID string) bool {
	if runID == "" {
		return false
	}
	a.chatCancelMu.Lock()
	cancel, ok := a.chatCancel[runID]
	a.chatCancelMu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

// ---------------------------------------------------------------------------
// Histórico de conversas (sidebar) — lê `.cosca/sessions/*.jsonl` do projeto
// ativo (o MESMO writer do runtime/CLI via engine.SessionManager, ativado por
// SetSessionsDir no serve). A UI usa isto como lista real de conversas.
// Sem projeto ativo → [] (honesto).
// ---------------------------------------------------------------------------

// chatSessionsDir resolve o diretório de sessões persistidas do projeto ativo.
// Retorna "" sem projeto ativo (a conversa local continua funcional, apenas
// sem histórico persistido em disco para a sidebar).
func (a *App) chatSessionsDir() string {
	if a.project == nil || a.project.Root == "" {
		return ""
	}
	return filepath.Join(a.project.Root, ".cosca", "sessions")
}

// ChatListSessions lista as conversas persistidas pelo runtime no projeto ativo
// (`<projeto>/.cosca/sessions/*.jsonl`), ordenadas pela mais recente. A sidebar
// usa isto como histórico. Nada é inventado — só o que existe em disco.
func (a *App) ChatListSessions() []ChatSessionSummary {
	dir := a.chatSessionsDir()
	if dir == "" {
		return []ChatSessionSummary{}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []ChatSessionSummary{}
	}
	out := make([]ChatSessionSummary, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		msgs, model, updated, ok := a.readChatSession(id)
		if !ok {
			continue
		}
		out = append(out, ChatSessionSummary{
			ID:        id,
			Title:     chatTitleFromMessages(msgs),
			Model:     model,
			Messages:  len(msgs),
			UpdatedAt: updated,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

// ChatSessionMessages devolve as mensagens de uma conversa persistida (para a
// UI exibir o histórico ao abrir uma sessão na sidebar). Sem projeto/id vazio
// → [].
func (a *App) ChatSessionMessages(id string) []ChatMessage {
	if id == "" {
		return []ChatMessage{}
	}
	msgs, _, _, ok := a.readChatSession(id)
	if !ok {
		return []ChatMessage{}
	}
	return msgs
}

// readChatSession lê um `.cosca/sessions/<id>.jsonl` e devolve a foto honesta
// da conversa: mensagens (role/content), o modelo e o updated_at do meta. Não
// inventa nada — só o que o runtime persistiu (JSONL: linha 1 type:"meta",
// depois linhas type:"message" com role/content).
func (a *App) readChatSession(id string) (msgs []ChatMessage, model, updatedAt string, ok bool) {
	dir := a.chatSessionsDir()
	if dir == "" {
		return nil, "", "", false
	}
	path := filepath.Join(dir, id+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, "", "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec struct {
			Type    string    `json:"type"`
			Role    string    `json:"role"`
			Content string    `json:"content"`
			Model   string    `json:"model"`
			Updated time.Time `json:"updated_at"`
			Stamp   time.Time `json:"timestamp"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		switch rec.Type {
		case "meta":
			model = rec.Model
			if !rec.Updated.IsZero() {
				updatedAt = rec.Updated.Format(time.RFC3339)
			}
		case "message":
			if rec.Role == "" {
				continue
			}
			at := ""
			if !rec.Stamp.IsZero() {
				at = rec.Stamp.Format("15:04:05")
			}
			msgs = append(msgs, ChatMessage{Role: rec.Role, Content: rec.Content, At: at})
		}
	}
	return msgs, model, updatedAt, true
}

// chatTitleFromMessages deriva um título curto e honesto de uma conversa a
// partir da primeira mensagem do usuário (nunca fabrica: sem msg de usuário
// devolve "Nova conversa").
func chatTitleFromMessages(msgs []ChatMessage) string {
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		t := strings.TrimSpace(m.Content)
		if t == "" {
			continue
		}
		if i := strings.IndexByte(t, '\n'); i >= 0 {
			t = t[:i]
		}
		r := []rune(t)
		if len(r) > 48 {
			t = string(r[:48]) + "…"
		}
		return t
	}
	return "Nova conversa"
}

// ProjectInfo retorna metadados (ProjectContext consolidado).
func (a *App) ProjectInfo() map[string]any {
	if a.project == nil {
		return map[string]any{"has_project": false, "runtime": a.runtime}
	}
	return map[string]any{"has_project": true, "project": a.project.Root, "name": a.project.Name, "runtime": a.runtime}
}

// AnalyzeProject roda o Project Intelligence (read-only) sobre o projeto ATIVO
// e devolve o ProjectProfile. É a camada que "entende o projeto" sem que o
// usuário configure nada: linguagens, frameworks, package managers, formatters,
// linters, test/build, docker, monorepo, git, ci, docs, comandos — tudo com
// evidência e confiança. NUNCA executa comandos (detecção é estática); no
// desktop standalone não carrega kernel/memória institucional (ADR-0007).
func (a *App) AnalyzeProject() map[string]any {
	if a.project == nil {
		return map[string]any{"has_project": false}
	}
	p, err := a.piCache.GetOrAnalyze(a.project.Root)
	if err != nil {
		return map[string]any{"has_project": true, "error": err.Error(), "root": a.project.Root}
	}
	return map[string]any{"has_project": true, "profile": p}
}

// CaptureUI recebe a cena visual da interface (bounds/roles/estados/tokens
// coletados pelo frontend via getBoundingClientRect + computed styles) e a
// analisa pela UI Perception Engine (sem GPU, local, determinística). É o
// "olho" do Kernel para a própria interface — CAMADA 1-2 do modelo de visão
// (estrutura + geometria). Devolve um Audit (violações com evidência + scene),
// governado por docs/UI_GOVERNANCE.md.
func (a *App) CaptureUI(sceneJSON string) map[string]any {
	var scene uiperception.SceneModel
	if err := json.Unmarshal([]byte(sceneJSON), &scene); err != nil {
		return map[string]any{"ok": false, "error": "scene inválida: " + err.Error()}
	}
	if scene.Root == nil && len(scene.Elements) == 0 {
		return map[string]any{"ok": false, "error": "scene vazia"}
	}
	audit := uiperception.Analyze(&scene)
	return map[string]any{"ok": true, "audit": audit}
}

// RefreshProject invalida o cache de Project Intelligence do projeto ativo e o
// reanalisa imediatamente. A UI chama no "Refresh" do painel Intelligence/agente
// (ex.: após editar um arquivo relevante). Em memória — nunca persiste nada.
func (a *App) RefreshProject() map[string]any {
	if a.project == nil {
		return map[string]any{"has_project": false}
	}
	a.piCache.Invalidate(a.project.Root)
	return a.AnalyzeProject()
}

// AgentProjectContext injeta o Project Intelligence no contexto do Agente — o
// resumo ESTRUTURADO, DETERMINÍSTICO e COMPACTO que elimina o agente ficar
// perguntando "que package manager?", "que framework?" etc. Consome o PI
// read-only (nunca duplica kernel/memória/skills do Cosca — ver ADR-0007).
//
// Contrato:
//   - Sem projeto ativo → {has_project:false}.
//   - Erro de análise    → {has_project:true, error, project, name}.
//   - Sucesso            → resumo com project/name/type/stack/package_manager/
//     build/test/lint/format/typecheck/language/framework/database/container/ci/
//     monorepo/git + skills[] (AgentSkillMatch) + pipeline[] (AgentPipeline) +
//     commands[] + important_dirs + important_config + detected_at + capabilities.
//
// NOTA (perf): NÃO calculamos o PI aqui dentro de `AgentRunStream`/`AgentPlan`.
// Este método é o ponto único de contexto para a UI/Agente; o fluxo de execução
// consome o que já foi derivado, sem re-scanneio desnecessário.
func (a *App) AgentProjectContext() map[string]any {
	if a.project == nil || a.project.Root == "" {
		return map[string]any{"has_project": false}
	}
	profile, err := a.piCache.GetOrAnalyze(a.project.Root)
	if err != nil {
		return map[string]any{
			"has_project": true,
			"error":       err.Error(),
			"project":     a.project.Root,
			"name":        a.project.Name,
		}
	}
	return a.buildProjectContext(profile)
}

// buildProjectContext deriva o resumo compacto do perfil para o Agente/UI.
// Mantém o contexto pequeno (não expõe o profile cru inteiro): o que sobra é o
// que a UI/Agente precisa para não perguntar nada — a stack, a ferramenta de
// cada categoria, o stack, skills casadas e o pipeline auto-descoberto.
func (a *App) buildProjectContext(p *projectintel.ProjectProfile) map[string]any {
	pm := firstTechName(p.PackageManagers)
	if pm == "" {
		pm = p.Capabilities["package_manager"]
	}
	return map[string]any{
		"has_project":      true,
		"project":          a.project.Root,
		"name":             a.project.Name,
		"type":             p.ProjectTypes,
		"stack":            projectintel.AgentStack(p),
		"package_manager":  pm,
		"build":            firstTechName(p.BuildTools),
		"test":             firstTechName(p.TestTools),
		"lint":             firstTechName(p.Linters),
		"format":           firstTechName(p.Formatters),
		"typecheck":        firstTechName(p.TypeCheckers),
		"language":         firstTechName(p.Languages),
		"framework":        firstTechName(p.Frameworks),
		"database":         firstTechName(p.Database),
		"container":        firstTechName(p.Container),
		"ci":               firstTechName(p.CI),
		"monorepo":         firstTechName(p.Monorepo),
		"git":              hasTechName(p.Repository, "Git"),
		"skills":           projectintel.AgentSkillMatch(p),
		"pipeline":         projectintel.AgentPipeline(p),
		"commands":         p.Commands,
		"important_dirs":   a.importantDirs(),
		"important_config": p.ConfigFiles,
		"detected_at":      p.DetectedAt,
		"capabilities":     p.Capabilities,
	}
}

// firstTechName devolve o nome da primeira tecnologia de uma categoria ("" se
// vazia) — a mesma leitura do helper interno `firstTech` do PI, exposta aqui
// porque `firstTech` é privado do package projectintel.
func firstTechName(list []projectintel.Tech) string {
	if len(list) == 0 {
		return ""
	}
	return list[0].Name
}

// hasTechName reporta se uma categoria contém a tecnologia `name`.
func hasTechName(list []projectintel.Tech, name string) bool {
	for _, t := range list {
		if t.Name == name {
			return true
		}
	}
	return false
}

// importantDirCandidates são os diretórios de convenção que o contexto marca
// como importantes para o Agente (do root do projeto, por nome real).
var importantDirCandidates = []string{"src", "app", "packages", "cmd", "internal", "docs", "tests", "infra"}

// importantDirs devolve os subdiretórios de convenção que EXISTEM no projeto
// ativo (leitura leve do root). Usado pelo contexto do Agente para apontar onde
// o código provavelmente vive — sem percorrer a árvore toda.
func (a *App) importantDirs() []string {
	dirs := []string{}
	if a.project == nil {
		return dirs
	}
	for _, d := range importantDirCandidates {
		if st, err := os.Stat(filepath.Join(a.project.Root, d)); err == nil && st.IsDir() {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// ─── Política de execução do runtime (COSCA) ─────────────────────────────────
//
// O Desktop invoca o binário cosca.exe como o PRÓPRIO agente da aplicação (não
// é um terminal ad-hoc do usuário). Estes são os ÚNICOS env-vars de política que
// ele injeta no runtime — e NENHUM deles é um "escape" que concede autorização.
//
// A AUTORIDADE de cada operação NÃO vem do Desktop: vem do runtime do Cosca,
// que aplica a defesa real documentada no ADR-0003 (approval + execpolicy +
// isolamento de projeto + Rails no filesystem). No Windows o runtime NÃO tem
// sandbox de processo (bwrap), então a contenção depende de execpolicy
// (Allow/Prompt/Forbidden) + `cosca approve`. O que a Desktop faz aqui é
// DECLARAR a realidade Windows, nunca contornar a autorização.
//
// ⚠ CRÍTICO para auditoria:
//
//	COSCA_ALLOW_NO_ROOT=1  declara que o host é Windows sem bwrap — e nada além
//	disso. Ele NÃO torna nada autorizado: a operação só ocorre porque o runtime
//	(a) aplicou a execpolicy sobre o comando e (b) passou pelo approval do
//	fluxo plan→approve→exec. Nunca trate esta variável como um gate de
//	permissão; ela apenas impede a negação fail-closed do runtime quando não há
//	jail. A Desktop NÃO deve permitir operação sensível que o runtime não
//	autorizaria — para isso a UI jamais contorna: ela pede, o backend delega ao
//	runtime (approx/execpolicy) e o runtime decide.
const (
	envProject     = "COSCA_PROJECT"       // raiz contextual = projeto ativo
	envAllowNoRoot = "COSCA_ALLOW_NO_ROOT" // declara Windows sem bwrap (opt-in, nunca autorização)
	envEnableExec  = "COSCA_ENABLE_EXEC"   // habilita exec/plan/approve (fail-closed por regra do Don)

	// Valores da política. A Desktop é o agente da aplicação e por isso opera o
	// binário dentro do fluxo de aprovação do Cosca.
	policyAllowNoRoot = "1"
	policyEnableExec  = "1"
)

// runtimeEnv monta o ambiente do runtime cosca no contexto do projeto ativo
// (project-first). Veja a POLÍTICA acima: este método NÃO autoriza nada — ele
// apenas preenche o ambiente que o runtime do Cosca espera. A autoridade é do
// runtime (execpolicy + approval), nunca do Desktop.
func (a *App) runtimeEnv() []string {
	env := []string{
		envProject + "=" + a.project.Root,
		envAllowNoRoot + "=" + policyAllowNoRoot,
		envEnableExec + "=" + policyEnableExec,
	}
	return append(os.Environ(), env...)
}

// runRuntime executa cosca.exe no diretório do projeto (REQUIRED LATER).
func (a *App) runRuntime(prompt string, args ...string) string {
	full := append([]string{}, args...)
	if prompt != "" {
		full = append(full, prompt)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.runtime, full...)
	cmd.Dir = a.project.Root
	cmd.Env = a.runtimeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)) + "\n[erro: " + err.Error() + "]"
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// Approval state-machine (opção B — DON): qualquer escrita/execução no agente
// exige uma aprovação EXPLÍCITA registrada no Desktop ANTES de disparar o
// runtime. READ-ONLY é direto.
//
// POLÍTICA (não é um segundo sistema de autorização): a AUTORIDADE final
// permanece no runtime do Cosca (execpolicy + approval). O Desktop apenas
// registra o consentimento local do usuário (UX / defesa em profundidade). A
// UI NUNCA contorna a decisão do runtime; e o Desktop NUNCA concede o que o
// runtime negaria. O gate abaixo é fail-closed: sem aprovação `approved`
// registrada, não roda `exec`.
// ---------------------------------------------------------------------------

// Approval é o registro de uma aprovação local de escrita/execução feita pelo
// usuário no Desktop. O campo `Risk` é uma descrição curta e descritiva da
// natureza da operação ("write", "exec", "read-only", "network").
// `Status` ∈ "pending" | "approved" | "denied".
type Approval struct {
	ID        string `json:"id"`
	Request   string `json:"request"`
	Model     string `json:"model"`
	Scope     string `json:"scope"`
	Risk      string `json:"risk"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// approvalRequiredMsg é a mensagem do gate quando uma operação de escrita/
// execução é solicitada sem uma aprovação `approved` registrada. Instrui a UI a
// chamar RequestApproval primeiro.
const approvalRequiredMsg = "Aprovação necessária: chame RequestApproval antes de executar (nenhuma aprovação aprovada registrada para este pedido)."

// RequestApproval cria um registro de aprovação pendente para `request`/`model`
// com a natureza `scope` (ex.: "exec", "write", "read-only"). Se já existir uma
// aprovação NÃO RESOLVIDA (status "pending") para o mesmo request, retorna a
// existente (não duplica). Guarda o registro em a.pendingApproval.
func (a *App) RequestApproval(request, model, scope string) *Approval {
	request = strings.TrimSpace(request)
	if a.pendingApproval != nil && a.pendingApproval.Status == "pending" && a.pendingApproval.Request == request {
		return a.pendingApproval
	}
	a.pendingApproval = &Approval{
		ID:        newApprovalID(),
		Request:   request,
		Model:     model,
		Scope:     scope,
		Risk:      riskFromScope(scope),
		Status:    "pending",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	return a.pendingApproval
}

// PendingApproval retorna a aprovação atual registrada (ou nil se não houver
// nenhuma). Não é preciso estar "pending"; o status é consultável via o campo.
func (a *App) PendingApproval() *Approval {
	return a.pendingApproval
}

// ApprovePending marca a aprovação pendente `id` como "approved". Se `id` não
// corresponde à pendente atual, retorna um Approval{Status:"denied"} (fail-
// closed: nunca aprova algo que não foi explicitamente solicitado). NÃO limpa
// o campo — a aprovação fica registrada até ser consumida por AgentRun/Chat
// (que a limpam ao rodar).
func (a *App) ApprovePending(id string) *Approval {
	if a.pendingApproval == nil || a.pendingApproval.ID != id {
		return &Approval{Status: "denied"}
	}
	a.pendingApproval.Status = "approved"
	return a.pendingApproval
}

// DenyPending marca a aprovação pendente `id` como "denied" e limpa o campo.
// Se `id` não corresponde à pendente atual, retorna Approval{Status:"denied"}.
func (a *App) DenyPending(id string) *Approval {
	if a.pendingApproval == nil || a.pendingApproval.ID != id {
		return &Approval{Status: "denied"}
	}
	ap := a.pendingApproval
	ap.Status = "denied"
	a.pendingApproval = nil
	return ap
}

// requireExecApproval é o GATE fail-closed das operações de escrita/execução.
// Retorna "" se `request` tem uma aprovação "approved" registrada (permitido),
// ou a mensagem a devolver à UI (bloqueado). A comparação é por request: uma
// aprovação aprovada para OUTRO pedido não libera este.
func (a *App) requireExecApproval(request string) string {
	if a.pendingApproval == nil || a.pendingApproval.Status != "approved" {
		return approvalRequiredMsg
	}
	if a.pendingApproval.Request != request {
		return approvalRequiredMsg
	}
	return ""
}

// clearApproval limpa o registro de aprovação (após o runtime consumir o
// consentimento local ao executar a operação).
func (a *App) clearApproval() {
	a.pendingApproval = nil
}

// riskFromScope deriva o Risk curto a partir do scope descritivo da operação.
func riskFromScope(scope string) string {
	s := strings.ToLower(strings.TrimSpace(scope))
	switch {
	case strings.Contains(s, "write"):
		return "write"
	case strings.Contains(s, "exec"):
		return "exec"
	case strings.Contains(s, "network"):
		return "network"
	case strings.Contains(s, "read"):
		return "read-only"
	case s != "":
		return s
	default:
		return "unknown"
	}
}

// newApprovalID gera um ID curto e único para o registro de aprovação.
func newApprovalID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ap-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("ap-%x", b)
}

// ---------------------------------------------------------------------------
// AGENT (primeiro grande slice) — usa comandos REAIS do Cosca. Nada é simulado.
// ---------------------------------------------------------------------------

// AgentPlan gera o Plano de Execução REAL via `cosca plan --target <alvo> --json`
// no projeto ativo.
//
// O `cosca plan` é SOMENTE LEITURA e a assinatura real é:
//
//	cosca plan --target <glob|lista-de-arquivos> [--type ...] [--agent ...] [--json]
//
// Ele aceita `cobra.NoArgs` — o alvo NÃO é argumento posicional. O pedido em
// linguagem natural da Desktop precisaria ser traduzido em arquivos-alvo; como
// esse mapeamento é do domínio do agente (não do estimador), quando o request
// não é um alvo plausível de arquivos (glob, caminho existente ou novo arquivo
// com extensão) devolvemos uma resposta HONESTA apontando o que o `plan`
// exige, em vez de fabricar um plano para um pseudo-arquivo. Sempre que o
// request casar com um alvo válido, o plano real do Cosca é retornado cru
// (JSON, compatível com `cosca approve --plan`).
func (a *App) AgentPlan(request string) string {
	if a.project == nil {
		return "Sem projeto ativo."
	}
	request = strings.TrimSpace(request)
	if request == "" {
		return "Pedido vazio. O `cosca plan` estima um plano a partir de um alvo de arquivos (--target), ex.: \"internal/kernel/*.go\" ou \"api/rest/users.go\"."
	}
	if !a.looksLikePlanTarget(request) {
		return "O `cosca plan` estima o plano a partir de ARQUIVOS-ALVO (--target), não de um pedido em linguagem natural.\n" +
			"Informe um glob ou caminho de arquivos, ex.: \"internal/kernel/*.go\" ou \"api/rest/users.go\".\n" +
			"Para executar a tarefa em linguagem natural, use o botão Executar (cosca exec)."
	}
	full := []string{"plan", "--target", request, "--json"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.runtime, full...)
	cmd.Dir = a.project.Root
	cmd.Env = a.runtimeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)) + "\n[erro: " + err.Error() + "]"
	}
	return strings.TrimSpace(string(out))
}

// looksLikePlanTarget avalia se uma string pode ser usada como `--target` do
// `cosca plan`: um glob ("internal/kernel/*.go"), um caminho absoluto, um
// caminho/arquivo que existe em disco (na raiz do projeto ou do processo), ou
// um caminho com extensão (novo arquivo a ser criado). Um pedido em linguagem
// natural não casa e retorna false — evita que o estimador fabrique um plano
// para um pseudo-arquivo.
func (a *App) looksLikePlanTarget(s string) bool {
	if strings.ContainsAny(s, "*?[") {
		return true
	}
	if filepath.IsAbs(s) {
		return true
	}
	if _, err := os.Stat(s); err == nil {
		return true
	}
	if a.project != nil {
		if _, err := os.Stat(filepath.Join(a.project.Root, s)); err == nil {
			return true
		}
	}
	if filepath.Ext(s) != "" {
		return true // possível novo arquivo (ex.: "api/rest/users.go")
	}
	return false
}

// AgentRun executa o pedido REAL via `cosca exec -m <model> <request>`.
//
// POLÍTICA (ADR-0003): `cosca exec` é a operação MAIS sensível. A autoridade
// NÃO vem do Desktop: o runtime aplica a execpolicy (Allow/Prompt/Forbidden) e
// o trabalho do agente só deve ser executado dentro do fluxo de aprovação
// (plan → approve → exec). O envAllowNoRoot injetado por runtimeEnv apenas
// DECLARA o Windows sem bwrap — ele não autoriza nada. A Desktop NÃO pode
// executar trabalho sensível que o runtime não autorizaria (a UI pede, o
// backend delega ao runtime, o runtime decide).
func (a *App) AgentRun(request, model string) string {
	if a.project == nil {
		return "Sem projeto ativo."
	}
	req := strings.TrimSpace(request)
	if req == "" {
		return "Pedido vazio."
	}
	// GATE (opção B): `cosca exec` é escrita/execução. Sem uma aprovação
	// "approved" registrada para ESTE pedido NÃO dispara o runtime (fail-closed).
	// A autoridade real permanece no runtime (execpolicy + approval) — este gate
	// é apenas o registro local do consentimento do usuário.
	if g := a.requireExecApproval(req); g != "" {
		return g
	}
	defer a.clearApproval() // o consentimento local é consumido ao executar
	if strings.TrimSpace(model) == "" {
		model = "deepseek"
	}
	full := []string{"exec", "-m", model, req}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.runtime, full...)
	cmd.Dir = a.project.Root
	cmd.Env = a.runtimeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)) + "\n[erro: " + err.Error() + "]"
	}
	return strings.TrimSpace(string(out))
}

// AgentRunStream materializa a EXECUÇÃO do agente por EVENTOS REAIS do daemon
// (cliente SSE via /v1/run/stream — mesma fonte de verdade do runtime), em
// vez do request→response bloqueante do `cosca exec`. Diferente de `AgentRun`
// (que roda o binário e devolve a resposta final), o `AgentRunStream` conecta a
// execução ao MECANISMO oficial de streaming do Cosca: consome o SSE, emite cada
// evento via `emitEvent` (cosca:agent:* / cosca:status) e devolve o texto final
// concatenado dos eventos `response`/`done` — para a UI exibir o resultado real.
//
// POLÍTICA: o MESMO gate fail-closed do `AgentRun` (`requireExecApproval`): sem
// uma aprovação `approved` registrada para ESTE pedido NÃO dispara o daemon.
// A autoridade permanece no runtime (execpolicy + approval); aqui apenas
// conectamos e re-apresentamos o que o daemon emite.
//
// FALLBACK HONESTO: se o daemon estiver OFFLINE (`serveHealth` false), NÃO
// fingimos eventos — emitimos via `streamRun` apenas o estado `status` offline
// (via offlineEvents) e devolvemos a mensagem honesta abaixo. NUNCA inventamos
// thinking/response/done que o runtime não produziu.
func (a *App) AgentRunStream(request, model string) string {
	if a.project == nil {
		return "Sem projeto ativo."
	}
	req := strings.TrimSpace(request)
	if req == "" {
		return "Pedido vazio."
	}
	// GATE fail-closed (igual ao AgentRun): sem aprovação "approved" para este
	// pedido NÃO dispara o daemon.
	if g := a.requireExecApproval(req); g != "" {
		return g
	}
	defer a.clearApproval() // o consentimento local é consumido ao executar
	if strings.TrimSpace(model) == "" {
		model = "deepseek"
	}

	// Daemon offline ⇒ fallback honesto: emite o status offline (sem fake) e
	// devolve a mensagem honesta (streamRun já registra/emite o evento).
	if ok, _ := a.serveHealth(); !ok {
		a.streamRun(req, model)
		return "daemon offline — o agente requer cosca serve (SSE) para eventos reais; use o modo exec"
	}

	// Consome o SSE do daemon (emite cada evento via emitEvent) e acumula o
	// texto final das respostas.
	events := a.streamRun(req, model)
	var sb strings.Builder
	for _, ev := range events {
		switch normalizeEventType(ev.Type) {
		case "response", "done":
			content := strings.TrimSpace(ev.Content)
			if content != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(content)
			}
		case "error":
			content := strings.TrimSpace(ev.Content)
			if content != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(content)
			}
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "o agente não retornou texto (sem eventos response/done) — verifique o runtime."
	}
	return strings.TrimSpace(sb.String())
}

// AgentApprove aprova um Plano de Execução REAL via `cosca approve --plan <json>`.
//
// O `cosca approve` recebe o plano via `--plan` (arquivo JSON ou JSON inline —
// exatamente o formato de `cosca plan --json`) e, não-interativo: exibe o
// plano, roda `go test` nos pacotes afetados (TestPackages) e registra a
// aprovação no audit (markdown diário em .cosca/memory/audit/approvals-<data>.md
// e audit store SQLite em .cosca/audit.db). O prompt interativo y/N do Don é
// auto-respondido com "y" (a Desktop já solicita essa aprovação no botão
// "Aprovar e executar"); nenhum trabalho é simulado — a decisão, os testes e o
// registro são os do Cosca real.
func (a *App) AgentApprove(planJSON string) string {
	if a.project == nil {
		return "Sem projeto ativo."
	}
	planJSON = strings.TrimSpace(planJSON)
	if planJSON == "" {
		return "Plano vazio. Passe o JSON de `cosca plan --json` (ou o caminho de um arquivo de plano)."
	}
	full := []string{"approve", "--plan", planJSON}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.runtime, full...)
	cmd.Dir = a.project.Root
	cmd.Env = a.runtimeEnv()
	cmd.Stdin = strings.NewReader("y\n") // auto-aprova o Don (já solicitado na UI)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)) + "\n[erro: " + err.Error() + "]"
	}
	return strings.TrimSpace(string(out))
}

// PlanDetails extrai do JSON real do plano (`cosca plan --json`) os campos que o
// Approval Center precisa exibir: action, command, files, packages, risk,
// policy, scope, cwd. É um PARSE honesto: só preenche o que existe de fato no
// plano; campos ausentes ficam em nil/"—" (nunca inventa). Se o formato não for
// detectável (JSON inválido / não-objeto), devolve `{raw:true, plan:<cru>}` com
// o erro honesto.
//
// Formato conhecido do `cosca plan --json` (ExecutionPlan, sem json tags →
// chaves PascalCase): FilesAffected, Files, TestsExpected, TestPackages,
// Migrations, RollbackAvailable, RollbackDetail, EstimatedMinutes, RiskLevel,
// ConfidencePercent, Method. Aceita também o plano vindo como caminho de arquivo
// (igual ao `cosca approve --plan`).
func (a *App) PlanDetails(planJSON string) map[string]any {
	planJSON = strings.TrimSpace(planJSON)
	// Shape canônica do Approval Center. Defaults HONESTOS: ausente → nil/"—".
	out := map[string]any{
		"action":   nil, // não existe no ExecutionPlan (sem inventar)
		"command":  nil, // não existe no ExecutionPlan (sem inventar)
		"files":    []string{},
		"packages": []string{},
		"risk":     "—",
		"policy":   "—", // não existe no ExecutionPlan (sem inventar)
		"scope":    "—", // não existe no ExecutionPlan (sem inventar)
		"cwd":      nil,
	}
	if planJSON == "" {
		out["raw"] = true
		out["plan"] = planJSON
		out["error"] = "plano vazio"
		return out
	}
	// O plano pode vir como caminho de arquivo (mesma semântica do approve --plan).
	content := planJSON
	if b, err := os.ReadFile(planJSON); err == nil && len(b) > 0 {
		content = strings.TrimSpace(string(b))
	}
	var doc any
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		out["raw"] = true
		out["plan"] = content
		out["error"] = "plano não é JSON detectável: " + err.Error()
		return out
	}
	m, ok := doc.(map[string]any)
	if !ok {
		out["raw"] = true
		out["plan"] = content
		out["error"] = "plano não é um objeto JSON"
		return out
	}
	// Se veio embrulhado em {"plan":{...}} (headless), desce para o objeto real.
	if inner, ok := m["plan"].(map[string]any); ok {
		m = inner
	}

	if v, ok := planValue(m, "Files", "files"); ok {
		if arr, ok2 := toStringSlice(v); ok2 {
			out["files"] = arr
		}
	}
	if v, ok := planValue(m, "TestPackages", "test_packages", "packages"); ok {
		if arr, ok2 := toStringSlice(v); ok2 {
			out["packages"] = arr
		}
	}
	if v, ok := planValue(m, "RiskLevel", "risk", "risk_level"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["risk"] = s
		}
	}
	if v, ok := planValue(m, "Policy", "policy"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["policy"] = s
		}
	}
	if v, ok := planValue(m, "Scope", "scope"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["scope"] = s
		}
	}
	if v, ok := planValue(m, "Action", "action"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["action"] = s
		}
	}
	if v, ok := planValue(m, "Command", "command"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["command"] = s
		}
	}
	if v, ok := planValue(m, "Cwd", "cwd", "ProjectDir", "project_dir"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			out["cwd"] = s
		}
	}

	// Campos extras do ExecutionPlan que o plano REAL traz — enriquecem o
	// Approval Center quando presentes; ausentes não são fabricados.
	if v, ok := planValue(m, "FilesAffected", "files_affected"); ok {
		out["files_affected"] = v
	}
	if v, ok := planValue(m, "TestsExpected", "tests_expected"); ok {
		out["tests_expected"] = v
	}
	if v, ok := planValue(m, "ConfidencePercent", "confidence_percent", "confidence"); ok {
		out["confidence_percent"] = v
	}
	if v, ok := planValue(m, "EstimatedMinutes", "estimated_minutes"); ok {
		out["estimated_minutes"] = v
	}
	if v, ok := planValue(m, "Method", "method"); ok {
		out["method"] = v
	}
	if v, ok := planValue(m, "Migrations", "migrations"); ok {
		out["migrations"] = v
	}
	if v, ok := planValue(m, "RollbackAvailable", "rollback_available"); ok {
		out["rollback_available"] = v
	}
	if v, ok := planValue(m, "RollbackDetail", "rollback_detail"); ok {
		out["rollback_detail"] = v
	}
	return out
}

// planValue devolve o primeiro campo presente em `m` entre as chaves dadas.
func planValue(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

// toStringSlice converte um valor JSON ([]any de scalar) em []string. Retorna
// false se o valor não for um array de scalars — o campo é deixado como default
// honesto ([]string{}) em vez de inventar.
func toStringSlice(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out, true
}

func (a *App) debugJSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

// ---------------------------------------------------------------------------
// FASE A — DETECÇÃO DE ROOT (FORGE vs STANDALONE) — ADR-0007
// ---------------------------------------------------------------------------
//
// O cosca-desktop é um produto INDEPENDENTE com dois modos, definidos por
// DETECÇÃO do root (nunca por cópia):
//   - STANDALONE (fora do root): Agents + Skills, SEM kernel e SEM memória da
//     família. Opera via IA externa/local.
//   - FORGE (no root): entra o KERNEL e revela TUDO do root (agents, skills,
//     memória da família, config, arquitetura, family chain, DNA).
//
// Detectar é LER do disco (os.Stat), NUNCA copiar conteúdo para o binário. Não
// carregamos a memória da família para o estado — apenas sinalizamos a presença
// do root e contamos/descrevemos a estrutura (agents/skills/ADRs).

// RootMode reporta se o Desktop está em modo FORGE (no root do framework Cosca).
// false => STANDALONE (fora do root, sem kernel/memória da família).
func (a *App) RootMode() bool { return a.rootMode }

// DetectRoot re-avalia o diretório ativo (projeto ativo ou cwd) contra os
// sinais de root do framework Cosca e atualiza rootMode/rootDir/rootInfo.
// Roda no startup e sempre que um projeto é aberto/criado/fechado. Retorna o
// RootInfo correspondente.
func (a *App) DetectRoot() map[string]any {
	dir := a.activeRootDir()
	root := a.isCoscaRoot(dir)
	a.rootMode = root
	a.rootDir = dir
	return a.buildRootInfo(dir, root)
}

// RootInfo retorna uma ESTRUTURA do que foi detectado no root (para a UI exibir
// "tudo do root" de forma estruturada). Sempre re-detecta sobre o estado atual.
// Se não em rootMode, retorna `{root: false}`.
func (a *App) RootInfo() map[string]any { return a.DetectRoot() }

// activeRootDir devolve o diretório-alvo da detecção: o root do projeto ativo,
// ou o diretório de trabalho do processo (quando não há projeto).
func (a *App) activeRootDir() string {
	if a.project != nil && a.project.Root != "" {
		return a.project.Root
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// isCoscaRoot reporta se `dir` tem sinais de ser o ROOT do framework Cosca
// (o projeto root). Critérios (os.Stat/isDir):
//   - tem `internal/embed/cosca/` (árvore do embed), OU
//   - tem `AGENT_DNA.md`, OU
//   - tem `.opencode/cosca/` (framework), OU
//   - tem a árvore raiz do cosca (ex.: `cmd/cosca` + `internal/kernel`).
func (a *App) isCoscaRoot(dir string) bool {
	if dir == "" {
		return false
	}
	if isDir(filepath.Join(dir, "internal", "embed", "cosca")) {
		return true
	}
	if isFile(filepath.Join(dir, "AGENT_DNA.md")) {
		return true
	}
	if isDir(filepath.Join(dir, ".opencode", "cosca")) {
		return true
	}
	if isDir(filepath.Join(dir, "cmd", "cosca")) && isDir(filepath.Join(dir, "internal", "kernel")) {
		return true
	}
	return false
}

// buildRootInfo constrói o mapa estruturado do que foi detectado. Em modo root
// retorna `{root, has_embed, has_dna, has_opencode_cosca, agents, skills,
// adr_count, adrs, paths}`; caso contrário `{root: false}`.
func (a *App) buildRootInfo(dir string, root bool) map[string]any {
	if !root || dir == "" {
		return map[string]any{"root": false}
	}
	embedRoot := filepath.Join(dir, "internal", "embed", "cosca")
	agentsDir := filepath.Join(embedRoot, "agents")
	skillsDir := filepath.Join(embedRoot, "skills")
	adrDir := filepath.Join(dir, "docs", "adr")

	hasEmbed := isDir(embedRoot)
	agents := 0
	skills := 0
	if hasEmbed {
		agents = countDirEntries(agentsDir)
		skills = countDirEntries(skillsDir)
	}

	// Lista leve dos ADRs presentes em docs/adr (leitura de nomes, não conteúdo).
	adrs := []string{}
	if entries, err := os.ReadDir(adrDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				adrs = append(adrs, e.Name())
			}
		}
	}
	sort.Strings(adrs)

	info := map[string]any{
		"root":               true,
		"root_dir":           dir,
		"has_embed":          hasEmbed,
		"has_dna":            isFile(filepath.Join(dir, "AGENT_DNA.md")),
		"has_opencode_cosca": isDir(filepath.Join(dir, ".opencode", "cosca")),
		"agents":             agents,
		"skills":             skills,
		"adr_count":          len(adrs),
		"adrs":               adrs,
		"paths": map[string]string{
			"root":     dir,
			"embed":    embedRoot,
			"agents":   agentsDir,
			"skills":   skillsDir,
			"docs_adr": adrDir,
		},
	}
	return info
}

// countDirEntries conta as entradas (arquivos + subdiretórios) de um diretório.
// Leitura leve; retorna 0 se o diretório não existe ou dá erro.
func countDirEntries(p string) int {
	entries, err := os.ReadDir(p)
	if err != nil {
		return 0
	}
	return len(entries)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ---------------------------------------------------------------------------
// FASE A — CLIENTE DO DAEMON (canal de eventos oficial do Cosca) — ADR-0007
// ---------------------------------------------------------------------------
//
// O mecanismo oficial de streaming do Cosca é o daemon `cosca serve`
// (REST + SSE + WebSocket). O Desktop CONSUME via client HTTP/SSE — não
// reimplementa eventos nem importa `internal/agentbridge`.
//
// A autoridade permanece no runtime: aqui apenas CONECTAMOS e REAPRESENTAMOS.
// Não reconstruímos execpolicy/gate/rails/agentbridge/memory/knowledge. Se o
// runtime não devolver algo, marcamos `NOT AVAILABLE` honesto.

const (
	// serveDefaultBaseURL é o default do `cosca serve` (api/rest/server.go:
	// Host 127.0.0.1, Port 14120). Pode ser sobrescrito via env COSCA_SERVE_URL.
	serveDefaultBaseURL = "http://127.0.0.1:14120"
	// serveHealthTimeout limita a sondagem de liveness do daemon.
	serveHealthTimeout = 5 * time.Second
	// serveRunTimeout limita a duração de uma execução/stream via daemon.
	serveRunTimeout = 180 * time.Second
	// maxBufferedEvents limita o buffer de eventos retido p/ a UI.
	maxBufferedEvents = 1000
)

// ── Nomes canônicos dos eventos Desktop→UI (cosca:agent:* / cosca:status) ─────
//
// São os canais que a UI assina via Wails EventsOn. Como NÃO reimplementamos o
// protocolo do runtime, estes apenas NOMEIAM a re-apresentação de um evento que
// o daemon (SSE /v1/run/stream) ou o agentbridge realmente emitiu. Tipos que o
// SSE run/stream NÃO produz (ex.: reasoning-delta, file-change, finish-step,
// finish) têm canais reservados, mas só são emitidos se/ad quando o runtime os
// de fato emitir — nunca fabricamos um evento que o runtime não produziu.
const (
	evtThinking  = "cosca:agent:thinking"  // thinking | reasoning-delta
	evtResponse  = "cosca:agent:response"  // response | token
	evtToolCall  = "cosca:agent:tool-call" // tool-call | tool-result
	evtWriting   = "cosca:agent:writing"   // file-change / diff
	evtExecuting = "cosca:agent:executing" // finish-step
	evtDone      = "cosca:agent:done"      // done
	evtCompleted = "cosca:agent:completed" // finish
	evtError     = "cosca:agent:error"     // error
	evtStatus    = "cosca:status"          // status (liveness/offline honesto)
)

// serveBaseURL devolve a base do daemon (ex.: http://127.0.0.1:14120), lendo o
// env COSCA_SERVE_URL com o default do `cosca serve`.
func (a *App) serveBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("COSCA_SERVE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return serveDefaultBaseURL
}

// serveHealth sonda o daemon (GET /health — liveness sem auth) para o Runtime
// Monitor saber se está de pé. Sem fake: se falhar retorna false + motivo.
func (a *App) serveHealth() (bool, string) {
	url := a.serveBaseURL() + "/health"
	client := &http.Client{Timeout: serveHealthTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return false, fmt.Sprintf("daemon não acessível em %s: %s", a.serveBaseURL(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("daemon respondeu %s em %s", resp.Status, url)
	}
	return true, url
}

// RuntimeEvent é o modelo de evento que o Desktop re-apresenta para a UI.
// Mapeia os eventos do runtime para os ESTADOS do agente que a UI entende:
// thinking | response | tool-call | tool-result | file-change | finish-step |
// finish | done | error | status. O campo Type é sempre o NORMALIZADO (grosso
// modo o do runtime, mas com sinonímias resolvidas: reasoning-delta→thinking,
// token→response, tool_call→tool-call). O campo TipoBruto preserva o que o
// runtime de fato emitiu (referência de auditoria, nunca inventado).
type RuntimeEvent struct {
	Type      string `json:"type"`       // estado normalizado do agente (ver acima)
	TipoBruto string `json:"raw_type"`   // tipo bruto do frame SSE do runtime (auditoria, sem fake)
	Seq       int    `json:"seq"`        // sequência monotônica dentro do buffer
	Content   string `json:"content"`    // payload textual do evento
	SessionID string `json:"session_id"` // id da sessão (vazio quando o runtime não a envia)
	At        string `json:"at"`         // timestamp RFC3339
}

// AgentStream é o método de COLETA da FASE A: executa uma chamada ao daemon e
// devolve os eventos capturados (bloqueante, mas honesto). O streaming contínuo
// assíncrono fica para uma fase posterior; neste bloco a UI pode usar o buffer
// (ConsumeEvents/SubscribeRuntime) + eventos Wails (EventsEmit).
func (a *App) AgentStream(request, model string) []RuntimeEvent {
	return a.streamRun(request, model)
}

// streamRun executa o consumo real do daemon via SSE: POST `/v1/run/stream`
// (fallback `/v1/run`), lê `text/event-stream` linha a linha, parseia os frames
// `data: {...}` e emite (via emitEvent) + registra cada evento. Se o daemon NÃO
// estiver de pé, emite `cosca:status` = "daemon offline" e cai no fallback
// honesto (não inventa evento).
func (a *App) streamRun(request, model string) []RuntimeEvent {
	if strings.TrimSpace(request) == "" {
		return a.offlineEvents("pedido vazio — nada a executar")
	}

	// Daemon fora do ar ⇒ fallback honesto (status offline), sem fake.
	if ok, why := a.serveHealth(); !ok {
		return a.offlineEvents("daemon offline: " + why)
	}

	payload, err := json.Marshal(map[string]any{
		"prompt":   request,
		"provider": strings.TrimSpace(model),
	})
	if err != nil {
		return a.offlineEvents("falha ao montar request: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), serveRunTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.serveBaseURL()+"/v1/run/stream", bytes.NewReader(payload))
	if err != nil {
		return a.offlineEvents("falha ao criar request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return a.offlineEvents("daemon request falhou: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return a.offlineEvents(fmt.Sprintf("daemon respondeu %s: %s", resp.Status, strings.TrimSpace(string(b))))
	}

	events := []RuntimeEvent{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
		if data == "" {
			continue
		}
		var env sseEnvelope
		if err := json.Unmarshal([]byte(data), &env); err != nil {
			continue // frame malformado é ignorado (não inventamos conteúdo)
		}
		rawTyp := strings.TrimSpace(env.Type)
		if rawTyp == "" {
			continue
		}
		content := env.Content
		if rawTyp == "error" { // output ou erro: honesto, vem do runtime
			if content == "" {
				content = env.Error
			}
		}
		// Normaliza o tipo bruto do runtime para o estado do agente que a UI
		// entende (reasoning-delta→thinking, token→response, tool_call→tool-call...).
		typ := normalizeEventType(rawTyp)
		ev := a.eventOf(typ, content)
		ev.TipoBruto = rawTyp
		ev.SessionID = env.SessionID
		a.emitRuntime(ev)
		events = append(events, ev)
		if typ == "done" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		ev := a.eventOf("error", "sse read error: "+err.Error())
		a.emitRuntime(ev)
		events = append(events, ev)
	}
	return events
}

// sseEnvelope é o frame `data: {...}` emitido pelo runtime (api/stream/sse.go):
//
//	data: {"type":"<type>","content":"<content>"}
//	data: {"type":"done","duration_ms":<ms>}
//	data: {"type":"error","content":"<err>"}
//
// O campo `session_id` e os tipos estendidos (reasoning-delta, tool-call,
// tool-result, file-change, finish-step, finish) chegam quando o runtime os
// produz; nós apenas re-apresentamos sem inventar.
type sseEnvelope struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	Error      string `json:"error"`
	SessionID  string `json:"session_id"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// offlineEvents é o fallback honesto (daemon offline / falha): emite um único
// evento de status "offline" (marcando NOT AVAILABLE) e devolve-o. NUNCA inventa
// thinking/response/done que o runtime não produziu.
func (a *App) offlineEvents(reason string) []RuntimeEvent {
	ev := a.eventOf("status", reason)
	a.emitRuntime(ev)
	return []RuntimeEvent{ev}
}

// eventOf cria um RuntimeEvent com sequência monotônica e timestamp.
func (a *App) eventOf(typ, content string) RuntimeEvent {
	a.eventsMu.Lock()
	a.eventSeq++
	seq := a.eventSeq
	a.eventsMu.Unlock()
	return RuntimeEvent{
		Type:    typ,
		Seq:     seq,
		Content: content,
		At:      time.Now().Format(time.RFC3339),
	}
}

// emitRuntime registra um evento no buffer e o EMITE para a UI via Wails
// (EventsEmit). É o canal Desktop→UI (reapresentação), mapeando os tipos do
// runtime para os nomes de evento que a UI assina.
func (a *App) emitRuntime(ev RuntimeEvent) {
	a.eventsMu.Lock()
	a.events = append(a.events, ev)
	if len(a.events) > maxBufferedEvents {
		a.events = a.events[len(a.events)-maxBufferedEvents:]
	}
	a.eventsMu.Unlock()
	a.emitEvent(eventNameForRuntimeType(ev.Type), ev)
}

// normalizeEventType reduz o tipo bruto do frame SSE do runtime ao ESTADO do
// agente que a UI entende. Sinonímias documentadas:
//
//	thinking → thinking            reasoning-delta → thinking
//	response → response            token           → response
//	tool-call → tool-call          tool_call        → tool-call
//	tool-result → tool-result      tool_result      → tool-result
//	file-change → file-change      file_change      → file-change
//	finish-step → finish-step      finish_step      → finish-step
//	finish → finish                done → done      error → error   status → status
//
// Tipos que o SSE run/stream NÃO emite (ex.: reasoning-delta, file-change,
// finish-step, finish) só aparecem aqui como RESERVA HONESTA: nós mapeamos o nome
// mas nunca fabricamos um evento que o runtime não produziu. Tipos fora desta
// tabela passam intactos (canal default cosca:agent:<tipo>).
func normalizeEventType(raw string) string {
	switch raw {
	case "thinking", "reasoning-delta":
		return "thinking"
	case "response", "token":
		return "response"
	case "tool-call", "tool_call":
		return "tool-call"
	case "tool-result", "tool_result":
		return "tool-result"
	case "file-change", "file_change":
		return "file-change"
	case "finish-step", "finish_step":
		return "finish-step"
	case "finish":
		return "finish"
	case "done":
		return "done"
	case "error":
		return "error"
	case "status":
		return "status"
	}
	return raw
}

// eventNameForRuntimeType mapeia o estado normalizado do agente para o nome de
// evento que a UI assina via EventsOn (wailsjs/runtime): cosca:agent:* e
// cosca:status. Usa as constantes canônicas (evt*). Não inventamos protocolo
// paralelo — apenas nomeamos o canal Desktop→UI que re-apresenta o evento do
// runtime. tool-result re-apresenta-se como tool-call (estado do agente: a
// ferramenta foi usada); file-change como writing; finish-step como executing;
// finish como completed.
func eventNameForRuntimeType(typ string) string {
	switch normalizeEventType(typ) {
	case "thinking":
		return evtThinking
	case "response":
		return evtResponse
	case "tool-call":
		return evtToolCall
	case "tool-result":
		return evtToolCall // estado do agente: tool-call (resultado de tool)
	case "file-change":
		return evtWriting
	case "finish-step":
		return evtExecuting
	case "finish":
		return evtCompleted
	case "done":
		return evtDone
	case "error":
		return evtError
	case "status":
		return evtStatus
	default:
		return "cosca:agent:" + typ
	}
}

// emitEvent emite um evento estruturado para a UI via Wails. É um no-op quando
// o contexto da UI não está disponível (ex.: em testes, onde startup não roda).
func (a *App) emitEvent(name string, data any) {
	if a.ctx == nil {
		return // sem UI registrada (testes/standalone) — não fatal (log.Fatalf no Wails)
	}
	runtime.EventsEmit(a.ctx, name, data)
}

// ConsumeEvents esvazia o buffer e devolve os eventos capturados (light polling).
func (a *App) ConsumeEvents() []RuntimeEvent {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	evs := a.events
	a.events = nil
	return evs
}

// SubscribeRuntime devolve uma cópia dos eventos mais recentes SEM esvaziar o
// buffer (para a UI pegar o estado atual sem perder eventos pendentes).
func (a *App) SubscribeRuntime() []RuntimeEvent {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	out := make([]RuntimeEvent, len(a.events))
	copy(out, a.events)
	return out
}

// ---------------------------------------------------------------------------
// FASE A — FALLBACK HONESTO (standalone / offline)
// ---------------------------------------------------------------------------

// CapabilityState expõe o estado REAL de capacidade do Desktop. Se standalone
// → kernel:false, family_memory:false (sem fingir que há kernel/memória). Em
// forge → kernel:true (e memória da família disponível, acessada do root).
// O campo `daemon` reflete a sonda real (sem fake).
func (a *App) CapabilityState() map[string]any {
	info := a.DetectRoot() // re-detecta sobre o estado atual
	mode := "standalone"
	kernel := false
	if a.rootMode {
		mode = "forge"
		kernel = true
	}
	daemon, _ := a.serveHealth()
	agents, _ := info["agents"].(int)
	skills, _ := info["skills"].(int)
	return map[string]any{
		"mode":          mode,
		"kernel":        kernel,
		"family_memory": kernel, // só acessível via root/kernel (FORGE)
		"daemon":        daemon,
		"agents":        agents,
		"skills":        skills,
	}
}

// ---------------------------------------------------------------------------
// FASE C — DIFF ENGINE HONESTA (consome `git diff`, não reimplementa diff)
// ---------------------------------------------------------------------------
//
// A experiência de Diff do Desktop NÃO fabrica patch: ela CONSUME o mecanismo
// real do repositório (`git diff` / `git status`) e re-apresenta o que o git
// realmente produz. Não importamos `internal/...` do cosca — usamos apenas o
// binário git (os/exec, cmd.Dir = raiz do projeto, com timeout). Caminhos
// sensíveis (.cosca/*, .env, segredos) passam pelo MESMO guard do Desktop
// (isSensitivePath, fail-closed — ADR-0002) e nunca chegam à UI.

// DiffItem é um item de mudança do repositório, extraído de `git status`
// (status + path) e `git diff HEAD --numstat` (additions/deletions). O patch é
// opcional e limitado, vindo de `git diff HEAD -- <path>`.
type DiffItem struct {
	Status    string `json:"status"`          // A (added) | M (modified) | D (deleted) | R (renamed) | ?? (untracked)
	Path      string `json:"path"`            // caminho relativo à raiz do projeto (slashes)
	Additions int    `json:"additions"`       // linhas adicionadas (git diff --numstat)
	Deletions int    `json:"deletions"`       // linhas removidas (git diff --numstat)
	Patch     string `json:"patch,omitempty"` // diff unificado de `git diff HEAD -- <path>` (limitado)
}

// DiffSummary agrega o resumo das mudanças do projeto (a partir dos itens).
type DiffSummary struct {
	Files      int `json:"files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

// Limites de performance/segurança da coleta de diff. Evitam que um repo grande
// ou cheio de conteúdo seja todo carregado para a WebView (que vazaria dados e
// estouraria a UI).
const (
	gitDiffTimeout    = 20 * time.Second // timeout de TODA a coleta git
	diffMaxItems      = 200              // máx. de itens retornados
	diffMaxPatchFiles = 20               // máx. de arquivos com patch (limita custo por item)
	diffMaxPatchBytes = 8000             // máx. de bytes por patch (limita payload)
)

// ProjectDiff consome `git diff` do projeto ativo e devolve a experiência de
// diff REAL (honesta): só informa o que o git realmente reporta.
//
//   - Sem projeto ativo            → {git:false, reason:"sem projeto ativo", ...}
//   - Sem repositório git (sem .git) → {git:false, reason:"não é repositório git", items:[], honesto:true} (NÃO inventa)
//   - Repo git                      → {git:true, branch, items:[{status,path,additions,deletions}], summary, honesto:true}
//
// Segurança/performance:
//   - roda git com cmd.Dir = a.project.Root (não escapa a raiz do projeto);
//   - filtra caminhos sensíveis (isSensitivePath — ADR-0002) e REJEITA itens
//     fora da raiz do projeto (escape via toplevel ≠ projeto);
//   - limita nº de itens, nº de patches e tamanho de cada patch (constantes);
//   - Timeout global (gitDiffTimeout) para não travar a UI.
func (a *App) ProjectDiff() map[string]any {
	if a.project == nil || a.project.Root == "" {
		return map[string]any{"git": false, "reason": "sem projeto ativo", "items": []DiffItem{}, "honesto": true}
	}
	root := a.project.Root
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return map[string]any{"git": false, "reason": "não é repositório git", "items": []DiffItem{}, "honesto": true}
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitDiffTimeout)
	defer cancel()

	branch := strings.TrimSpace(a.gitOutput(ctx, root, "branch", "--show-current"))
	// Toplevel do repo: usamos para resolver o caminho relativo de cada item à
	// raiz do PROJETO (um repo pai ou um projeto em subpasta é tratado de forma
	// honesta: itens fora da raiz do projeto são rejeitados).
	toplevel := strings.TrimSpace(a.gitOutput(ctx, root, "rev-parse", "--show-toplevel"))
	if toplevel == "" {
		toplevel = root
	}
	// `--untracked-files=all` expande diretórios não-rastreados em arquivos
	// individuais; sem ele git colapsaria `.cosca/` em uma única entrada `?? .cosca/`
	// que o guard sensível NÃO bloquearia — o que vazaria `serve.env`/`keys`.
	porcelain := a.gitOutput(ctx, root, "status", "--porcelain", "--untracked-files=all")
	numstat := a.gitNumstat(ctx, root, toplevel)

	items := []DiffItem{}
	patchBudget := diffMaxPatchFiles
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		status, gitPath := parsePorcelainLine(line)
		if gitPath == "" {
			continue
		}
		// Caminho relativo à raiz do projeto. Se cair FORA do projeto (escape)
		// ou for sensível (.cosca/*, .env, segredos), é rejeitado (fail-closed).
		rel := a.relToProject(root, toplevel, gitPath)
		if rel == "" || isSensitivePath(rel) {
			continue
		}
		if len(items) >= diffMaxItems {
			break
		}
		item := DiffItem{Status: status, Path: rel}
		if ns, ok := numstat[rel]; ok {
			item.Additions = ns.add
			item.Deletions = ns.del
		} else if status == "??" {
			// Não-rastreado não aparece no `git diff --numstat`; contamos as
			// linhas do arquivo como additions (best-effort, honesto — é o
			// conteúdo real do arquivo, não um patch fabricado).
			if n := countFileLines(filepath.Join(root, filepath.FromSlash(rel))); n >= 0 {
				item.Additions = n
			}
		}
		// Patch opcional, limitado a N arquivos (custo) — só quando o git
		// realmente produz (arquivo rastreado alterado/adicionado/deletado).
		if patchBudget > 0 {
			if p := a.gitPatch(ctx, root, rel); p != "" {
				item.Patch = p
				patchBudget--
			}
		}
		items = append(items, item)
	}

	summary := DiffSummary{Files: len(items)}
	for _, it := range items {
		summary.Insertions += it.Additions
		summary.Deletions += it.Deletions
	}
	return map[string]any{
		"git":     true,
		"branch":  branch,
		"items":   items,
		"summary": summary,
		"honesto": true,
	}
}

// DiffSnapshot é o ponto único que a UI chama para recarregar o diff após uma
// execução do agente (eventos file-change / tool-result). Delega a ProjectDiff()
// — a fonte de verdade (git). A UI não conhece os detalhes: apenas re-consulta o
// snapshot atual (mesmo contrato de ProjectDiff()).
func (a *App) DiffSnapshot() map[string]any {
	return a.ProjectDiff()
}

// gitOutput roda `git <args...>` no diretório `dir` (cmd.Dir — impede escapar a
// raiz do projeto) e devolve a saída do STDIN/STDOUT. Apenas stdout é capturado:
// avisos do git (ex.: "LF will be replaced by CRLF") vão para stderr e NÃO
// corrompem o parsing. Erro (comando não encontrado, timeout, saída ≠ 0) => "".
func (a *App) gitOutput(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// diffCount guarda as contagens de um arquivo a partir de `git diff --numstat`.
type diffCount struct{ add, del int }

// gitNumstat coleciona `git diff HEAD --numstat` (diff combinado staged+unstaged
// vs HEAD — a vista que o usuário espera) e devolve um mapa path→contagens com
// os caminhos RESOLVIDOS para a raiz do projeto. Rejeita itens fora do projeto.
func (a *App) gitNumstat(ctx context.Context, root, toplevel string) map[string]diffCount {
	out := map[string]diffCount{}
	raw := a.gitOutput(ctx, root, "diff", "HEAD", "--numstat")
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		// Renames: numstat usa "old => new" — usamos o NOVO caminho (o mesmo
		// que o status porcelain reporta como caminho atual do item).
		if i := strings.Index(path, " => "); i >= 0 {
			path = path[i+4:]
		}
		rel := a.relToProject(root, toplevel, path)
		if rel == "" {
			continue
		}
		add := parseDiffCount(parts[0])
		del := parseDiffCount(parts[1])
		if ex, ok := out[rel]; ok {
			ex.add += add
			ex.del += del
			out[rel] = ex
		} else {
			out[rel] = diffCount{add: add, del: del}
		}
	}
	return out
}

// gitPatch devolve o diff unificado de `git diff HEAD -- <rel>` (limitado a
// diffMaxPatchBytes). Devolve "" quando o git não produz patch (arquivo
// não-rastreado, ou erro) — nunca fabricamos um patch que o git não gerou.
func (a *App) gitPatch(ctx context.Context, root, rel string) string {
	p := a.gitOutput(ctx, root, "diff", "HEAD", "--", filepath.FromSlash(rel))
	if strings.TrimSpace(p) == "" {
		return ""
	}
	if len(p) > diffMaxPatchBytes {
		p = p[:diffMaxPatchBytes] + "\n... [patch truncado por limite de tamanho]"
	}
	return p
}

// parsePorcelainLine extrai o status canônico e o caminho de uma linha de
// `git status --porcelain` (v1). Formato: "XY<espaço><caminho>", em que X é o
// status do index e Y o do worktree; renames vêm como "XY old -> new".
// Devolve (status ∈ A|M|D|R|??, caminho_ATUAL).
func parsePorcelainLine(line string) (status, path string) {
	if len(line) < 4 {
		return "", ""
	}
	xy := line[0:2]
	if xy == "??" {
		return "??", strings.TrimPrefix(line[3:], "./")
	}
	s := canonicalStatus(xy[0])
	if s == "" {
		s = canonicalStatus(xy[1])
	}
	p := strings.TrimSpace(line[3:])
	if i := strings.Index(p, " -> "); i >= 0 {
		p = strings.TrimSpace(p[i+4:])
	}
	return s, strings.TrimPrefix(p, "./")
}

// canonicalStatus reduz um caracter de status do git ao token canônico do
// Desktop (A/M/D/R). Caracteres não reconhecidos (' ' etc.) ficam vazios.
func canonicalStatus(ch byte) string {
	switch ch {
	case 'A':
		return "A"
	case 'M':
		return "M"
	case 'D':
		return "D"
	case 'R':
		return "R"
	case 'C':
		return "R" // cópia tratada como rename (sem token próprio no spec)
	}
	return ""
}

// parseDiffCount converte o contador de `--numstat` para int ("-" = binário => 0).
func parseDiffCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// relToProject resolve o caminho que o git reporta (relativo à TOPLEVEL do repo)
// para um caminho relativo à RAIZ DO PROJETO (normalizado p/ slash). Devolve ""
// se o item cair FORA da raiz do projeto (escape) — nunca expomos conteúdo fora
// do projeto (ADR-0002/defesa em profundidade).
func (a *App) relToProject(root, toplevel, gitPath string) string {
	gitPath = strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(gitPath), "./"), "/")
	if gitPath == "" {
		return ""
	}
	abs := filepath.Join(toplevel, filepath.FromSlash(gitPath))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "" // fora do projeto
	}
	return rel
}

// countFileLines conta as linhas de um arquivo (usado para `??` não-rastreados,
// que o git não inclui no numstat). Returna -1 quando não é legível.
func countFileLines(p string) int {
	b, err := os.ReadFile(p)
	if err != nil {
		return -1
	}
	if len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte("\n"))
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

// ---------------------------------------------------------------------------
// FASE D — TERMINAL AUTORIZADO E HONESTO (registro de política do runtime)
// ---------------------------------------------------------------------------
//
// REGRA DE OURO (contrato do Don): o Terminal do cosca-desktop NÃO é um shell
// livre que contorna a autoridade do runtime. A autoridade de qualquer comando
// (execpolicy Allow/Prompt/Forbidden + Rails de filesystem + approval) é do
// RUNTIME Cosca; ele NÃO expõe endpoint REST de spawn de processo (somente
// /v1/run, pipeline, execuções; `cosca terminal` é um TUI que roda FORA do jail
// em modo OpenCode). Portanto o Terminal do Desktop é um PANE que orquestra
// comandos ATRAVÉS da autoridade — nunca um `exec` arbitrário direto.
//
// O que este bloco faz:
//   - `RunCommand(cmdline)`              → o terminal do usuário, que roda o
//     comando NO PROJETO ATIVO (cwd correto, sem escapar a raiz), com timeout,
//     COM o gate fail-closed de aprovação e COM o filtro de perigo. NÃO executa
//     fora da autoridade; se for perigoso recusa honestamente.
//   - `RunCommandAuthorized(cmdline)`    → variante que exige aprovação para
//     comandos sensíveis/perigosos (recusa perigo sem executar e deixa claro o
//     caminho via execpolicy).
//   - `isDangerousCommand(cmdline)`      → filtro de perigo replicado (sem
//     importar `internal/...`) do `internal/policy/dangerous.go`.
//   - `isReadOnlyCommand(cmdline)`       → heurística fail-closed: só é
//     "read-only" (segura direto) um conjunto notório; qualquer comando fora
//     disso é tratado como escrita/execução e passa pelo gate de aprovação.
//
// POR QUE NÃO É UM SHELL LIVRE:
//  1. O comando roda com `cmd.Dir = a.project.Root` — nunca fora da raiz do
//     projeto (não há "~/", "/tmp", ".." escapando).
//  2. Antes de executar, passa por `requireExecApproval` (o MESMO gate fail-
//     closed do fluxo do agente) para operações de escrita/execução.
//  3. `isDangerousCommand` recusa comandos claramente perigosos (rm -rf, sudo,
//     curl|sh, ssh, git push --force, operadores | && ; `) sem qualquer execução.
//  4. A autoridade FINAL dos comandos do AGENTE permanece no runtime via
//     `cosca exec` / Approval Center — este terminal é apenas o pane do usuário,
//     confinado à política. Nada aqui concede um poder que o runtime negaria.
const (
	// terminalCommandTimeout limita a duração de um comando do Terminal
	// autorizado (evita travar a UI e um processo pendurado).
	terminalCommandTimeout = 120 * time.Second

	cmdEmptyMsg    = "comando vazio — nada a executar"
	cmdNoProject   = "Sem projeto ativo — crie/abra um projeto primeiro."
	cmdExecErrSufx = "\n[erro: "

	// cmdDangerousRefusal é a recusa honesta do `RunCommand` para um comando
	// perigoso (não executa; aponta a autoridade do runtime). %s = padrão casado.
	cmdDangerousRefusal = "comando perigoso recusado (padrão: %s) — a autoridade de execução é do runtime Cosca (execpolicy Allow/Prompt/Forbidden + approval); o Desktop não executa comandos que contornem essa autoridade. Execute via cosca exec / Approval Center."

	// cmdDangerousAuthorizedMsg é a recusa exata do `RunCommandAuthorized` para
	// um comando perigoso (honesta, NÃO executa — exige aprovação via execpolicy).
	cmdDangerousAuthorizedMsg = "comando perigoso — requer aprovação via execpolicy (Allow/Prompt/Forbidden) do runtime; não executado no Desktop"
)

// RunCommand executa um comando pelo MECANISMO AUTORIZADO do runtime (não por
// exec direto). Contrato:
//
//   - Rejeita cmdline vazio (err honesto).
//   - Requer projeto ativo (roda NO PROJETO — `cmd.Dir = a.project.Root`).
//   - Filtra perigo via `isDangerousCommand`: se perigoso, RECUSA honestamente
//     (exigindo aprovação/execpolicy) e NÃO executa.
//   - Operações de escrita/execução passam pelo MESMO gate fail-closed do fluxo
//     do agente (`requireExecApproval`) — sem aprovação "approved" registrada
//     para ESTE comando, não roda. O consentimento local é consumido (limpo)
//     após executar. Comandos notoriamente READ-ONLY rodam direto (são leitura).
//   - Devolve stdout+stderr ou a mensagem honesta de recusa/erro.
func (a *App) RunCommand(cmdline string) string {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return cmdEmptyMsg
	}
	if a.project == nil || a.project.Root == "" {
		return cmdNoProject
	}
	// Filtro de perigo (fail-closed): comando claramente perigoso é recusado
	// ANTES de qualquer gate/exec — nunca roda fora da autoridade.
	if pat, ok := isDangerousCommand(cmdline); ok {
		return fmt.Sprintf(cmdDangerousRefusal, pat)
	}
	// GATE de aprovação para escrita/execução (mesmo gate do fluxo do agente).
	if !isReadOnlyCommand(cmdline) {
		if g := a.requireExecApproval(cmdline); g != "" {
			return g
		}
		defer a.clearApproval() // o consentimento local é consumido ao executar
	}
	return a.execProjectCommand(cmdline)
}

// RunCommandAuthorized é a versão que EXIGE aprovação para comandos
// sensíveis/perigosos. Contrato:
//
//   - Se `isDangerousCommand` → devolve a recusa honesta
//     (`cmdDangerousAuthorizedMsg`) e NÃO executa (a autoridade do comando
//     perigoso é o runtime: Allow/Prompt/Forbidden via execpolicy).
//   - Se não perigoso mas de escrita/execução → passa pelo `requireExecApproval`
//     (gate fail-closed do Desktop) e limpa a aprovação após executar.
//   - Senão (read-only) → executa no projeto.
func (a *App) RunCommandAuthorized(cmdline string) string {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return cmdEmptyMsg
	}
	if a.project == nil || a.project.Root == "" {
		return cmdNoProject
	}
	if pat, ok := isDangerousCommand(cmdline); ok {
		_ = pat
		return cmdDangerousAuthorizedMsg
	}
	if !isReadOnlyCommand(cmdline) {
		if g := a.requireExecApproval(cmdline); g != "" {
			return g
		}
		defer a.clearApproval()
	}
	return a.execProjectCommand(cmdline)
}

// execProjectCommand executa `cmdline` no projeto ativo (cmd.Dir = a.project.Root
// — nunca fora) com timeout. É o "modo terminal autorizado do Desktop": o comando
// roda com o ambiente do runtime (runtimeEnv → COSCA_PROJECT aponta para a raiz
// do projeto). Os chamadores (RunCommand/RunCommandAuthorized) já aplicaram o
// filtro de perigo e o gate de aprovação — aqui apenas confinamos a execução à
// raiz do projeto e devolvemos a saída real (stdout+stderr).
func (a *App) execProjectCommand(cmdline string) string {
	if a.project == nil || a.project.Root == "" {
		return cmdNoProject
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalCommandTimeout)
	defer cancel()
	cmd := buildShellCmd(ctx, cmdline)
	cmd.Dir = a.project.Root
	cmd.Env = a.runtimeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimRight(string(out), "\r\n") + cmdExecErrSufx + err.Error() + "]"
	}
	return strings.TrimRight(string(out), "\r\n")
}

// buildShellCmd monta o comando do shell NATIVO (cmd.exe no Windows; sh -c no
// POSIX) vinculado ao contexto `ctx` (timeout do terminal). NÃO é um shell livre
// contornando a autoridade: ele é CONFINADO por `isDangerousCommand` (filtro de
// operadores/perigo) e pelo gate de aprovação — é apenas o meio de expressar a
// linha de comando do usuário no SO, dentro da raiz do projeto.
func buildShellCmd(ctx context.Context, cmdline string) *exec.Cmd {
	if os.PathSeparator == '\\' {
		return exec.CommandContext(ctx, "cmd", "/C", cmdline)
	}
	return exec.CommandContext(ctx, "sh", "-c", cmdline)
}

// isDangerousCommand replica (sem importar `internal/...`) a detecção de padrão
// perigoso do cosca (`internal/policy/dangerous.go` → MatchesDangerousPattern).
// Detecção determinística por substring/operador de shell (NÃO é um parser de
// shell/AST — anti over-engineering). Retorna (padrão_casado, true) quando o
// comando é perigoso, ou ("", false) quando limpo.
//
// Padrões reconhecidos (fail-closed): "rm -rf"/"rm -r "/"rm -fr" (destrutivo),
// "sudo" (elevação), "curl ... | sh/bash" (download-e-executa), "ssh", "git push
// --force"/"git push -f" (reescrita de histórico) e os operadores de composição
// "|", "&&", ";", "`" (injeção/composição). Comparação case-insensitive.
func isDangerousCommand(cmdline string) (string, bool) {
	cmd := strings.ToLower(strings.TrimSpace(cmdline))
	if cmd == "" {
		return "", false
	}
	has := func(sub string) bool { return strings.Contains(cmd, sub) }
	switch {
	case has("rm -rf") || has("rm -r ") || has("rm -fr"):
		return "rm -rf", true
	case has("sudo"):
		return "sudo", true
	case has("curl") && has("|") && (has("sh") || has("bash")):
		return "curl|sh", true
	case has("ssh"):
		return "ssh", true
	case has("git push --force") || has("git push -f"):
		return "git push --force", true
	case has("|"):
		return "|", true
	case has("&&"):
		return "&&", true
	case has(";"):
		return ";", true
	case has("`"):
		return "`", true
	}
	return "", false
}

// readOnlyBinaries: comandos NOTORIAMENTE read-only (leitura/inspeção/navegação)
// que rodam no Terminal SEM gate de aprovação. Qualquer comando fora desta lista
// é tratado como escrita/execução e passa pelo gate fail-closed (não é uma
// lista exaustiva de permissões — é o limite fail-closed do que é seguro direto).
var readOnlyBinaries = map[string]bool{
	"echo": true, "cat": true, "type": true, "ls": true, "dir": true, "pwd": true,
	"grep": true, "find": true, "head": true, "tail": true, "wc": true, "sort": true,
	"where": true, "which": true, "date": true, "time": true, "whoami": true,
	"ver": true, "help": true, "cls": true, "cd": true,
}

// gitReadOnlyOps: subcomandos do git de leitura pura. Subcomandos ambíguos ou
// destrutivos (branch, tag, remote, add, commit, push, reset, checkout, ...)
// ficam FORA → exigem aprovação (fail-closed).
var gitReadOnlyOps = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "rev-parse": true,
	"describe": true, "ls-files": true, "help": true, "version": true,
}

// goReadOnlyOps: subcomandos do go de leitura/inspeção. build/test/run/mod/install
// são escrita/execução → exigem aprovação.
var goReadOnlyOps = map[string]bool{
	"version": true, "env": true, "list": true, "doc": true,
}

// isReadOnlyCommand reporta se um comando é NOTORIAMENTE read-only e portanto pode
// rodar no Terminal sem gate de aprovação. FAIL-CLOSED: qualquer comando que não
// seja claramente read-only (default) é uma operação de escrita/execução e exige
// aprovação via `requireExecApproval`.
func isReadOnlyCommand(cmdline string) bool {
	cmd := strings.ToLower(strings.TrimSpace(cmdline))
	if cmd == "" {
		return false
	}
	// Redirecionamento escreve/consome disco => write (fail-closed), mesmo com
	// binário read-only (ex.: "echo x > out.txt").
	if strings.ContainsAny(cmd, "><") {
		return false
	}
	bin := firstToken(cmd)
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cmd), bin))
	switch bin {
	case "git", "git.exe":
		return readOnlyOp(rest, gitReadOnlyOps)
	case "go", "go.exe":
		return readOnlyOp(rest, goReadOnlyOps)
	default:
		return readOnlyBinaries[bin]
	}
}

// readOnlyOp verifica se o subcomando de `rest` (primeiro token) é de leitura
// (mapa). Um subcomando ausente/desconhecido => false (fail-closed → aprovação).
func readOnlyOp(rest string, ops map[string]bool) bool {
	return ops[firstToken(rest)]
}

// firstToken devolve o primeiro token de uma linha de comando (delimitado por
// espaço/tab), sem aspas circunstantes. "" quando a linha é vazia.
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	i := strings.IndexAny(s, " \t")
	var tok string
	if i >= 0 {
		tok = s[:i]
	} else {
		tok = s
	}
	return strings.Trim(tok, `"'`)
}
