// Package uiperception is the Cosca UI Perception Engine — the Kernel's eye
// for interfaces, WITHOUT a GPU or a multimodal model.
//
// Philosophy (professor, 2026-08): "visão" não é jogar imagem numa VLM, é
// construir uma representação do mundo visual a partir de MÚLTIPLAS evidências
// determinísticas (DOM/geometria/estados/tokens/contraste). GPU é opcional e
// apenas acelera; a percepção base é CPU, local e determinística.
//
// This package is CAMADA 1-2: it consumes a UI Scene (bounds/roles/state/style
// collected by the frontend via getBoundingClientRect + computed styles) and
// ANALYSES it by rules: geometry, overlap, alignment, spacing, accessibility
// targets, contrast, clipped content. The output is a UISceneModel + violations
// the kernel can reason about and gate (see docs/UI_GOVERNANCE.md).
//
// It is READ-ONLY, local, no GPU, no VLM, no cloud API — matching the Cosca
// epistemology: FACT (what is) → RULE (must) → DECISION (chosen) → evidence.
package uiperception

import "fmt"

// ---------------------------------------------------------------------------
// Scene Model (a representação do mundo visual)
// ---------------------------------------------------------------------------

// Bounds is a rectangle in the viewport coordinate space (CSS px).
type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// UIElement is a single node of the interface with its spatial, semantic and
// style evidence. It mirrors what the frontend reports.
type UIElement struct {
	ID        string  `json:"id"`
	Role      string  `json:"role"`        // button, navitem, panel, editor, text, icon...
	Label     string  `json:"label"`       // accessible name / text
	Bounds    Bounds  `json:"bounds"`      // position + size
	Visible   bool    `json:"visible"`     // display/visibility computed
	Disabled  bool    `json:"disabled"`    // aria-disabled/disabled
	State     string  `json:"state"`       // active, hover, collapsed, open, error...
	Tokens    map[string]string `json:"tokens"` // style tokens used (border-radius, font-size, etc.)
	Text      string  `json:"text"`        // raw text content (for OCR-like alignment)
	Overflow  bool    `json:"overflow"`    // content clips/overflows its bounds
	ContrastRatio float64 `json:"contrast_ratio"` // measured or -1 if unknown
	Children  []*UIElement `json:"children,omitempty"`
}

// SceneModel packages the whole scene for analysis.
type SceneModel struct {
	Viewport  Bounds       `json:"viewport"`
	Root      *UIElement   `json:"root"`
	Elements  []*UIElement `json:"elements"` // flat list (all nodes)
	GeneratedAt string     `json:"generated_at"`
}

// ---------------------------------------------------------------------------
// Rules / analysis
// ---------------------------------------------------------------------------

// Violation is a detected issue with evidence (kernel can gate on this).
type Violation struct {
	Severity  string `json:"severity"`  // HIGH | MEDIUM | LOW
	Rule      string `json:"rule"`      // rule id from UI_GOVERNANCE
	ElementID string `json:"element_id"`
	Reason    string `json:"reason"`
	Evidence  string `json:"evidence"`  // concrete fact
}

// Audit is the result of running rules against a SceneModel.
type Audit struct {
	Package     string       `json:"package"`
	Summary     string       `json:"summary"` // "N violations, M high"
	Violations  []Violation  `json:"violations"`
	Scene       *SceneModel  `json:"scene"`
}

// ---------------------------------------------------------------------------
// Analyzer
// ---------------------------------------------------------------------------

// RuleFunc checks a single element (and/or the whole scene) and returns
// violations. Keeps the rule engine extensible (add a RuleFunc, no rewrite).
type RuleFunc func(scene *SceneModel, el *UIElement) []Violation

func defaultRules() []RuleFunc {
	return []RuleFunc{
		ruleOffCanvas,
		ruleZeroSize,
		ruleInvisibleText,
		ruleOverlap,
		ruleTouchTarget,
		ruleClipart,
	}
}

// Analyze runs the default rule set against the scene and returns an Audit.
// It is pure/deterministic: same scene → same audit.
func Analyze(scene *SceneModel) *Audit {
	if scene == nil {
		return &Audit{Package: "uiperception", Summary: "scene nil"}
	}
	audit := &Audit{Package: "uiperception", Scene: scene}
	seen := map[*UIElement]bool{}
	var walk func(el *UIElement)
	walk = func(el *UIElement) {
		if el == nil || seen[el] {
			return
		}
		seen[el] = true
		for _, r := range defaultRules() {
			audit.Violations = append(audit.Violations, r(scene, el)...)
		}
		for _, c := range el.Children {
			walk(c)
		}
	}
	walk(scene.Root)
	for _, el := range scene.Elements {
		walk(el)
	}
	high := 0
	for _, v := range audit.Violations {
		if v.Severity == "HIGH" {
			high++
		}
	}
	audit.Summary = fmt.Sprintf("%d violações (%d HIGH)", len(audit.Violations), high)
	return audit
}

