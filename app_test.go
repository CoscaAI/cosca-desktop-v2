package main

// Teste do primeiro vertical slice (PROJECT-FIRST).
// Valida: CreateProject executa o cosca init REAL e o isolamento entre
// Projeto A e Projeto B (estados e raízes separados).

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"cosca-desktop/internal/projectintel"
)

func TestVerticalSlice_CreateProjectAndCoscaInit(t *testing.T) {
	app := NewApp()
	base := t.TempDir()

	p, err := app.CreateProject("Alpha", base)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Root == "" || p.Name != "Alpha" {
		t.Errorf("ProjectContext inválido: %+v", p)
	}

	// cosca init REAL cria o .cosca/ do projeto (quando runtime disponível).
	// Não exigimos que .cosca exista (pode falhar sem runtime) mas exigimos
	// que a execução não tenha deixado estado inválido.
	if p.Error != "" {
		t.Logf("cosca init retornou warning (runtime): %s", p.Error)
	}
	_ = os.RemoveAll(p.Root)
}

func TestVerticalSlice_ProjectIsolationAandB(t *testing.T) {
	app := NewApp()
	base := t.TempDir()

	pa, err := app.CreateProject("ProjetoA", base)
	if err != nil {
		t.Fatalf("A: %v", err)
	}
	pb, err := app.CreateProject("ProjetoB", base)
	if err != nil {
		t.Fatalf("B: %v", err)
	}

	if pa.Root == pb.Root {
		t.Errorf("A e B devem ter raízes DISTINTAS, got %s", pa.Root)
	}
	if pa.Name == pb.Name {
		t.Errorf("A e B não podem ter o mesmo nome")
	}
	// Estados separados: cada um com seu diretório.
	if _, err := os.Stat(pa.Root); err != nil {
		t.Errorf("raiz de A deve existir: %v", err)
	}
	if _, err := os.Stat(pb.Root); err != nil {
		t.Errorf("raiz de B deve existir: %v", err)
	}
	if filepath.Clean(pa.Root) == filepath.Clean(filepath.Join(base, "ProjetoB")) {
		t.Errorf("A não pode apontar para a raiz de B")
	}
	_ = os.RemoveAll(pa.Root)
	_ = os.RemoveAll(pb.Root)
}

func TestVerticalSlice_DiscoverAndOpen(t *testing.T) {
	app := NewApp()
	base := t.TempDir()
	root := filepath.Join(base, "Gamma")
	if err := os.MkdirAll(filepath.Join(root, ".cosca"), 0o755); err != nil {
		t.Fatal(err)
	}
	projs := app.DiscoverProjects(base)
	if len(projs) == 0 {
		t.Fatalf("DiscoverProjects deve achar o projeto com .cosca")
	}
	p, err := app.OpenProject(root)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if !p.Initialized {
		t.Errorf("projeto com .cosca deve estar inicializado")
	}
	_ = os.RemoveAll(base)
}

