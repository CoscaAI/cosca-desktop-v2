package projectintel

// CATEGORY: database.go — detecção de bancos/ORMs.
// Evidência por config (prisma/schema.prisma, drizzle.config.*, migrations/),
// por dependência declarada (pg, mysql, sqlite3, mongoose, redis, ...) e por
// imagem de serviço em docker-compose/compose. Nunca conecta a um banco.

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectDatabases returns databases/ORMs evidenced by config files, deps and
// compose service images. It never connects to any database.
func DetectDatabases(root string) []Tech { return detectDatabases(root, inspect(root)) }

func detectDatabases(root string, insp *inspection) []Tech {
	out := []Tech{}
	add := func(name, conf string, ev ...Signal) { out = append(out, tech(name, "database", conf, ev, "")) }

	if st, err := os.Stat(filepath.Join(root, "prisma", "schema.prisma")); err == nil && !st.IsDir() {
		add("Prisma", confHigh, fileSig("prisma/schema.prisma", "prisma/schema.prisma"))
	}
	if m := globAt(root, "drizzle.config.*"); len(m) > 0 {
		add("Drizzle", confHigh, fileSig(sortedFirst(m), sortedFirst(m)))
	}
	if insp.rootDirs["migrations"] {
		add("migrations", confMedium, fileSig("migrations", "migrations"))
	}

	// dependency evidence
	if pkg := readPackage(root); pkg != nil {
		dbDeps := map[string]string{
			"pg":              "PostgreSQL",
			"postgres":        "PostgreSQL",
			"mysql":           "MySQL",
			"mysql2":          "MySQL",
			"better-sqlite3":  "SQLite",
			"sqlite3":         "SQLite",
			"mongoose":        "MongoDB",
			"mongodb":         "MongoDB",
			"redis":           "Redis",
			"ioredis":         "Redis",
			"@prisma/client":  "Prisma",
			"@drizzle-orm/*":  "Drizzle",
			"drizzle-orm":     "Drizzle",
		}
		deps := packageDeps(pkg)
		keys := sortedBoolKeys(deps)
		for _, k := range keys {
			if name, ok := dbDeps[k]; ok {
				if !hasTech(out, name) {
					add(name, confMedium, depSig(k))
				}
			}
		}
	}

	// Compose service image evidence (read-only content scan).
	for _, f := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		content, err := readSmallFile(filepath.Join(root, f))
		if err != nil {
			continue
		}
		lower := strings.ToLower(content)
		imgDB := map[string]string{
			"postgres": "PostgreSQL",
			"mysql":    "MySQL",
			"mariadb":  "MySQL",
			"mongo":    "MongoDB",
			"redis":    "Redis",
			"sqlite":   "SQLite",
		}
		for marker, name := range imgDB {
			if strings.Contains(lower, "image: "+marker) {
				if !hasTech(out, name) {
					add(name, confMedium, contentSig("imagem "+marker+" em "+f))
				}
			}
		}
	}
	return out
}
