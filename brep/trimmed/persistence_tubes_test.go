package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestNativeSplineTubeCut(t *testing.T) {
	ctx := context.Background()
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	f := s.faces[0]
	frame := f.Surface.(geom.PlaneSurface).Frame()
	lo, hi := f.Outer[0].UVStart, f.Outer[0].UVStart
	for _, u := range f.Outer {
		lo = geom.Vec2{X: min(lo.X, u.UVStart.X), Y: min(lo.Y, u.UVStart.Y)}
		hi = geom.Vec2{X: max(hi.X, u.UVStart.X), Y: max(hi.Y, u.UVStart.Y)}
	}
	d := geom.ParameterRectangle{UMin: lo.X, UMax: hi.X, VMin: lo.Y, VMax: hi.Y}
	k := func(degree int, a, b float64) []float64 {
		r := make([]float64, 2*(degree+1))
		for i := range r {
			r[i] = a
			if i > degree {
				r[i] = b
			}
		}
		return r
	}
	point := func(x, y, z float64) geom.Vec3 { return frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z}) }
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{point(lo.X, lo.Y, 0), point(lo.X, hi.Y, 0), point(hi.X, lo.Y, 0), point(hi.X, hi.Y, 0)}, k(1, lo.X, hi.X), k(1, lo.Y, hi.Y), nil)
	if e != nil {
		t.Fatal(e)
	}
	// The second chart is a parabolic wall. It extends outside the first
	// chart in x, and crosses it in z, so all dependent roots are interior.
	x0, x1, y := lo.X-.5, hi.X+.5, (lo.Y+hi.Y)/2
	var poles []geom.Vec3
	for i, c := range []float64{-5.0 / 64, 13.0 / 64, -5.0 / 64} {
		x := x0 + (x1-x0)*float64(i)/2
		poles = append(poles, point(x, y+c, -1), point(x, y+c, 1))
	}
	wall, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, k(2, x0, x1), k(1, -1, 1), nil)
	if e != nil {
		t.Fatal(e)
	}
	wd := geom.ParameterRectangle{UMin: x0, UMax: x1, VMin: -1, VMax: 1}
	cells := []intersect.TubeRecipeCell{{DomainA: d, DomainB: wd}, {DomainA: d, DomainB: wd}}
	cells[0].DomainA.UMax, cells[1].DomainA.UMin = (lo.X+hi.X)/2, (lo.X+hi.X)/2
	recipe := intersect.TubeRecipeData{
		A:     intersect.Patch{Surface: a, Min: lo, Max: hi},
		B:     intersect.Patch{Surface: wall, Min: geom.Vec2{X: x0, Y: -1}, Max: geom.Vec2{X: x1, Y: 1}},
		Cells: cells,
	}
	segments, e := intersect.RebuildTubeCurves(ctx, recipe)
	if e != nil {
		t.Fatal(e)
	}
	anchors := [2]EdgeAnchor{{Edge: -1}, {Edge: -1}}
	for _, u := range f.Outer {
		if u.UVStart.X != u.UVEnd.X {
			continue
		}
		i := 0
		if u.UVStart.X == hi.X {
			i = 1
		}
		r := s.edges[u.Edge].Range
		anchors[i] = EdgeAnchor{u.Edge, (r.Min + r.Max) / 2}
	}
	faces := s.Faces()
	faces[0].Surface = a
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.SplitSplineTubeAcrossEdges(ctx, 0, anchors[0], anchors[1], segments, true); e == nil {
		t.Fatal("accepted the other tube chart")
	}
	s, hist, e := s.SplitSplineTubeAcrossEdges(ctx, 0, anchors[0], anchors[1], segments, false)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	var native nativeSolid
	if e = json.Unmarshal(data.Bytes(), &native); e != nil || native.Version != 3 || len(native.Tubes) != 1 {
		t.Fatal(native.Version, len(native.Tubes), e)
	}
	s, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, id := range hist.Path.Edges {
		c := s.edges[id].Curve.(surfaceTrimCurve)
		p, e := c.curve.EvalUV(.5)
		if e != nil || p.Y <= y+.02 || math.Abs(p.X-(lo.X+.5+float64(i))) > 1e-10 {
			t.Fatal(p, e)
		}
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-8) > 1e-10 || math.Abs(p.Area-24) > 1e-10 {
		t.Fatal(p, e)
	}
	native.Tubes[0].Cells[1].DomainB.VMin = .25 // the second tube can no longer contact z=0
	corrupt, e := json.Marshal(native)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadJSON(bytes.NewReader(corrupt), JSONLimits{}); e == nil {
		t.Fatal("accepted a tube with no intersection")
	}
}