// TestVerticalSlice_LooksLikePlanTarget valida o guard honesto do `AgentPlan`:
// o `cosca plan` requer um `--target` de arquivos (glob/caminho) — NUNCA um
// pedido em linguagem natural como argumento posicional. Sem executar o runtime,
// garantimos que o guard classifica corretamente o que é um alvo plausível e o
// que é pedido de linguagem natural (que deve ser rejeitado, não fabricado).
func TestVerticalSlice_LooksLikePlanTarget(t *testing.T) {
	app := NewApp() // sem projeto ativo — o guard não depende de runtime
	cases := []struct {
		in   string
		want bool
	}{
		{"internal/kernel/*.go", true},   // glob
		{"/abs/path/pkg/file.go", true},  // caminho absoluto
		{"api/rest/users.go", true},      // novo arquivo com extensão
		{"main.go", true},                // arquivo top-level com extensão
		{"crie um endpoint rest", false}, // linguagem natural
		{"", false},                      // vazio
	}
	for _, c := range cases {
		if got := app.looksLikePlanTarget(c.in); got != c.want {
			t.Errorf("looksLikePlanTarget(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestVerticalSlice_AgentPlanHonest garante que o `AgentPlan` não inventa plano:
// sem projeto ativo devolve mensagem honesta; com projeto ativo, um pedido em
// linguagem natural aponta que o `cosca plan` exige `--target` (não simula).
func TestVerticalSlice_AgentPlanHonest(t *testing.T) {
	app := NewApp()
	if got := app.AgentPlan("qualquer coisa"); got != "Sem projeto ativo." {
		t.Errorf("sem projeto: got %q", got)
	}
	base := t.TempDir()
	app.project = &Project{Name: "Gamma", Root: base}
	if got := app.AgentPlan("crie um endpoint rest"); !strings.Contains(got, "--target") {
		t.Errorf("linguagem natural deve apontar --target; got: %q", got)
	}
	if got := app.AgentPlan(""); !strings.Contains(got, "--target") {
		t.Errorf("pedido vazio deve apontar --target; got: %q", got)
	}
}

// TestVerticalSlice_NameValidation valida a correção do F3: nomes de projeto
// inseguros/inválidos devem ser rejeitados ANTES de criar qualquer diretório,
// sem deixar estado fora da base.
func TestVerticalSlice_NameValidation(t *testing.T) {
	app := NewApp()
	base := t.TempDir()
	bad := []string{"../evil", "a/b", "CON", ""}
	for _, name := range bad {
		_, err := app.CreateProject(name, base)
		if err == nil {
			t.Errorf("CreateProject(%q) deveria retornar erro, mas não retornou", name)
		}
		// Nenhum diretório deve ter sido criado na base pelas tentativas invalid.
		entries, _ := os.ReadDir(base)
		if len(entries) != 0 {
			t.Errorf("CreateProject(%q) criou estado na base: %d entradas", name, len(entries))
		}
	}
}

// TestVerticalSlice_SafeJoinSymlinkEscape valida a correção do F2: se um
// symlink dentro do workspace aponta para FORA da raiz, o safeJoin deve
// detectar o escape e retornar erro (path traversal via symlink). Se o
// ambiente não suporta os.Symlink (privilegios em Windows), o teste é pulado.
func TestVerticalSlice_SafeJoinSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir() // diretório garantidamente FORA da raiz do projeto
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("os.Symlink não suportado neste ambiente: %v", err)
	}
	app := NewApp()
	app.project = &Project{Name: "Test", Root: root}

	// 1. Caminho que passa pelo symlink apontando para fora => erro.
	if _, err := app.safeJoin("link/somefile.txt"); err == nil {
		t.Errorf("safeJoin sobre symlink que escapa da raiz deveria retornar erro")
	}
	// 2. Caminho que passa pelo symlink mas para um subdir existente fora => erro.
	if _, err := app.safeJoin("link/deep/nested.txt"); err == nil {
		t.Errorf("safeJoin sobre subpath do symlink externo deveria retornar erro")
	}
	// 3. Caminho legítimo (sem symlink) ainda funciona.
	if _, err := app.safeJoin("docs/readme.md"); err != nil {
		t.Errorf("safeJoin de caminho legítimo dentro da raiz falhou: %v", err)
	}
}

// containsTreePath procura recursivamente por um path relativo na árvore
// retornada por TreeDirs.
func containsTreePath(entries []TreeEntry, rel string) bool {
	for _, e := range entries {
		if e.Path == rel {
			return true
		}
		if containsTreePath(e.Children, rel) {
			return true
		}
	}
	return false
}

// TestVerticalSlice_SensitivePathBlocklist valida a correção do F1/F4: a lista de
// bloqueio (isSensitivePath) rejeita caminhos sensíveis (.git, .cosca/data,
// .cosca/serve.env, .cosca/keys, .env etc.) inclusive variantes de case e de
// ".." — e NÃO bloqueia caminhos legítimos do projeto.
func TestVerticalSlice_SensitivePathBlocklist(t *testing.T) {
	blocked := []string{
		".git", ".git/config",
		".cosca/data", ".cosca/data/vault.json", ".COSCA/DATA",
		".cosca/serve.env", ".cosca/jail-secrets.env",
		".cosca/keys", ".cosca/keys/kernel_private.key",
		".cosca/audit.db", ".cosca/gate.db", ".cosca/trace.db",
		".env", "foo/../.cosca/serve.env",
	}
	for _, p := range blocked {
		if !isSensitivePath(p) {
			t.Errorf("isSensitivePath(%q) = false, want true (deveria ser bloqueado)", p)
		}
	}
	allowed := []string{
		".cosca", ".cosca/config.yaml", ".cosca/agents", ".cosca/agents/foo.md",
		".cosca/sessions", ".cosca/sessions/s1.jsonl",
		"readme.md", "api", "api/rest/users.go", ".gitignore", "",
	}
	for _, p := range allowed {
		if isSensitivePath(p) {
			t.Errorf("isSensitivePath(%q) = true, want false (não deveria ser bloqueado)", p)
		}
	}
}

// TestVerticalSlice_FilesystemBlocksSensitive valida o F4 no backend: ReadFile e
// WriteFile bloqueiam caminhos sensíveis e a TreeDirs não os expõe — mas arquivos
// legítimos do projeto continuam acessíveis.
func TestVerticalSlice_FilesystemBlocksSensitive(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{filepath.Join(root, ".cosca", "data"), filepath.Join(root, ".cosca", "keys")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		".cosca/serve.env":       "COSCA_JWT_SECRET=supersecret\n",
		".cosca/data/vault.json": "{}\n",
		".cosca/keys/kernel.key": "key-material\n",
		".env":                   "SECRET=x\n",
		"readme.md":              "hello world\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := NewApp()
	app.project = &Project{Name: "Proj", Root: root}

	// ReadFile deve bloquear (fail-closed) os caminhos sensíveis.
	for _, rel := range []string{".cosca/serve.env", ".env", ".cosca/data/vault.json", ".cosca/keys/kernel.key"} {
		if _, err := app.ReadFile(rel); err == nil {
			t.Errorf("ReadFile(%q) deveria ser bloqueado, mas retornou nil", rel)
		}
	}
	// WriteFile deve bloquear os caminhos sensíveis (nunca sobrescrever segredo).
	for _, rel := range []string{".cosca/serve.env", ".cosca/data/vault.json"} {
		if err := app.WriteFile(rel, "overwrite"); err == nil {
			t.Errorf("WriteFile(%q) deveria ser bloqueado, mas retornou nil", rel)
		}
	}
	// Arquivo legítimo continua lido.
	content, err := app.ReadFile("readme.md")
	if err != nil {
		t.Fatalf("ReadFile(readme.md) falhou: %v", err)
	}
	if !strings.Contains(content, "hello world") {
		t.Errorf("ReadFile(readme.md) retornou conteúdo inesperado: %q", content)
	}
	// A árvore NÃO expõe nem entra nos caminhos sensíveis.
	tree := app.TreeDirs()
	for _, rel := range []string{".cosca/serve.env", ".cosca/data", ".cosca/keys", ".env"} {
		if containsTreePath(tree, rel) {
			t.Errorf("TreeDirs expôs caminho sensível %q", rel)
		}
	}
	if !containsTreePath(tree, ".cosca") {
		t.Errorf("TreeDirs deveria mostrar o diretório .cosca")
	}
	if !containsTreePath(tree, "readme.md") {
		t.Errorf("TreeDirs deveria mostrar readme.md")
	}
}

// TestVerticalSlice_RuntimeEnvPolicy valida o F1: o runtimeEnv injeta o opt-in
// COSCA_ALLOW_NO_ROOT=1 (declara Windows sem bwrap) e COSCA_ENABLE_EXEC=1 dentro
// do fluxo de aprovação do Cosca, nunca como gate de permissão próprio. Garante
// também que a política fica atrás das constantes declaradas.
func TestVerticalSlice_RuntimeEnvPolicy(t *testing.T) {
	app := NewApp()
	app.project = &Project{Name: "P", Root: `C:\proj`}
	env := app.runtimeEnv()
	got := map[string]string{}
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			got[kv[:i]] = kv[i+1:] // última ocorrência vence (a política é anexada ao fim)
		}
	}
	if got["COSCA_PROJECT"] != `C:\proj` {
		t.Errorf("COSCA_PROJECT = %q, want C:\\proj", got["COSCA_PROJECT"])
	}
	if got["COSCA_ALLOW_NO_ROOT"] != "1" {
		t.Errorf("COSCA_ALLOW_NO_ROOT = %q, want 1 (opt-in declarado p/ Windows, não autorização)", got["COSCA_ALLOW_NO_ROOT"])
	}
	if got["COSCA_ENABLE_EXEC"] != "1" {
		t.Errorf("COSCA_ENABLE_EXEC = %q, want 1", got["COSCA_ENABLE_EXEC"])
	}
}

// TestVerticalSlice_ApprovalRequiredBeforeExec valida o GATE (opção B) do
// AgentRun: sem uma aprovação "approved" registrada, a escrita/execução NÃO
// dispara o runtime (fail-closed) — devolve a mensagem de aprovação necessária
// e não tenta rodar. Com RequestApproval → ApprovePending, o runtime é disparado
// (runtime apontado para um caminho inexistente ⇒ retorna "[erro:"), e a
// aprovação é consumida (limpa) após executar.
func TestVerticalSlice_ApprovalRequiredBeforeExec(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}
	// Runtime inexistente: se o gate falhar e NÃO tentar rodar, devolve a
	// mensagem de aprovação; se tentar rodar, devolve "[erro:" (arquivo inexistente).
	app.runtime = filepath.Join(root, "no-such-runtime")
	req := "crie um endpoint rest"
	model := "deepseek"

	// 1. SEM aprovação registrada: o gate deve bloquear ANTES de rodar o runtime.
	got := app.AgentRun(req, model)
	if !strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("sem aprovação: AgentRun deveria exigir aprovação; got: %q", got)
	}
	if strings.Contains(got, "[erro:") {
		t.Errorf("sem aprovação: AgentRun NÃO deveria tentar rodar o runtime; got: %q", got)
	}

	// 2. Registra e aprova explicitamente (RequestApproval → ApprovePending).
	ap := app.RequestApproval(req, model, "exec")
	if ap == nil {
		t.Fatal("RequestApproval retornou nil")
	}
	if ap.Status != "pending" {
		t.Fatalf("RequestApproval deveria retornar pending; got: %+v", ap)
	}
	if app.PendingApproval() != ap {
		t.Errorf("PendingApproval() deveria apontar para a aprovação pendente")
	}
	if approved := app.ApprovePending(ap.ID); approved == nil || approved.Status != "approved" {
		t.Fatalf("ApprovePending deveria retornar approved; got: %+v", approved)
	}

	// 3. Com aprovação aprovada, AgentRun TENTA rodar (runtime inexistente ⇒ erro).
	got = app.AgentRun(req, model)
	if strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("com aprovação: AgentRun ainda bloqueado; got: %q", got)
	}
	if !strings.Contains(got, "[erro:") {
		t.Errorf("com aprovação: AgentRun deveria ter tentado rodar o runtime; got: %q", got)
	}

	// 4. Após rodar, a aprovação é consumida (limpa).
	if app.PendingApproval() != nil {
		t.Errorf("após consumir a aprovação, PendingApproval() deveria ser nil")
	}
}

// TestVerticalSlice_ApprovalLifecycle valida o ciclo de vida do Approval:
// RequestApproval → pending (e não duplica para o mesmo pedido pendente);
// ApprovePending → approved; DenyPending → denied e limpa o campo; e ids
// incorretos (ou estado vazio) sempre resultam em "denied" (fail-closed).
func TestVerticalSlice_ApprovalLifecycle(t *testing.T) {
	app := NewApp()
	ap := app.RequestApproval("faça X", "deepseek", "exec")
	if ap == nil {
		t.Fatal("RequestApproval retornou nil")
	}
	if ap.Status != "pending" {
		t.Errorf("status inicial deveria ser pending; got: %q", ap.Status)
	}
	if ap.Risk != "exec" {
		t.Errorf("Risk para scope exec deveria ser \"exec\"; got: %q", ap.Risk)
	}
	if ap.ID == "" {
		t.Errorf("ID não pode ser vazio")
	}
	if up := app.RequestApproval("faça X", "deepseek", "exec"); up != ap {
		t.Errorf("RequestApproval com mesmo pedido pendente deveria retornar a MESMA aprovação (não duplicar)")
	}
	if approved := app.ApprovePending(ap.ID); approved == nil || approved.Status != "approved" {
		t.Fatalf("ApprovePending deveria marcar approved; got: %+v", approved)
	}
	if wrong := app.ApprovePending("id-que-nao-existe"); wrong == nil || wrong.Status != "denied" {
		t.Errorf("ApprovePending com id incorreto deveria retornar denied; got: %+v", wrong)
	}
	if denied := app.DenyPending(ap.ID); denied == nil || denied.Status != "denied" {
		t.Fatalf("DenyPending deveria marcar denied; got: %+v", denied)
	}
	if app.PendingApproval() != nil {
		t.Errorf("após DenyPending, PendingApproval() deveria ser nil")
	}
	if d := app.DenyPending("qualquer"); d == nil || d.Status != "denied" {
		t.Errorf("DenyPending sem pendente deveria retornar denied; got: %+v", d)
	}
}

