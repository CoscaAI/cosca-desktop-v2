package projectintel

// CATEGORY: repository.go — detecção de controle de versão (Git).
//
// O `.git` é tratado como um diretório a NÃO descer (excludedDirs confere a
// varredura barata), mas a Project Intelligence deve REPORTAR Git como
// tecnologia. Este arquivo expõe detectRepository/DetectRepository, que
// procuram `.git` (dir), `.gitignore` e `.gitattributes` e devolvem um
// `[]Tech` com a entrada "Git".
//
// NOTA (decisão de segurança): por enquanto esta função NÃO é acoplada no
// Analyze — permanece pronta para ser ligada em um próximo PR. Isso evita
// qualquer mudança no contrato público (ProjectProfile) e nos testes de
// fixtures, que continuam passando.

// DetectRepository returns version-control evidence (Git) for root.
func DetectRepository(root string) []Tech { return detectRepository(root, inspect(root)) }

func detectRepository(root string, insp *inspection) []Tech {
	out := []Tech{}
	if insp.rootDirs[".git"] || insp.rootFileSet[".gitignore"] || insp.rootFileSet[".gitattributes"] {
		conf := confMedium
		ev := []Signal{}
		if insp.rootDirs[".git"] {
			conf = confHigh
			ev = append(ev, fileSig(".git", ".git"))
		}
		if insp.rootFileSet[".gitignore"] {
			ev = append(ev, fileSig(".gitignore", ".gitignore"))
		}
		if insp.rootFileSet[".gitattributes"] {
			ev = append(ev, fileSig(".gitattributes", ".gitattributes"))
		}
		out = append(out, tech("Git", "vcs", conf, ev, ""))
	}
	return out
}
