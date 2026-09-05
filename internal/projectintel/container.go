package projectintel

// CATEGORY: container.go — detecção de container (Docker/Compose/devcontainer)
// e de infraestrutura (IaC/kubernetes: Terraform, Helm, Kustomize).

import "fmt"

// ---------------------------------------------------------------------------
// Container
// ---------------------------------------------------------------------------

// DetectContainer returns container tooling evidenced by Dockerfile/compose.
func DetectContainer(root string) []Tech { return detectContainer(root, inspect(root)) }

func detectContainer(root string, insp *inspection) []Tech {
	out := []Tech{}
	if m := globAt(root, "Dockerfile*"); len(m) > 0 {
		out = append(out, tech("Docker", "container", confHigh, []Signal{fileSig(sortedFirst(m), sortedFirst(m))}, "Dockerfile"))
	}
	for _, f := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml", "docker-compose.json"} {
		if insp.rootFileSet[f] {
			out = append(out, tech("Compose", "container", confHigh, []Signal{fileSig(f, f)}, "docker compose"))
		}
	}
	if insp.rootDirs[".devcontainer"] {
		out = append(out, tech("devcontainer", "container", confMedium, []Signal{fileSig(".devcontainer", ".devcontainer")}, "devcontainer"))
	}
	return out
}

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

// DetectInfrastructure returns IaC/kubernetes tooling evidenced by files/counts.
func DetectInfrastructure(root string) []Tech { return detectInfrastructure(root, inspect(root)) }

func detectInfrastructure(root string, insp *inspection) []Tech {
	out := []Tech{}
	if insp.rootFileSet["terraform.lock.hcl"] || insp.tfCount > 0 {
		conf := confMedium
		ev := []Signal{countSig(fmt.Sprintf("%d .tf file(s)", insp.tfCount))}
		if insp.rootFileSet["terraform.lock.hcl"] {
			conf = confHigh
			ev = append(ev, fileSig("terraform.lock.hcl", "terraform.lock.hcl"))
		}
		out = append(out, tech("Terraform", "infrastructure", conf, ev, ""))
	}
	if insp.rootFileSet["Chart.yaml"] {
		out = append(out, tech("Helm", "infrastructure", confHigh, []Signal{fileSig("Chart.yaml", "Chart.yaml")}, ""))
	}
	if insp.rootFileSet["kustomization.yaml"] || insp.rootFileSet["kustomization.yml"] {
		name := "kustomization.yaml"
		conf := confHigh
		if !insp.rootFileSet["kustomization.yaml"] {
			name = "kustomization.yml"
		}
		out = append(out, tech("Kustomize", "infrastructure", conf, []Signal{fileSig(name, name)}, ""))
	}
	return out
}