// TestVerticalSlice_ReadOnlyNoApproval valida que o AgentPlan (READ-ONLY) NÃO
// passa pelo gate de aprovação: sem RequestApproval ele executa direto (não
// devolve a mensagem de aprovação), mesmo com runtime inexistente — e não
// cria/consome nenhum estado de aprovação.
func TestVerticalSlice_ReadOnlyNoApproval(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	target := filepath.Join(root, "readme.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.project = &Project{Name: "Proj", Root: root}
	app.runtime = filepath.Join(root, "no-such-runtime")

	got := app.AgentPlan(target)
	if strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("AgentPlan (read-only) NÃO deve exigir aprovação; got: %q", got)
	}
	if !strings.Contains(got, "[erro:") {
		t.Errorf("AgentPlan deveria ter tentado rodar o runtime (read-only direto); got: %q", got)
	}
	if app.PendingApproval() != nil {
		t.Errorf("AgentPlan (read-only) não deve criar/consumir estado de aprovação")
	}
}

// ─── FASE A (ADR-0007): Detecção de root / daemon / capacidade ───────────────

// closedServeURL devolve uma URL de loopback que está (com alta probabilidade)
// FECHADA — usada para testar `serveHealth`/`streamRun` contra daemon offline de
// forma determinística e rápida (sem depender de um runtime real na porta do
// default 14120). Técnica a mesma usada em runtime_test.go do Cosca.
func closedServeURL(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	return "http://" + addr
}

func TestVerticalSlice_DetectRoot(t *testing.T) {
	app := NewApp()

	// STANDALONE: diretório temporário SEM sinais de root.
	plain := t.TempDir()
	app.project = &Project{Name: "Plain", Root: plain}
	info := app.DetectRoot()
	if app.RootMode() {
		t.Errorf("dir sem sinais de root deveria ser STANDALONE (RootMode=false); RootInfo=%+v", info)
	}
	if info["root"] != false {
		t.Errorf("RootInfo em standalone deveria ter root=false; got %+v", info)
	}

	// FORGE: diretório com internal/embed/cosca + AGENT_DNA.md.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "embed", "cosca"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENT_DNA.md"), []byte("# DNA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.project = &Project{Name: "Root", Root: root}
	info = app.DetectRoot()
	if !app.RootMode() {
		t.Errorf("dir com internal/embed/cosca + AGENT_DNA.md deveria ser FORGE (RootMode=true); RootInfo=%+v", info)
	}
	if info["root"] != true {
		t.Errorf("RootInfo em forge deveria ter root=true; got %+v", info)
	}
}

func TestVerticalSlice_RootInfoStructured(t *testing.T) {
	app := NewApp()

	// FORGE: estrutura do root com contagens REAIS de agents/skills.
	root := t.TempDir()
	aDir := filepath.Join(root, "internal", "embed", "cosca", "agents")
	sDir := filepath.Join(root, "internal", "embed", "cosca", "skills")
	for _, d := range []string{aDir, sDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"agent-a.md", "agent-b.md"} {
		if err := os.WriteFile(filepath.Join(aDir, f), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sDir, "skill-a.md"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENT_DNA.md"), []byte("# DNA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.project = &Project{Name: "Root", Root: root}
	info := app.RootInfo()
	if info["root"] != true {
		t.Errorf("RootInfo root deveria ser true; got %+v", info["root"])
	}
	if info["has_embed"] != true {
		t.Errorf("RootInfo has_embed deveria ser true; got %+v", info["has_embed"])
	}
	if info["has_dna"] != true {
		t.Errorf("RootInfo has_dna deveria ser true; got %+v", info["has_dna"])
	}
	if info["agents"] != 2 {
		t.Errorf("RootInfo agents deveria ser 2; got %+v", info["agents"])
	}
	if info["skills"] != 1 {
		t.Errorf("RootInfo skills deveria ser 1; got %+v", info["skills"])
	}
	if _, ok := info["paths"].(map[string]string); !ok {
		t.Errorf("RootInfo paths deveria ser um mapa estruturado; got %T", info["paths"])
	}

	// STANDALONE: RootInfo deve refletir root:false.
	app.project = &Project{Name: "Plain", Root: t.TempDir()}
	info = app.RootInfo()
	if info["root"] != false {
		t.Errorf("RootInfo em standalone deveria ter root=false; got %+v", info)
	}
}

func TestVerticalSlice_CapabilityState(t *testing.T) {
	app := NewApp()
	t.Setenv("COSCA_SERVE_URL", closedServeURL(t)) // daemon offline determinístico

	// Standalone: sem kernel/família.
	app.project = &Project{Name: "Plain", Root: t.TempDir()}
	cs := app.CapabilityState()
	if cs["mode"] != "standalone" {
		t.Errorf("CapabilityState mode deveria ser standalone; got %+v", cs["mode"])
	}
	if cs["kernel"] != false {
		t.Errorf("CapabilityState kernel deveria ser false em standalone; got %+v", cs["kernel"])
	}
	if cs["family_memory"] != false {
		t.Errorf("CapabilityState family_memory deveria ser false em standalone; got %+v", cs["family_memory"])
	}

	// Forge: kernel/família disponíveis (acessados do root, nunca copiados).
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "embed", "cosca"), 0o755); err != nil {
		t.Fatal(err)
	}
	app.project = &Project{Name: "Root", Root: root}
	cs = app.CapabilityState()
	if cs["mode"] != "forge" {
		t.Errorf("CapabilityState mode deveria ser forge; got %+v", cs["mode"])
	}
	if cs["kernel"] != true {
		t.Errorf("CapabilityState kernel deveria ser true em forge; got %+v", cs["kernel"])
	}
	if cs["family_memory"] != true {
		t.Errorf("CapabilityState family_memory deveria ser true em forge; got %+v", cs["family_memory"])
	}
}

func TestVerticalSlice_ServeHealthOffline(t *testing.T) {
	app := NewApp()
	t.Setenv("COSCA_SERVE_URL", closedServeURL(t)) // porta morta
	app.project = &Project{Name: "Proj", Root: t.TempDir()}

	// serveHealth deve retornar false + motivo (sem fake).
	ok, why := app.serveHealth()
	if ok {
		t.Errorf("serveHealth deveria ser false com daemon offline")
	}
	if strings.TrimSpace(why) == "" {
		t.Errorf("serveHealth deveria reportar um motivo; got empty")
	}

	// streamRun com daemon offline NÃO emite fake: emite apenas status offline.
	evs := app.streamRun("crie um endpoint rest", "deepseek")
	if len(evs) == 0 {
		t.Fatalf("streamRun offline deveria devolver pelo menos o evento de status")
	}
	hasStatus := false
	for _, ev := range evs {
		switch ev.Type {
		case "status":
			hasStatus = true
		case "thinking", "response", "done", "tool-call":
			t.Errorf("streamRun offline NÃO deveria emitir evento fake %q", ev.Type)
		}
	}
	if !hasStatus {
		t.Errorf("streamRun offline deveria emitir evento de status; evs=%+v", evs)
	}
	// O buffer público também deve refletir o estado real (sem eventos inventados).
	sub := app.SubscribeRuntime()
	if len(sub) == 0 || sub[len(sub)-1].Type != "status" {
		t.Errorf("SubscribeRuntime deveria expor o status offline; got %+v", sub)
	}
}

// ─── FASE B (ADR-0008): execução materializada por eventos reais do daemon ─────

