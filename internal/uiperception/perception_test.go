package uiperception

import "testing"

func b(x, y, w, h float64) Bounds { return Bounds{X: x, Y: y, Width: w, Height: h} }

func TestAnalyze_OffCanvas(t *testing.T) {
	scene := &SceneModel{
		Viewport: b(0, 0, 1000, 800),
		Root:     &UIElement{ID: "root", Role: "workspace", Visible: true, Bounds: b(0, 0, 1000, 800)},
		Elements: []*UIElement{
			{ID: "btn", Role: "button", Visible: true, Bounds: b(900, 700, 200, 30)}, // fora do viewport
		},
	}
	audit := Analyze(scene)
	for _, v := range audit.Violations {
		if v.Rule == "off-canvas" && v.ElementID == "btn" {
			return // esperado
		}
	}
	t.Errorf("esperava off-canvas para 'btn', got %v", audit.Violations)
}

func TestAnalyze_ZeroSizeIcon(t *testing.T) {
	scene := &SceneModel{
		Viewport: b(0, 0, 1000, 800),
		Root:     &UIElement{ID: "root", Role: "workspace", Visible: true, Bounds: b(0, 0, 1000, 800)},
		Elements: []*UIElement{{ID: "icon", Role: "icon", Visible: true, Bounds: b(100, 100, 10, 10)}}, // <12px
	}
	audit := Analyze(scene)
	found := false
	for _, v := range audit.Violations {
		if v.Rule == "icon-too-small" && v.ElementID == "icon" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperava icon-too-small, got %v", audit.Violations)
	}
}

func TestAnalyze_OverlapHigh(t *testing.T) {
	scene := &SceneModel{
		Viewport: b(0, 0, 1000, 800),
		Root:     &UIElement{ID: "root", Role: "workspace", Visible: true, Bounds: b(0, 0, 1000, 800)},
		Elements: []*UIElement{
			{ID: "panel", Role: "panel", Visible: true, Bounds: b(100, 100, 300, 200)},
			{ID: "over", Role: "dialog", Visible: true, Bounds: b(150, 150, 300, 200)}, // sobrepõe >50%
		},
	}
	audit := Analyze(scene)
	found := false
	for _, v := range audit.Violations {
		if v.Rule == "overlap" && v.Severity == "HIGH" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperava overlap HIGH, got %v", audit.Violations)
	}
}

func TestAnalyze_NoViolations_FineScene(t *testing.T) {
	scene := &SceneModel{
		Viewport: b(0, 0, 1000, 800),
		Root:     &UIElement{ID: "root", Role: "workspace", Visible: true, Bounds: b(0, 0, 1000, 800)},
		Elements: []*UIElement{
			{ID: "btn", Role: "button", Visible: true, Bounds: b(40, 40, 120, 40), Label: "Settings"},
			{ID: "nav", Role: "navitem", Visible: true, Bounds: b(40, 90, 120, 30), Label: "Files"},
		},
	}
	audit := Analyze(scene)
	if len(audit.Violations) != 0 {
		t.Errorf("cena válida não deve ter violações, got %v", audit.Violations)
	}
}

func TestAnalyze_TouchTarget(t *testing.T) {
	scene := &SceneModel{
		Viewport: b(0, 0, 1000, 800),
		Root:     &UIElement{ID: "root", Role: "workspace", Visible: true, Bounds: b(0, 0, 1000, 800)},
		Elements: []*UIElement{{ID: "tiny", Role: "button", Visible: true, Bounds: b(10, 10, 18, 18)}}, // <24px
	}
	audit := Analyze(scene)
	found := false
	for _, v := range audit.Violations {
		if v.Rule == "touch-target" && v.ElementID == "tiny" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperava touch-target, got %v", audit.Violations)
	}
}