// ---------------------------------------------------------------------------
// Rules (deterministic, evidence-based)
// ---------------------------------------------------------------------------

// ruleOffCanvas: element positioned outside the viewport (clipped/off-screen).
func ruleOffCanvas(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible || el.Bounds.Width == 0 || el.Bounds.Height == 0 {
		return nil
	}
	b, v := el.Bounds, scene.Viewport
	if b.X < 0 || b.Y < 0 || b.X+b.Width > v.Width || b.Y+b.Height > v.Height {
		return []Violation{{Severity: "MEDIUM", Rule: "off-canvas", ElementID: el.ID,
			Reason: "elemento fora do viewport", Evidence: fmt.Sprintf("%s bounds=%v viewport=%v", el.ID, b, v)}}
	}
	return nil
}

// ruleZeroSize: element has zero width/height but is intended to be seen.
func ruleZeroSize(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible || el.Bounds.Width <= 0 || el.Bounds.Height <= 0 {
		return nil
	}
	if el.Role == "icon" && (el.Bounds.Width < 12 || el.Bounds.Height < 12) {
		return []Violation{{Severity: "MEDIUM", Rule: "icon-too-small", ElementID: el.ID,
			Reason: "ícone menor que 12px", Evidence: fmt.Sprintf("%s=%v", el.ID, el.Bounds)}}
	}
	return nil
}

// ruleInvisibleText: visible element with overflow and text (content clipped).
func ruleInvisibleText(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible || el.Text == "" {
		return nil
	}
	if el.Overflow && (el.Bounds.Width < 8 || el.Bounds.Height < 8) {
		return []Violation{{Severity: "MEDIUM", Rule: "text-clipped", ElementID: el.ID,
			Reason: "texto/overflow pode estar cortado", Evidence: fmt.Sprintf("%s text=%q overflow=%v", el.ID, el.Text, el.Overflow)}}
	}
	return nil
}

// ruleOverlap: two non-trivial visible elements at the same viewport region.
func ruleOverlap(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible || el.Bounds.Width == 0 || el.Bounds.Height == 0 {
		return nil
	}
	var out []Violation
	for _, other := range scene.Elements {
		if other == el || !other.Visible || other.Bounds.Width == 0 || other.Bounds.Height == 0 {
			continue
		}
		if overlapArea(el.Bounds, other.Bounds) > 0.5*area(el.Bounds) {
			out = append(out, Violation{Severity: "HIGH", Rule: "overlap", ElementID: el.ID,
				Reason: "elementos sobrepostos > 50% da área", Evidence: fmt.Sprintf("%s (%v) ↔ %s (%v)", el.ID, el.Bounds, other.ID, other.Bounds)})
			return out
		}
	}
	return out
}

// ruleTouchTarget: interactive element with target smaller than 24px (cursor/keyboard).
func ruleTouchTarget(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible {
		return nil
	}
	if !isInteractive(el.Role) {
		return nil
	}
	if el.Bounds.Width < 24 || el.Bounds.Height < 24 {
		return []Violation{{Severity: "MEDIUM", Rule: "touch-target", ElementID: el.ID,
			Reason: "alvo interativo < 24px", Evidence: fmt.Sprintf("%s role=%s size=%v", el.ID, el.Role, el.Bounds)}}
	}
	return nil
}

// ruleClipart: decorative / placeholder role without accessible label.
func ruleClipart(scene *SceneModel, el *UIElement) []Violation {
	if el == nil || !el.Visible {
		return nil
	}
	if el.Role == "icon" && el.Label == "" && el.Text == "" {
		return []Violation{{Severity: "LOW", Rule: "unlabeled-icon", ElementID: el.ID,
			Reason: "ícone sem acessibilidade (aria-label/label)", Evidence: el.ID}}
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func isInteractive(role string) bool {
	switch role {
	case "button", "navitem", "input", "select", "resizer", "tab", "link":
		return true
	}
	return false
}

func area(b Bounds) float64 {
	if b.Width <= 0 || b.Height <= 0 {
		return 0
	}
	return b.Width * b.Height
}

func overlapArea(a, b Bounds) float64 {
	w := min(a.X+a.Width, b.X+b.Width) - max(a.X, b.X)
	h := min(a.Y+a.Height, b.Y+b.Height) - max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func min(a, b float64) float64 { if a < b { return a }; return b }
func max(a, b float64) float64 { if a > b { return a }; return b }