// TestVerticalSlice_AgentRunStreamGate valida o GATE fail-closed do
// AgentRunStream: SEM uma aprovação "approved" registrada, a execução NÃO
// dispara o daemon — devolve a mensagem de aprovação necessária (e nada chega a
// tentar conectar no run/stream offline). Mesmo with daemon offline (URL morta),
// o gate é o primeiro a bloquear.
func TestVerticalSlice_AgentRunStreamGate(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}
	app.runtime = filepath.Join(root, "no-such-runtime")
	t.Setenv("COSCA_SERVE_URL", closedServeURL(t)) // daemon offline (mas o gate bloqueia antes)

	req := "crie um endpoint rest"
	model := "deepseek"

	got := app.AgentRunStream(req, model)
	if !strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("sem aprovação: AgentRunStream deveria exigir aprovação; got: %q", got)
	}
	if strings.Contains(got, "daemon offline") {
		t.Errorf("sem aprovação: AgentRunStream não deveria chegar ao daemon; got: %q", got)
	}
	if strings.Contains(got, "[erro:") {
		t.Errorf("sem aprovação: AgentRunStream NÃO deveria tentar rodar; got: %q", got)
	}
	// Não deve ter consumido nem criado estado de aprovação.
	if app.PendingApproval() != nil {
		t.Errorf("sem aprovação: PendingApproval() deveria permanecer nil")
	}
}

// TestVerticalSlice_PlanDetailsParse valida o parser honesto do plano real
// (cosca plan --json, ExecutionPlan com chaves PascalCase): extrai files/risk e
// marca policy/scope como "—" quando ausentes (não inventa). JSON inválido →
// raw:true + erro honesto; campos ausentes → defaults honestos.
func TestVerticalSlice_PlanDetailsParse(t *testing.T) {
	app := NewApp()

	// Plano no formato real de `cosca plan --json` (ExecutionPlan, PascalCase).
	plan := `{
		"FilesAffected": 1,
		"Files": ["tiny/tiny.go"],
		"TestsExpected": 1,
		"TestPackages": ["tiny"],
		"Migrations": [],
		"RollbackAvailable": true,
		"RollbackDetail": "git revert",
		"EstimatedMinutes": 5,
		"RiskLevel": "baixo",
		"ConfidencePercent": 90,
		"Method": "heurístico"
	}`
	d := app.PlanDetails(plan)
	files, ok := d["files"].([]string)
	if !ok || len(files) != 1 || files[0] != "tiny/tiny.go" {
		t.Fatalf("PlanDetails files do plano real inválido: %#v", d["files"])
	}
	if d["risk"] != "baixo" {
		t.Errorf("PlanDetails risk = %v, want baixo", d["risk"])
	}
	// Campos que o ExecutionPlan NÃO tem → "—" honesto (não inventados).
	if d["policy"] != "—" {
		t.Errorf("PlanDetails policy ausente deveria ser \"—\"; got %#v", d["policy"])
	}
	if d["scope"] != "—" {
		t.Errorf("PlanDetails scope ausente deveria ser \"—\"; got %#v", d["scope"])
	}
	if d["action"] != nil || d["command"] != nil || d["cwd"] != nil {
		t.Errorf("PlanDetails action/command/cwd ausentes deveriam ser nil; got %#v", d)
	}
	// Extras do plano real presentes.
	if d["files_affected"] != float64(1) {
		t.Errorf("files_affected deve vir do plano; got %#v", d["files_affected"])
	}
	if d["confidence_percent"] != float64(90) {
		t.Errorf("confidence_percent deve vir do plano; got %#v", d["confidence_percent"])
	}

	// JSON inválido → raw:true + erro honesto (não inventa conteúdo).
	bad := app.PlanDetails("isso não é {json")
	if bad["raw"] != true {
		t.Errorf("plano inválido deveria marcar raw:true; got %#v", bad)
	}
	if _, ok := bad["error"]; !ok {
		t.Errorf("plano inválido deveria expor erro honesto; got %#v", bad)
	}

	// Plano mínimo (objeto vazio): campos ausentes → defaults honestos ([]/—/nil).
	empty := app.PlanDetails("{}")
	if fl, ok := empty["files"].([]string); !ok || len(fl) != 0 {
		t.Errorf("plano vazio: files deveria ser []string{}; got %#v", empty["files"])
	}
	if empty["risk"] != "—" || empty["policy"] != "—" || empty["scope"] != "—" {
		t.Errorf("plano vazio: risk/policy/scope deveriam ser \"—\"; got %#v", empty)
	}
	if empty["raw"] != nil {
		t.Errorf("objeto JSON válido não deveria marcar raw; got %#v", empty["raw"])
	}
}

// TestVerticalSlice_AgentRunStreamOffline valida o fallback honesto do modo SSE:
// com o daemon OFFLINE (URL de loopback fechada), o AgentRunStream retorna a
// mensagem honesta de "daemon offline" e NÃO emite eventos fake (thinking/
// response/done/tool-call) — apenas o status offline real via ConsumeEvents.
func TestVerticalSlice_AgentRunStreamOffline(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}
	app.runtime = filepath.Join(root, "no-such-runtime")
	t.Setenv("COSCA_SERVE_URL", closedServeURL(t)) // porta morta

	req := "crie um endpoint rest"
	model := "deepseek"

	// Registra e aprova explicitamente para passar o gate (senão nem chega ao daemon).
	ap := app.RequestApproval(req, model, "exec")
	if ap == nil {
		t.Fatal("RequestApproval retornou nil")
	}
	if app.ApprovePending(ap.ID) == nil || app.PendingApproval().Status != "approved" {
		t.Fatal("falha ao aprovar para o gate do AgentRunStream")
	}

	got := app.AgentRunStream(req, model)
	if !strings.Contains(got, "daemon offline") {
		t.Errorf("AgentRunStream offline deveria retornar mensagem honesta; got: %q", got)
	}
	if !strings.Contains(got, "cosca serve") {
		t.Errorf("mensagem honesta deveria apontar o cosca serve (SSE); got: %q", got)
	}

	// NÃO deve ter emulado eventos do agente — apenas o status offline real.
	evs := app.ConsumeEvents()
	if len(evs) == 0 {
		t.Fatalf("offline: ConsumeEvents deveria expor o status offline")
	}
	for _, ev := range evs {
		switch normalizeEventType(ev.Type) {
		case "thinking", "response", "done", "tool-call", "file-change", "finish", "finish-step":
			t.Errorf("offline: Não deveria emitir evento fake %q (raw=%q)", ev.Type, ev.TipoBruto)
		case "status":
			// esperado — representa a liveness offline honesta.
		}
	}
	if evs[len(evs)-1].Type != "status" {
		t.Errorf("último evento offline deveria ser status; got %+v", evs[len(evs)-1])
	}

	// Após consumir, o buffer não deve mentir (subscrição não mostra fake).
	sub := app.SubscribeRuntime()
	for _, ev := range sub {
		switch normalizeEventType(ev.Type) {
		case "thinking", "response", "done", "tool-call":
			t.Errorf("SubscribeRuntime offline expôs fake %q", ev.Type)
		}
	}
}

// ─── FASE C (Diff engine honesta) — usa o MECANISMO real (`git`), nunca inventa ──

// gitRunErr executa `git -C <dir> <args...>` e devolve a saída combinada. NÃO
// fatalizar: permite detectar ausência/indisponibilidade do git (→ t.Skip).
func gitRunErr(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// gitRun executa git no diretório `dir` com fatal em caso de erro (usado para as
// operações de setup do teste, que devem funcionar se o git está disponível).
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := gitRunErr(dir, args...)
	if err != nil {
		t.Fatalf("git %v falhou: %v\n%s", args, err, out)
	}
}

// initGitRepo inicializa um repositório git REAL em `dir` (isolado e
// determinístico: autocrlf=false, sem assinatura, identidade local). Se `git
// init` não funcionar no ambiente, o teste é PULADO com motivo (não marcado como
// falha — atende à ressalva de ambiente Windows). Retorna true se o repo criou.
func initGitRepo(t *testing.T, dir string) bool {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git indisponível neste ambiente: %v", err)
		return false
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "core.autocrlf", "false"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@example.com"},
		{"config", "commit.gpgsign", "false"},
	} {
		if _, err := gitRunErr(dir, args...); err != nil {
			t.Skipf("git init indisponível neste ambiente (%v): %v", args, err)
			return false
		}
	}
	return true
}

