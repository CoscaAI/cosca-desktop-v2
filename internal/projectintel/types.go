// Package projectintel implements the Project Intelligence CORE for the
// cosca-desktop (Wails) application.
//
// It is a READ-ONLY, cheap, deterministic, incremental and NON-DESTRUCTIVE
// layer that inspects a project root by FILE EVIDENCE only and produces a
// ProjectProfile. It never executes commands, never installs anything, never
// connects to a database, and never writes to the project. It does not touch
// the Cosca kernel / institutional memory / legacy (see ADR-0007: an
// independent Desktop has agents + skills + project intelligence without
// kernel/memory/family/blockchain; only the Forge (root) consumes official
// infrastructure). Project Intelligence is NOT kernel nor COSCA memory.
package projectintel

import "time"

// Confidence is the strength of a detection. It is intentionally a small,
// string-typed vocabulary so the model stays extensible (no giant enum).
type Confidence string

// Confidence levels.
const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Signal is a single, specific piece of evidence that supports a detection.
// Kind: "file" | "dependency" | "content" | "count" | "command".
type Signal struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	File  string `json:"file,omitempty"`
}

// Tech is a detected technology (language, framework, tool, ...).
// Category groups it ("language", "frontend", "backend", "buildtool",
// "testtool", "formatter", "linter", "typechecker", "runtime", "container",
// "infrastructure", "database", "monorepo", "ci", "documentation").
// Confidence is one of high/medium/low. The model is extensible by adding new
// Tech entries, never by growing an enum.
type Tech struct {
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Confidence string   `json:"confidence"`
	Evidence   []Signal `json:"evidence,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// DetectedCommand is a command discovered in a project manifest (package.json
// scripts, Makefile, Justfile, Taskfile, ...). Only real commands are reported.
type DetectedCommand struct {
	Name   string `json:"name"`
	Cmd    string `json:"cmd"`
	Source string `json:"source"`
}

// ProjectProfile is the result of a Project Intelligence analysis of a root.
type ProjectProfile struct {
	Root      string `json:"root"`
	Name      string `json:"name"`
	// ProjectTypes groups the root into one or more coarse shapes:
	// "frontend","backend","monorepo","cli","library","service","docs-only","empty".
	ProjectTypes    []string `json:"project_types"`
	Languages       []Tech   `json:"languages"`
	Frameworks      []Tech   `json:"frameworks"`
	PackageManagers []Tech   `json:"package_managers"`
	BuildTools      []Tech   `json:"build_tools"`
	TestTools       []Tech   `json:"test_tools"`
	Formatters      []Tech   `json:"formatters"`
	Linters         []Tech   `json:"linters"`
	TypeCheckers    []Tech   `json:"type_checkers"`
	Runtime         []Tech   `json:"runtime"`
	Container       []Tech   `json:"container"`
	Infrastructure  []Tech   `json:"infrastructure"`
	Database        []Tech   `json:"database"`
	Monorepo        []Tech   `json:"monorepo"`
	CI              []Tech   `json:"ci"`
	Documentation   []Tech   `json:"documentation"`
	Repository      []Tech   `json:"repository"`

	ConfigFiles []string           `json:"config_files"`
	Commands    []DetectedCommand  `json:"commands"`
	Capabilities map[string]string `json:"capabilities"`
	Evidence    []Signal           `json:"evidence"`
	DetectedAt  string             `json:"detected_at"`
}

// TimeNow returns the current time in UTC RFC3339. Separated so tests can rely
// on DetectedAt being a non-empty, well-formed timestamp without depending on
// clock order.
var TimeNow = func() string { return time.Now().UTC().Format(time.RFC3339) }
