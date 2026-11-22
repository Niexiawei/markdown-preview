package view

import "math"

// The body's zoom, as the outline's buttons and the shortcuts step it: a
// multiplier of the theme's text size, held between the two, stepped by
// zoomStep, one showing it as the theme has it.
const (
	zoomMin     float32 = 0.8
	zoomMax     float32 = 2.0
	zoomStep    float32 = 0.1
	zoomDefault float32 = 1
)

// zoomBy steps the body's text size, within its range, as the outline's two
// buttons and the platform's shortcuts do.
func (s *State) zoomBy(delta float32) {
	z := float32(math.Round(float64(s.zoomOrOne()+delta)*100) / 100)
	s.zoom = min(zoomMax, max(zoomMin, z))
	s.applyWebZoom()
}

// resetZoom shows the body at the theme's own size.
func (s *State) resetZoom() {
	s.zoom = zoomDefault
	s.applyWebZoom()
}

// zoomPercent is the zoom as the outline shows it, as a whole percentage.
func (s *State) zoomPercent() int {
	return int(math.Round(float64(s.zoomOrOne()) * 100))
}

// zoomOrOne is the zoom, one while it was never changed.
func (s *State) zoomOrOne() float32 {
	if s.zoom == 0 {
		return zoomDefault
	}
	return s.zoom
}