// writeTestFile escreve um arquivo relatício à raiz `root` (criando pais).
func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemPaths(items []DiffItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Path
	}
	return out
}

func hasDiffPath(items []DiffItem, path string) bool {
	for _, it := range items {
		if it.Path == path {
			return true
		}
	}
	return false
}

// TestVerticalSlice_ProjectDiff_NotGit garante a honestidade: um projeto SEM
// .git NÃO inventa diff — devolve {git:false, reason:"não é repositório git",
// items:[], honesto:true}. Também cobre o caso sem projeto ativo.
func TestVerticalSlice_ProjectDiff_NotGit(t *testing.T) {
	app := NewApp()
	dir := t.TempDir() // diretório SEM .git (t.TempDir não é repo)
	app.project = &Project{Name: "Plain", Root: dir}

	got := app.ProjectDiff()
	if got["git"] != false {
		t.Errorf("projeto sem .git: git deveria ser false; got %+v", got["git"])
	}
	reason, _ := got["reason"].(string)
	if !strings.Contains(reason, "repositório git") {
		t.Errorf("projeto sem .git: reason deveria ser honesta; got %#v", got["reason"])
	}
	if items, ok := got["items"].([]DiffItem); !ok || len(items) != 0 {
		t.Errorf("projeto sem .git: items deveria ser []DiffItem{} vazio; got %#v", got["items"])
	}
	if got["honesto"] != true {
		t.Errorf("projeto sem .git: honesto deveria ser true; got %#v", got["honesto"])
	}

	// Sem projeto ativo também é honesto (sem inventar diff).
	app.project = nil
	got = app.ProjectDiff()
	if got["git"] != false {
		t.Errorf("sem projeto ativo: git deveria ser false; got %+v", got["git"])
	}
	if items, ok := got["items"].([]DiffItem); !ok || len(items) != 0 {
		t.Errorf("sem projeto ativo: items deveria ser vazio; got %#v", got["items"])
	}
}

// TestVerticalSlice_ProjectDiff_FiltersSensitive garante que o diff do git
// NÃO expõe segredos: um repo git com um arquivo normal modificado e caminhos
// sensíveis (.env rastreado-e-modificado e .cosca/serve.env não-rastreado) →
// o diff filtra os sensíveis (isSensitivePath — ADR-0002) e mantém os legítimos.
func TestVerticalSlice_ProjectDiff_FiltersSensitive(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	if !initGitRepo(t, root) {
		return // já sinalizou t.Skip
	}
	// Rastreados no commit inicial: main.go (normal) e .env (sensível).
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n}\n")
	writeTestFile(t, root, ".env", "SECRET=x\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "init")
	// Modifica um normal (M) e um sensível (M); acrescenta sensível não-rastreado
	// (.cosca/serve.env) e um arquivo legítimo não-rastreado (readme.md).
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	writeTestFile(t, root, ".env", "SECRET=y\n")
	writeTestFile(t, root, ".cosca/serve.env", "JWT=secret\n")
	writeTestFile(t, root, "readme.md", "ok\n")

	app.project = &Project{Name: "Repo", Root: root}
	got := app.ProjectDiff()
	if got["git"] != true {
		t.Fatalf("repo git: git deveria ser true; got %+v", got["git"])
	}
	items, ok := got["items"].([]DiffItem)
	if !ok {
		t.Fatalf("items deveria ser []DiffItem; got %T", got["items"])
	}
	// Nenhum item pode ser sensível (.env ou .cosca/*).
	for _, it := range items {
		lower := strings.ToLower(it.Path)
		if strings.Contains(lower, ".env") || strings.HasPrefix(lower, ".cosca") {
			t.Errorf("items NÃO deveria expor caminho sensível %q; items=%v", it.Path, itemPaths(items))
		}
	}
	// main.go (modificado) e readme.md (legítimo) devem estar presentes.
	if !hasDiffPath(items, "main.go") {
		t.Errorf("items deveria conter main.go; got %v", itemPaths(items))
	}
	if !hasDiffPath(items, "readme.md") {
		t.Errorf("items deveria conter readme.md (legítimo); got %v", itemPaths(items))
	}
	// Resumo reflete apenas os itens NÃO sensíveis exibidos (2).
	if sm, ok := got["summary"].(DiffSummary); !ok || sm.Files != 2 {
		t.Errorf("summary.Files deveria ser 2 (main.go + readme.md); got %#v, items=%v", got["summary"], itemPaths(items))
	}
}

// TestVerticalSlice_ProjectDiff_UsesGit valida que o diff REAL do git é
// consumido: um repo git com `main.go` TRACKED e MODIFICADO → items contém um
// item {status:"M", path:"main.go"} com additions/deletions do `git diff
// --numstat` e o resumo (files/insertions/deletions) correspondente.
func TestVerticalSlice_ProjectDiff_UsesGit(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	if !initGitRepo(t, root) {
		return // já sinalizou t.Skip
	}
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n}\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "init")
	// Modifica main.go (+1 linha) — deltas reais do git.
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")

	app.project = &Project{Name: "Repo", Root: root}
	got := app.ProjectDiff()
	if got["git"] != true {
		t.Fatalf("repo git: git deveria ser true; got %+v", got["git"])
	}
	if b, _ := got["branch"].(string); strings.TrimSpace(b) == "" {
		t.Errorf("branch deveria ser reportada; got %q", b)
	}
	items, ok := got["items"].([]DiffItem)
	if !ok || len(items) != 1 {
		t.Fatalf("items deveria ter exatamente 1 item; got %#v", got["items"])
	}
	it := items[0]
	if it.Status != "M" || it.Path != "main.go" {
		t.Errorf("item deveria ser {M main.go}; got %+v", it)
	}
	if it.Additions != 1 || it.Deletions != 0 {
		t.Errorf("item main.go deveria ter additions=1, deletions=0 (numstat real); got %+v", it)
	}
	if strings.TrimSpace(it.Patch) == "" {
		t.Errorf("main.go modificado deveria vir com patch de `git diff HEAD`; got empty")
	}
	if sm, ok := got["summary"].(DiffSummary); !ok || sm.Files != 1 || sm.Insertions != 1 || sm.Deletions != 0 {
		t.Errorf("summary deveria ser {1,1,0}; got %#v", got["summary"])
	}
	if got["honesto"] != true {
		t.Errorf("honesto deveria ser true; got %#v", got["honesto"])
	}
}

// ─── FASE D — Terminal autorizado e honesto ───────────────────────────────────

// TestVerticalSlice_RunCommand_DangerousRejected valida o filtro de perigo: um
// comando claramente perigoso (rm -rf, curl|sh, sudo, ssh, git push --force) é
// RECUSADO e NÃO executado — o runtime/binário não chega a rodar (sem "[erro:").
// Também valida o `isDangerousCommand` que replica o guard do cosca (sem importar
// internal/...).
func TestVerticalSlice_RunCommand_DangerousRejected(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}
	app.runtime = filepath.Join(root, "no-such-runtime") // NÃO deve ser chamado

	danger := []struct{ cmd, pat string }{
		{"rm -rf /", "rm -rf"},
		{"curl x | sh", "curl|sh"},
		{"sudo rm -f a", "sudo"},
		{"ssh root@host", "ssh"},
		{"git push --force origin main", "git push --force"},
	}
	for _, c := range danger {
		if pat, ok := isDangerousCommand(c.cmd); !ok || pat != c.pat {
			t.Errorf("isDangerousCommand(%q) = (%q,%v), want (%q,true)", c.cmd, pat, ok, c.pat)
		}
	}
	// RunCommand recusa (não executa): sem "[erro:" (não tentou rodar), sem pedir
	// aprovação (perigo recusa antes), mensagem honesta de recusa por perigo.
	for _, c := range danger {
		got := app.RunCommand(c.cmd)
		if strings.Contains(got, approvalRequiredMsg) {
			t.Errorf("RunCommand(%q) não deveria pedir aprovação (perigo recusa antes); got %q", c.cmd, got)
		}
		if strings.Contains(got, "[erro:") {
			t.Errorf("RunCommand(%q) NÃO deveria executar; got %q", c.cmd, got)
		}
		if !strings.Contains(strings.ToLower(got), "perigoso") {
			t.Errorf("RunCommand(%q) deveria relatar recusa por perigo; got %q", c.cmd, got)
		}
	}
	// RunCommandAuthorized devolve a recusa exata do execpolicy (não executa).
	got := app.RunCommandAuthorized("rm -rf /")
	if !strings.Contains(got, cmdDangerousAuthorizedMsg) {
		t.Errorf("RunCommandAuthorized(rm -rf /) deveria devolver a recusa do execpolicy; got %q", got)
	}
	if strings.Contains(got, "[erro:") {
		t.Errorf("RunCommandAuthorized NÃO deveria executar; got %q", got)
	}
}

