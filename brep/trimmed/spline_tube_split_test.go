package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestSplitCertifiedSplineTubeLoop(t *testing.T) {
	k1, k2 := []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k1, k1, nil)
	q := [3]float64{.25, -.25, .25}
	const r = 5.0 / 16
	var p []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: q[i] + q[j] - r*r})
		}
	}
	b, _ := geom.NewBSplineSurface(2, 2, 3, 3, p, k2, k2, nil)
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	patch := func(s *geom.BSplineSurface) intersect.Patch {
		return intersect.Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
	}
	var seeds intersect.SplineSeedReport
	var add func(int, float64, float64, float64, int)
	add = func(axis int, lo, hi, sign float64, depth int) {
		t.Helper()
		coord := func(x float64) float64 { return .5 + sign*math.Sqrt(r*r-(x-.5)*(x-.5)) }
		y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
		if lo <= .5 && hi >= .5 {
			y0, y1 = min(y0, .5+sign*r), max(y1, .5+sign*r)
		}
		pad := (hi - lo) / 2
		pad = min(pad, .03125)
		d := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: y0 - pad, VMax: y1 + pad}
		g := d
		g.UMin, g.UMax = lo-pad, hi+pad
		if axis == 1 {
			d.UMin, d.UMax, d.VMin, d.VMax = d.VMin, d.VMax, d.UMin, d.UMax
			g.UMin, g.UMax, g.VMin, g.VMax = g.VMin, g.VMax, g.UMin, g.UMax
		}
		x, e := pa.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		y, e := pb.Trim(g)
		if e != nil {
			t.Fatal(e)
		}
		c, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		if !c.ProvenBranch {
			if depth == 8 {
				t.Fatal("no local proof", d, g)
			}
			m := (lo + hi) / 2
			add(axis, lo, m, sign, depth+1)
			add(axis, m, hi, sign, depth+1)
			return
		}
		seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: g}, Tube: &c})
	}
	for _, sign := range []float64{-1, 1} {
		add(0, .5-3.0/16, .5+3.0/16, sign, 0)
		add(1, .5-4.0/16, .5+4.0/16, sign, 0)
	}
	ctx := context.Background()
	report, e := intersect.JoinSplineTubeBranches(ctx, patch(a), patch(b), seeds, intersect.TubeBranchOptions{})
	if e != nil || report.Truncated || len(report.Branches) != 1 || !report.Branches[0].Closed ||
		len(report.Branches[0].Cells) != len(seeds.Attempts) {
		t.Fatal(report, e)
	}
	segments, e := intersect.SplineBranchCurves(ctx, patch(a), patch(b), seeds, report.Branches[0], intersect.TubeCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(box, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	face := -1
	for i, f := range faces {
		fr := f.Surface.(geom.PlaneSurface).Frame()
		if fr.Origin().Z != 0 || fr.ZAxis().Z >= 0 {
			continue
		}
		face = i
		faces[i].Surface = a
		faces[i].Reversed = true
		uses := append([]Use(nil), f.Outer...)
		for j, u := range uses {
			v, w := fr.ToWorld(geom.Vec3{X: u.UVStart.X, Y: u.UVStart.Y}), fr.ToWorld(geom.Vec3{X: u.UVEnd.X, Y: u.UVEnd.Y})
			u.UVStart, u.UVEnd = geom.Vec2{X: w.X, Y: w.Y}, geom.Vec2{X: v.X, Y: v.Y}
			u.Reversed = !u.Reversed
			faces[i].Outer[len(uses)-1-j] = u
		}
		break
	}
	if face < 0 {
		t.Fatal("no bottom face")
	}
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	source := s
	s, h, e := s.SplitSplineTubeLoop(ctx, face, segments, false)
	if e != nil || len(h.Edges) != len(segments) || len(s.faces) != 7 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	var native bytes.Buffer
	if e = WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&native, JSONLimits{})
	if e != nil || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(s.Topology(), e)
	}
	s, h, e = source.ReplaceSplineTubePatch(ctx, face, segments, false)
	if e != nil || s.faces[h.Inner].Surface != b || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	// A moderately refined closed mesh is an independent geometric
	// volume check of this local edit; exact tube sampling is expensive.
	m, e := s.refine(ctx, 1, 0, 0)
	want := 1 + math.Pi*math.Pow(r, 4)/2
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 1.005 || math.Abs(m.SignedVolume()-want) > .005 {
		t.Fatal(m.SignedVolume(), want, e)
	}
}
