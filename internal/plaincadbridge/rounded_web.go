package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type roundedWebBoreFaces struct {
	frame     geom.Frame
	low, high int
	span      float64
}

func (b *Bridge) fuseRoundedWeb(a, tool stored) (Shape, bool, error) {
	if a.planar() {
		a, tool = tool, a
	}
	if a.analytic == nil || a.analytic.rounded == nil || !tool.planar() {
		return Shape{}, false, nil
	}
	base := *a.analytic.rounded
	if base.fillet != 0 || base.chamfer <= 0 || base.cut || base.stack != nil || base.pedestal != nil || !cardinalFrame(base.frame) {
		return Shape{}, false, nil
	}
	authored, ok := orthogonalPrism(tool.solid, base.frame)
	p := base.profile
	if !ok || authored.Min.X != p.Min.X || authored.Max.X != p.Max.X || authored.Max.Y != p.Max.Y ||
		authored.Min.Z <= base.chamfer || authored.Min.Z >= base.height || authored.Max.Z <= base.height {
		return Shape{}, false, nil
	}
	result, err := trimmed.JoinBottomChamferedRoundedRectangleWeb(context.Background(), base.frame, p, base.height, base.chamfer,
		authored.Min.Y, authored.Min.Z, authored.Max.Z, a.analytic.solid.Tolerance())
	if err != nil {
		return Shape{}, true, err
	}
	faces := result.Solid.Faces()
	entry, ok := faces[result.WebSides[1]].Surface.(geom.PlaneSurface)
	if !ok {
		return Shape{}, true, geom.ErrDomain
	}
	depth, width := authored.Max.Y-authored.Min.Y, authored.Max.X-authored.Min.X
	base.stack = &roundedStack{top: authored.Max.Z, upper: result.WebTop, pocketCap: -1,
		web: &roundedWebBoreFaces{frame: entry.Frame(), low: result.WebSides[0], high: result.WebSides[1], span: depth}}

	// The web fills two outside-corner pieces in the overlap band and
	// contributes its complete rectangular section above the original foot.
	radius := p.Radii[0]
	cornerArea := (2 - math.Pi/2) * radius * radius
	cornerLength := (4 - math.Pi) * radius
	overlap, height := base.height-authored.Min.Z, authored.Max.Z-base.height
	volume := a.analytic.volume + cornerArea*overlap + width*depth*height
	area := a.analytic.area + 2*cornerArea + cornerLength*overlap + 2*(width+depth)*height
	shape, err := b.putAnalyticBody(&analyticBody{solid: result.Solid, rounded: &base, volume: volume, area: area})
	return shape, true, err
}

func (p roundedWebBoreFaces) throughSite(c *cylinderBody) (roundedBoreSite, trimmed.AxialBoreStep, error) {
	start := p.frame.ToLocal(c.frame.Origin())
	axis, normal := c.frame.ZAxis(), p.frame.ZAxis()
	if axis != normal && axis != normal.Scale(-1) || p.frame.ToWorld(start) != c.frame.Origin() {
		return roundedBoreSite{}, trimmed.AxialBoreStep{}, geom.ErrDomain
	}
	end := start.Z + c.height*axis.Dot(normal)
	if !finite(end) || min(start.Z, end) > -p.span || max(start.Z, end) < 0 {
		return roundedBoreSite{}, trimmed.AxialBoreStep{}, geom.ErrDomain
	}
	return roundedBoreSite{center: geom.Vec2{X: start.X, Y: start.Y}, lower: p.low, upper: p.high, span: p.span},
		trimmed.AxialBoreStep{Radius: c.radius}, nil
}