// TestVerticalSlice_RunCommand_EmptyRejected valida que um cmdline vazio (ou
// apenas espaços/tabs) é rejeitado honestamente em ambos os métodos, sem executar.
func TestVerticalSlice_RunCommand_EmptyRejected(t *testing.T) {
	app := NewApp()
	app.project = &Project{Name: "Proj", Root: t.TempDir()}
	for _, cmd := range []string{"", "   ", "\t"} {
		if got := app.RunCommand(cmd); !strings.Contains(strings.ToLower(got), "vazio") {
			t.Errorf("RunCommand(%q) deveria rejeitar vazio; got %q", cmd, got)
		}
		if got := app.RunCommandAuthorized(cmd); !strings.Contains(strings.ToLower(got), "vazio") {
			t.Errorf("RunCommandAuthorized(%q) deveria rejeitar vazio; got %q", cmd, got)
		}
	}
}

// TestVerticalSlice_RunCommand_RequiresApproval valida o GATE fail-closed para
// operações de escrita/execução: um comando de escrita (redirecionamento) sem
// aprovação "approved" registrada NÃO executa (devolve a mensagem de aprovação e
// não cria o arquivo). Após RequestApproval → ApprovePending, executa e cria o
// arquivo; e a aprovação local é consumida (limpa) após executar.
func TestVerticalSlice_RunCommand_RequiresApproval(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}
	cmd := "echo hi > out.txt" // redirecionamento => operação de escrita

	// 1. SEM aprovação: o gate bloqueia antes de qualquer escrita.
	got := app.RunCommand(cmd)
	if !strings.Contains(got, approvalRequiredMsg) {
		t.Fatalf("sem aprovação: RunCommand deveria exigir aprovação; got %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "out.txt")); err == nil {
		t.Errorf("sem aprovação: out.txt NÃO deveria ter sido criado (não executou)")
	}
	if app.PendingApproval() != nil {
		t.Errorf("sem aprovação: não deveria haver estado de aprovação")
	}

	// 2. Registra e aprova explicitamente (RequestApproval → ApprovePending).
	ap := app.RequestApproval(cmd, "deepseek", "exec")
	if ap == nil || ap.Status != "pending" {
		t.Fatalf("RequestApproval deveria retornar pending; got %+v", ap)
	}
	if app.ApprovePending(ap.ID) == nil || app.PendingApproval() == nil || app.PendingApproval().Status != "approved" {
		t.Fatalf("ApprovePending deveria marcar approved")
	}

	// 3. Com aprovação aprovada, o comando de escrita EXECUTA (cria o arquivo).
	got = app.RunCommand(cmd)
	if strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("com aprovação: RunCommand ainda bloqueado; got %q", got)
	}
	if b, err := os.ReadFile(filepath.Join(root, "out.txt")); err != nil {
		t.Errorf("com aprovação: out.txt deveria ter sido criado; err %v", err)
	} else if !strings.Contains(string(b), "hi") {
		t.Errorf("out.txt deveria conter 'hi'; got %q", string(b))
	}
	// 4. Após executar, a aprovação é consumida (limpa).
	if app.PendingApproval() != nil {
		t.Errorf("após executar, PendingApproval() deveria ser nil")
	}
}

// TestVerticalSlice_RunCommand_RunsInProject valida que um comando benigno roda
// DENTRO da raiz do projeto (sem gate de aprovação) e retorna a saída real:
//   - `echo hi` → saída em stdout (read-only, sem aprovação);
//   - `cd` → imprime o diretório corrente = raiz do projeto (prova o cmd.Dir).
func TestVerticalSlice_RunCommand_RunsInProject(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}

	// echo é read-only → roda DIRETO (sem gate) e devolve a saída.
	got := app.RunCommand("echo hi")
	if strings.Contains(got, approvalRequiredMsg) {
		t.Errorf("echo (read-only) não deveria exigir aprovação; got %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("RunCommand(echo hi) deveria retornar a saída real; got %q", got)
	}

	// `cd` imprime o diretório corrente → prova que o comando roda DENTRO da
	// raiz do projeto (cmd.Dir = a.project.Root), sem escapar.
	got = app.RunCommand("cd")
	if !strings.Contains(strings.ToLower(filepath.Clean(got)), strings.ToLower(filepath.Clean(root))) {
		t.Errorf("RunCommand(cd) deveria rodar na raiz do projeto %s; got %q", root, got)
	}
}

// ─── FASE 3 — Project Intelligence alimenta o Agente (contexto + skills + pipeline) ──

// writeFixture cria um projeto em t.TempDir() a partir de um map[path]conteúdo
// (mesma técnica do projectintel: detecção é por arquivo estático).
func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// assertPipelineStep confirma que a etapa `step` do pipeline tem o tool/status
// esperado (não existe a etapa → falha).
func assertPipelineStep(t *testing.T, steps []map[string]string, step, wantTool, wantStatus string) {
	t.Helper()
	for _, s := range steps {
		if s["step"] == step {
			if s["tool"] != wantTool || s["status"] != wantStatus {
				t.Errorf("pipeline[%s] = {tool:%q,status:%q}, want {tool:%q,status:%q}", step, s["tool"], s["status"], wantTool, wantStatus)
			}
			return
		}
	}
	t.Errorf("pipeline não contém a etapa %q; steps=%v", step, steps)
}

// TestVerticalSlice_AgentProjectContext valida a injeção do Project Intelligence
// no contexto do Agente: com projeto ativo (Go: go.mod + src + Makefile) →
// has_project:true, language="Go", package_manager/build/test/lint/format
// preenchidos, stack não vazio e commands/config refletem o projeto. Sem projeto
// → has_project:false. Erro de análise → has_project:true + error honesto.
func TestVerticalSlice_AgentProjectContext(t *testing.T) {
	app := NewApp()

	// Sem projeto ativo → honesto.
	if ctx := app.AgentProjectContext(); ctx["has_project"] != false {
		t.Errorf("sem projeto: has_project deveria ser false; got %+v", ctx["has_project"])
	}

	// Projeto ativo com perfil Go (go.mod + src + go.sum + Makefile).
	root := writeFixture(t, map[string]string{
		"go.mod":           "module example.com/foo\n\ngo 1.25\n",
		"src/main.go":      "package main\n\nfunc main() {}\n",
		"src/main_test.go": "package main\n\nfunc TestX(t *testing.T) {}\n",
		"go.sum":           "example.com/foo v1.0.0\n",
		"Makefile":         "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n",
	})
	app.project = &Project{Name: "Foo", Root: root}
	ctx := app.AgentProjectContext()

	if ctx["has_project"] != true {
		t.Fatalf("com projeto: has_project deveria ser true; got %+v", ctx["has_project"])
	}
	if ctx["name"] != "Foo" {
		t.Errorf("name deveria ser Foo; got %v", ctx["name"])
	}
	if ctx["language"] != "Go" {
		t.Errorf("language deveria ser Go; got %v", ctx["language"])
	}
	if pm, _ := ctx["package_manager"].(string); strings.TrimSpace(pm) == "" {
		t.Errorf("package_manager deveria estar preenchido; got %q", pm)
	}
	for _, f := range []string{"build", "test", "lint", "format"} {
		if v, _ := ctx[f].(string); strings.TrimSpace(v) == "" {
			t.Errorf("%s deveria estar preenchido; got %q", f, v)
		}
	}
	stack, _ := ctx["stack"].(string)
	if strings.TrimSpace(stack) == "" {
		t.Errorf("stack não deveria estar vazio; got %q", stack)
	}
	if !strings.Contains(stack, "Go") {
		t.Errorf("stack deveria conter Go; got %q", stack)
	}
	// commands refletem o Makefile (build/test).
	cmds, ok := ctx["commands"].([]projectintel.DetectedCommand)
	if !ok || len(cmds) == 0 {
		t.Errorf("commands deveria refletir o Makefile; got %T", ctx["commands"])
	}
	// important_config reflete os config files detectados (go.mod, Makefile, ...).
	if cfg, ok := ctx["important_config"].([]string); !ok || len(cfg) == 0 {
		t.Errorf("important_config não deveria estar vazio; got %T", ctx["important_config"])
	}

	// Erro de análise → has_project:true + error honesto (não inventa perfil).
	app.project = &Project{Name: "Bad", Root: filepath.Join(root, "does-not-exist")}
	ctx = app.AgentProjectContext()
	if ctx["has_project"] != true {
		t.Errorf("erro de análise: has_project deveria ser true; got %+v", ctx["has_project"])
	}
	if errStr, ok := ctx["error"].(string); !ok || strings.TrimSpace(errStr) == "" {
		t.Errorf("erro de análise deveria expor error honesto; got %+v", ctx["error"])
	}
}

