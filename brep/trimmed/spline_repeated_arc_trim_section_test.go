//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestSectionRepeatedSplineGraphArcTrimmed(t *testing.T) {
	ctx := context.Background()
	// A.Z-B.Z=((5+2u)v-(2+u))². Both supports retain a shared
	// independent nonzero graph height u*v along the contact itself.
	controls := [3][3]float64{{4, -6, 9}, {6, -8.5, 12}, {9, -12, 16}}
	var faces [2]Face
	for i := range faces {
		var poles []geom.Vec3
		for u := range 3 {
			for v := range 3 {
				p := geom.Vec3{X: float64(u) / 2, Y: float64(v) / 2}
				p.Z = p.X*p.Y + float64(1-i)*controls[u][v]
				poles = append(poles, p)
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		faces[i].Surface = s
	}
	wire := func(first int, points ...geom.Vec2) []Use {
		out := make([]Use, len(points))
		for i, p := range points {
			out[i] = Use{Edge: first + i, UVStart: p, UVEnd: points[(i+1)%len(points)]}
		}
		return out
	}
	faces[0].Outer = wire(17, geom.Vec2{X: .125, Y: .25}, geom.Vec2{X: .875, Y: .25}, geom.Vec2{X: .875, Y: .75}, geom.Vec2{X: .125, Y: .75})
	faces[1].Outer = wire(33, geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1})
	faces[1].Holes = [][]Use{wire(37, geom.Vec2{X: .25, Y: .375}, geom.Vec2{X: .25, Y: .625}, geom.Vec2{X: .75, Y: .5})}
	faces[1].Holes[0][2].Reversed = true
	r, e := SectionRepeatedSplineGraphArc(ctx, faces, 1, RepeatedSplineArcTrimOptions{})
	if e != nil || !r.Complete || !r.Arc.Proven || len(r.Nodes) != 6 || len(r.Intervals) != 5 {
		t.Fatalf("complete original graph arc clipping: %v nodes=%d intervals=%d: %v", r.Complete, len(r.Nodes), len(r.Intervals), e)
	}
	for i, p := range r.Intervals {
		if p.Retained != (i == 1 || i == 3) || (p.Locations[1] == trimvalidate.Outside) != (i == 2) || p.Start != i || p.End != i+1 {
			t.Fatal("two exact retained runs separated by the original hole", i, p)
		}
	}
	for i, n := range r.Nodes {
		if n.Isolated || n.OnFaces != (i > 0 && i < 5) {
			t.Fatal("closed contact incidence", i, n)
		}
	}
	// The slanted hole edge is v=u/4+5/16, so the exact intersection
	// parameter satisfies 8u²+14u−7=0 in (1/4,1/2).
	root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "14", "8"}, Lower: "1/4", Upper: "1/2"}, geom.ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	n := r.Nodes[3]
	q, e := n.Parameter.Compare(ctx, root, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !q.Proven || q.Order != 0 || len(n.Contacts[1]) != 1 {
		t.Fatal("independently isolated rational-arc intersection", q, e)
	}
	c := n.Contacts[1][0]
	if c.Face != 1 || c.Loop != 1 || c.Use != 2 || c.Edge != 39 || !c.Reversed || c.CoedgeEndpointMask != 0 || c.CoedgeParameter.Lower.Sign() <= 0 || c.CoedgeParameter.Upper.Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal("lost original directed hole edge", c)
	}
	for i, m := range r.Model {
		if !m.Complete || r.Sources[i] != faces[i].Surface || len(m.Cells) != 1 {
			t.Fatal("original paired composed arcs", m)
		}
		p, e := m.Cells[0].EvaluateRational(ctx, big.NewRat(1, 2), geom.ExactCurvePlaneEventOptions{})
		if e != nil || p.Point[2].Cmp(big.NewRat(5, 24)) != 0 {
			t.Fatal("contact lies on original nonplanar graph in both charts", p, e)
		}
	}
	if p, e := SectionRepeatedSplineGraphArc(ctx, faces, 1, RepeatedSplineArcTrimOptions{MaxContacts: 1}); !errors.Is(e, geom.ErrConvergence) || p.Complete || len(p.Nodes) != 0 {
		t.Fatal("contact limit must fail atomically", e)
	}

	// Replace the bottom edge with a curved UV(t)=((t+t²)/2,3/8+t/8).
	// Its original spline is reversed and has knot units [2,6].
	faces[0].Outer = wire(45, geom.Vec2{Y: .375}, geom.Vec2{X: 1, Y: .5}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1})
	curved, e := geom.NewBSpline(2, []geom.Vec3{{X: 1, Y: .5}, {X: .25, Y: 7.0 / 16}, {Y: .375}}, []float64{2, 2, 2, 6, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	faces[0].Outer[0].Curve = SplineUV{Curve: curved, Range: UVInterval{Min: 2, Max: 6}, Reversed: true}
	faces[0].Outer[0].Reversed = true
	faces[1].Holes = nil
	r, e = SectionRepeatedSplineGraphArc(ctx, faces, 1, RepeatedSplineArcTrimOptions{})
	if e != nil || !r.Complete || len(r.Nodes) != 3 || !r.Intervals[0].Retained || r.Intervals[1].Retained || len(r.Nodes[1].Contacts[0]) != 1 {
		t.Fatalf("exact curved original coedge section: %v nodes=%d: %v", r.Complete, len(r.Nodes), e)
	}
	c = r.Nodes[1].Contacts[0][0]
	if c.Edge != 45 || c.Span != 0 || !c.Reversed || c.ExactCoedgeParameter == nil || c.CoedgeEndpointMask != 0 {
		t.Fatal("original reversed curve ownership", c)
	}
	root, e = geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "4", "0", "1"}, Lower: "0", Upper: "1/4"}, geom.ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root, _, e = root.AffineMap(ctx, big.NewRat(6, 1), big.NewRat(-4, 1), geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q, e := c.ExactCoedgeParameter.Compare(ctx, root, geom.ExactCurvePlaneEventOptions{}); e != nil || !q.Proven || q.Order != 0 {
		t.Fatal("exact original curved coedge root in knot units", q, e)
	}
	var encoded bytes.Buffer
	r.Nodes = nil
	if e := WriteRepeatedSplineGraphArcJSON(&encoded, r); e != nil {
		t.Fatal(e)
	}
	back, e := ReadRepeatedSplineGraphArcJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{}, RepeatedSplineArcTrimOptions{})
	if e != nil || !back.Complete || len(back.Nodes) != 3 || !back.Intervals[0].Retained || back.Intervals[1].Retained {
		t.Fatal("source recipe must independently reprove trimmed arc", e)
	}
	contact := back.Nodes[1].Contacts[0][0]
	if q, e := contact.ExactCoedgeParameter.Compare(ctx, root, geom.ExactCurvePlaneEventOptions{}); e != nil || !q.Proven || q.Order != 0 || contact.Edge != 45 {
		t.Fatal("native reload lost original source-use identity", q, e)
	}
	var corrupt nativeRepeatedSplineArcSection
	if e := json.Unmarshal(encoded.Bytes(), &corrupt); e != nil {
		t.Fatal(e)
	}
	corrupt.Faces[0].Outer[0].SplineReversed = false
	bad, e := json.Marshal(corrupt)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := ReadRepeatedSplineGraphArcJSON(ctx, bytes.NewReader(bad), JSONLimits{}, RepeatedSplineArcTrimOptions{}); e == nil || q.Complete {
		t.Fatal("corrupted original pcurve direction accepted", e)
	}
}