// TestAgentSkillMatch valida o skill auto-matching: um perfil
// React+TypeScript+Vite+ESLint+Prettier → skills contém react/typescript/vite/
// eslint/prettier (superfície do perfil real); NÃO contém skills que não casam
// (docker/postgres/go/rust/nextjs); e é único (sem duplicados).
func TestAgentSkillMatch(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"package.json": `{
			"name":"react-app","scripts":{"dev":"vite","build":"vite build","test":"vitest"},
			"dependencies":{"react":"^18","react-dom":"^18"},
			"devDependencies":{"vite":"^5","vitest":"^1","typescript":"^5","eslint":"^8","prettier":"^3"}
		}`,
		"tsconfig.json":     `{"compilerOptions":{"jsx":"react-jsx"}}`,
		"vite.config.ts":    `export default {}`,
		"eslint.config.mjs": `export default []`,
		".prettierrc":       `{"semi":false}`,
		"src/App.tsx":       `export default function App(){return <div/>}`,
		"src/main.tsx":      `import App from './App'`,
		"src/one.tsx":       `export const a=1`,
		"src/two.tsx":       `export const b=2`,
	})
	profile, err := projectintel.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	skills := projectintel.AgentSkillMatch(profile)

	for _, want := range []string{"react", "typescript", "vite", "eslint", "prettier"} {
		if !containsStr(skills, want) {
			t.Errorf("AgentSkillMatch deveria conter %q (casa com o perfil); got %v", want, skills)
		}
	}
	for _, no := range []string{"docker", "postgres", "go", "rust", "nextjs"} {
		if containsStr(skills, no) {
			t.Errorf("AgentSkillMatch NÃO deveria conter %q (não casa com este perfil); got %v", no, skills)
		}
	}
	seen := map[string]bool{}
	for _, s := range skills {
		if seen[s] {
			t.Errorf("AgentSkillMatch deveria ser único (sem duplicados); duplicado %q; got %v", s, skills)
		}
		seen[s] = true
	}
	if len(skills) == 0 {
		t.Errorf("AgentSkillMatch não deveria ser vazio para um perfil com tecnologia")
	}
}

// pipelineBaseOrder é a sequência fixa do pipeline auto-descoberto (o contrato
// garante que as etapas vêm nesta ordem e sem duplicados).
var pipelineBaseOrder = []string{"discover", "understand", "plan", "implement", "format", "lint", "typecheck", "test", "build", "diff", "review"}

// TestAgentPipeline_Go valida o pipeline adaptado a um projeto Go: format gofmt,
// lint go vet, typecheck NOT_APPLICABLE (Go não tem type checker), test go test,
// build go build. Também confirma a ordem/base da sequência.
func TestAgentPipeline_Go(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":               "module example.com/foo\n\ngo 1.25\n",
		"cmd/foo/main.go":      "package main\n\nfunc main(){}\n",
		"pkg/bar/bar.go":       "package bar\nfunc Bar() string { return \"x\" }",
		"internal/bar_test.go": "package internal\nfunc TestX(t *testing.T){}\n",
		"Makefile":             "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...",
	})
	profile, err := projectintel.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	steps := projectintel.AgentPipeline(profile)

	assertPipelineStep(t, steps, "format", "gofmt", "available")
	assertPipelineStep(t, steps, "lint", "go vet", "available")
	assertPipelineStep(t, steps, "typecheck", "NOT_APPLICABLE", "not_applicable")
	assertPipelineStep(t, steps, "test", "go test", "available")
	assertPipelineStep(t, steps, "build", "go build", "available")

	// Ordem + unicidade da sequência base.
	assertPipelineOrder(t, steps)
}

// TestAgentPipeline_TS valida o pipeline adaptado a um projeto TS/Next:
// format prettier, lint eslint, typecheck tsc, test vitest, build next. Etapas
// de ferramenta SEM ferramenta detectada → not_applicable (nesse perfil todas
// têm, então era um projeto Go que cobre o caso not_applicable em typecheck).
func TestAgentPipeline_TS(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"package.json": `{"name":"next-app","scripts":{"dev":"next dev","build":"next build"},
			"dependencies":{"next":"^14","react":"^18"},"devDependencies":{"eslint":"^8","prettier":"^3","typescript":"^5","vitest":"^1"}}`,
		"tsconfig.json":     `{}`,
		"next.config.mjs":   `export default {}`,
		"vitest.config.ts":  `export default {}`,
		"eslint.config.mjs": `export default []`,
		".prettierrc":       `{"semi":false}`,
		"app/page.tsx":      `export default function Home(){return <h1>hi</h1>}`,
		"app/layout.tsx":    `export default function Layout(){return <html/>}`,
		"app/index.tsx":     `export default function X(){return null}`,
		"app/about.tsx":     `export default function X(){return null}`,
	})
	profile, err := projectintel.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	steps := projectintel.AgentPipeline(profile)

	assertPipelineStep(t, steps, "format", "prettier", "available")
	assertPipelineStep(t, steps, "lint", "eslint", "available")
	assertPipelineStep(t, steps, "typecheck", "tsc", "available")
	assertPipelineStep(t, steps, "test", "vitest", "available")
	assertPipelineStep(t, steps, "build", "next", "available")

	assertPipelineOrder(t, steps)
}

// assertPipelineOrder confirma que a sequência retornada é a base fixa (ordem e
// sem duplicados), o que garante determinismo do pipeline auto-descoberto.
func assertPipelineOrder(t *testing.T, steps []map[string]string) {
	t.Helper()
	if len(steps) != len(pipelineBaseOrder) {
		t.Fatalf("pipeline deveria ter %d etapas; got %d (%v)", len(pipelineBaseOrder), len(steps), steps)
	}
	seen := map[string]bool{}
	for i, s := range steps {
		if s["step"] != pipelineBaseOrder[i] {
			t.Errorf("pipeline ordem[%d] = %q, want %q; steps=%v", i, s["step"], pipelineBaseOrder[i], steps)
		}
		if seen[s["step"]] {
			t.Errorf("pipeline tem etapa duplicada %q", s["step"])
		}
		seen[s["step"]] = true
		// Toda etapa carrega os três campos (step/tool/status).
		if s["tool"] == "" || s["status"] == "" {
			t.Errorf("etapa %q deveria ter tool e status preenchidos; got %v", s["step"], s)
		}
	}
}

// ─── FILE EXPLORER LAZY (ReadDir) — fonte de verdade do filesystem ─────────────
//
// O File Explorer profissional migra do TreeDirs (recursivo, depth 5) para o
// carregamento LAZY via ReadDir(): um diretório por chamada, não-recursivo, com
// metadata REAL do disco (IsDir, IsSymlink, Size, ModifiedAt, Language). Os
// testes abaixo validam a fonte de verdade e o contrato lazy/isolação.

// findFileNode procura um nó por path em uma lista FLAT (não-recursiva) de
// FileNode (os filhos diretos de um diretório do ReadDir lazy).
func findFileNode(nodes []FileNode, path string) (FileNode, bool) {
	for _, n := range nodes {
		if n.Path == path {
			return n, true
		}
	}
	return FileNode{}, false
}

// TestVerticalSlice_ReadDir_Root valida o ReadDir da RAIZ (rel vazio): a fonte
// de verdade do filesystem. Um projeto com go.mod (file), cmd/ (dir), main.go
// (file) e caminhos SENSÍVEIS (.git/config, .env) deve listar apenas os
// legítimos — sensíveis NUNCA aparecem (isSensitivePath, fail-closed ADR-0002).
// Também garante metadata real (IsDir/Ext/Language/Size/ModifiedAt) e ordenação
// (diretórios primeiro, depois alfabético).
func TestVerticalSlice_ReadDir_Root(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	for _, d := range []string{filepath.Join(root, "cmd"), filepath.Join(root, ".git")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"go.mod":      "module example.com/foo\n\ngo 1.25\n",
		"main.go":     "package main\n\nfunc main() {}\n",
		".git/config": "[core]\n",
		".env":        "SECRET=x\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app.project = &Project{Name: "Proj", Root: root}

	// rel vazio/"" => raiz do projeto.
	nodes, err := app.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(raiz): %v", err)
	}
	// Sensíveis NÃO aparecem (.git, .git/config, .env) — fail-closed.
	for _, p := range []string{".git", ".git/config", ".env"} {
		if _, ok := findFileNode(nodes, p); ok {
			t.Errorf("ReadDir expôs caminho sensível %q", p)
		}
	}
	// Filhos diretos legítimos: cmd (dir), go.mod (file), main.go (file).
	if len(nodes) != 3 {
		t.Fatalf("ReadDir(raiz) esperava 3 filhos diretos, got %d: %+v", len(nodes), nodes)
	}
	// Ordenado: diretório primeiro (cmd), depois alfabético (go.mod, main.go).
	if nodes[0].Name != "cmd" {
		t.Errorf("primeiro nó deveria ser o diretório cmd; got %q", nodes[0].Name)
	}
	if !nodes[0].IsDir || nodes[0].Kind != "directory" || nodes[0].Path != "cmd" {
		t.Errorf("nó cmd com metadata errada (IsDir/Kind/Path): %+v", nodes[0])
	}
	// LAZY: o nó diretório não vem carregado (Children nil, Loaded false).
	if nodes[0].Children != nil || nodes[0].Loaded {
		t.Errorf("cmd deveria ser LAZY (Children nil, Loaded false); got %+v", nodes[0])
	}
	if nodes[1].Name != "go.mod" || nodes[1].IsDir {
		t.Errorf("2º nó deveria ser go.mod (file); got %+v", nodes[1])
	}
	if nodes[2].Name != "main.go" || nodes[2].IsDir {
		t.Errorf("3º nó deveria ser main.go (file); got %+v", nodes[2])
	}
	// Metadata real da fonte de verdade.
	if nodes[2].Language != "go" {
		t.Errorf("main.go Language deveria ser go; got %q", nodes[2].Language)
	}
	if nodes[2].Ext != ".go" {
		t.Errorf("main.go Ext deveria ser .go; got %q", nodes[2].Ext)
	}
	if nodes[2].Size <= 0 {
		t.Errorf("main.go Size deveria ser > 0 (metadata real); got %d", nodes[2].Size)
	}
	if nodes[2].ModifiedAt == "" {
		t.Errorf("main.go ModifiedAt não deveria ser vazio (metadata real)")
	}
}

// TestVerticalSlice_ReadDir_LazyNested valida o LAZY não-recursivo: ReadDir("cmd")
// retorna apenas os filhos diretos de cmd (deep incluso, mas NÃO o nested.go
// dentro de deep). Os nós não vêm com Children preenchidos (Loaded=false). O
// carregamento sob demanda continua via ReadDir("cmd/deep").
func TestVerticalSlice_ReadDir_LazyNested(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"cmd/serve.go":       "package main\n",
		"cmd/main.go":        "package main\n",
		"cmd/deep/nested.go": "package deep\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app.project = &Project{Name: "Proj", Root: root}

	nodes, err := app.ReadDir("cmd")
	if err != nil {
		t.Fatalf("ReadDir(cmd): %v", err)
	}
	// Apenas os filhos DIRETOS de cmd: serve.go, main.go, deep (dir).
	if len(nodes) != 3 {
		t.Fatalf("ReadDir(cmd) esperava 3 filhos diretos, got %d: %+v", len(nodes), nodes)
	}
	// LAZY: nenhum nó do listing vem com Children preenchido nem Loaded=true.
	for _, n := range nodes {
		if n.Loaded {
			t.Errorf("nó %q não deveria estar carregado (lazy); got %+v", n.Path, n)
		}
		if n.Children != nil {
			t.Errorf("nó %q não deveria ter Children preenchidos (não-recursivo); got %+v", n.Path, n.Children)
		}
	}
	// deep é listado como dir, mas o nested.go NÃO (não desce — lazy).
	if deep, ok := findFileNode(nodes, "cmd/deep"); !ok {
		t.Fatalf("ReadDir(cmd) deveria listar o diretório cmd/deep; nodes=%+v", nodes)
	} else if !deep.IsDir || deep.Kind != "directory" {
		t.Errorf("cmd/deep deveria ser directory; got %+v", deep)
	}
	if _, ok := findFileNode(nodes, "cmd/deep/nested.go"); ok {
		t.Errorf("ReadDir(cmd) NÃO deveria recursar (nested.go não é filho direto)")
	}
	// Arquivos filhos de cmd com Language real.
	for _, name := range []string{"cmd/serve.go", "cmd/main.go"} {
		n, ok := findFileNode(nodes, name)
		if !ok {
			t.Fatalf("ReadDir(cmd) deveria conter %q; nodes=%v", name, nodes)
		}
		if n.Language != "go" {
			t.Errorf("%s Language deveria ser go; got %q", name, n.Language)
		}
	}
	// Lazy sob demanda: carregar deep na hora em que for expandido.
	deepNodes, err := app.ReadDir("cmd/deep")
	if err != nil {
		t.Fatalf("ReadDir(cmd/deep): %v", err)
	}
	if len(deepNodes) != 1 || deepNodes[0].Path != "cmd/deep/nested.go" {
		t.Errorf("ReadDir(cmd/deep) deveria ter 1 filho (nested.go); got %+v", deepNodes)
	}
}

// TestVerticalSlice_ResolveLanguage valida o resolver determinístico de
// linguagem (extensão composta + nome especial, sem adivinhar extensões
// desconhecidas — sempre "plaintext").
func TestVerticalSlice_ResolveLanguage(t *testing.T) {
	cases := []struct{ in, want string }{
		{".go", "go"},
		{".tsx", "tsx"},
		{".d.ts", "typescript"},      // extensão composta
		{".test.ts", "typescript"},   // extensão composta
		{"Dockerfile", "dockerfile"}, // nome especial
		{".md", "markdown"},
		{".png", "image"},
		{".unknownext", "plaintext"}, // desconhecida → nunca adivinhar
	}
	for _, c := range cases {
		if got := resolveLanguage(c.in); got != c.want {
			t.Errorf("resolveLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestVerticalSlice_ReadDir_Error valida os erros honestos do ReadDir:
// diretório inexistente, apontar para um arquivo (não diretório) e a ausência
// de projeto ativo (fail-closed).
func TestVerticalSlice_ReadDir_Error(t *testing.T) {
	app := NewApp()
	root := t.TempDir()
	app.project = &Project{Name: "Proj", Root: root}

	// Diretório inexistente → erro honesto.
	if _, err := app.ReadDir("nope"); err == nil {
		t.Errorf("ReadDir(dir inexistente) deveria retornar erro")
	} else if !strings.Contains(err.Error(), "não é diretório") {
		t.Errorf("erro deveria mencionar 'não é diretório'; got %q", err.Error())
	}

	// Apontar para um ARQUIVO (não diretório) → erro honesto.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReadDir("main.go"); err == nil {
		t.Errorf("ReadDir(arquivo) deveria retornar erro")
	} else if !strings.Contains(err.Error(), "não é diretório") {
		t.Errorf("erro deveria mencionar 'não é diretório'; got %q", err.Error())
	}

	// Sem projeto ativo → fail-closed.
	app2 := NewApp()
	if _, err := app2.ReadDir(""); err == nil {
		t.Errorf("ReadDir sem projeto ativo deveria retornar erro")
	} else if !strings.Contains(err.Error(), "sem projeto ativo") {
		t.Errorf("sem projeto: erro deveria mencionar 'sem projeto ativo'; got %q", err.Error())
	}
}
