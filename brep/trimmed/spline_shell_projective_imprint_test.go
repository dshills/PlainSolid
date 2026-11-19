//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestImprintProjectiveSplineShellFacesPropagatesOriginalCuts(t *testing.T) {
	ctx := context.Background()
	a := projectiveImprintShellFixture(t, true, false)
	b := projectiveImprintShellFixture(t, false, true)
	axes := [2]int{0, 1}
	r, err := a.ImprintProjectiveSplineShellFaces(ctx, 5, b, 5, axes, ProjectiveShellImprintOptions{})
	if err != nil || !r.Complete {
		t.Fatal("native section failed to propagate to adjacent original faces", r, err)
	}
	for owner, v := range r.Owners {
		if len(v.Solid.faces) != 7 || len(v.Solid.edges) != 15 || len(v.Solid.vertices) != 10 ||
			len(v.SourceFaces[5]) != 2 || v.InsertedBoundaries != 2 {
			t.Fatal("wrong exact closed shell subdivision", owner, len(v.Solid.faces), len(v.Solid.edges), len(v.Solid.vertices), v)
		}
		section := v.Solid.edges[v.SectionEdge]
		negativeChecked := false
		for _, faceIndex := range v.SourceFaces[5] {
			f := v.Solid.faces[faceIndex]
			for _, u := range f.Outer {
				if u.Edge != v.SectionEdge {
					continue
				}
				q := u.Curve.(ExactQuadraticUV).HomogeneousControls()
				q[1][0].Add(q[1][0], new(big.Rat).Quo(q[1][2], big.NewRat(64, 1)))
				changed, err := NewExactQuadraticUV(q)
				if err != nil {
					t.Fatal(err)
				}
				u.Curve = changed
				match, err := compareSplineEdgeUse(ctx, section, f.Surface, u, edgecheck.Options{})
				if err != nil || match {
					t.Fatal("matching terminals do not prove a perturbed original pcurve", owner, faceIndex, match, err)
				}
				negativeChecked = true
			}
		}
		if !negativeChecked {
			t.Fatal("missing original native section use", owner)
		}
		for _, want := range []geom.Vec3{{X: .5, Z: 1}, {X: .5, Y: 1, Z: 1.5}} {
			if v.Solid.vertices[section.Start] != want && v.Solid.vertices[section.End] != want {
				t.Fatal("canonical section edge lost an original terminal", owner, section, want)
			}
		}
		ends := make(map[int]int)
		for _, f := range v.Solid.faces {
			for _, u := range f.Outer {
				ends[u.Edge]++
			}
		}
		for edge, count := range ends {
			if count != 2 {
				t.Fatal("unpaired closed face edge", owner, edge, count)
			}
		}
		type intervalKey struct{ lo, hi string }
		bySource := make(map[int]map[intervalKey]int)
		for _, e := range v.Edges {
			if e.Edge == v.SectionEdge || e.Lower.Cmp(e.Upper) >= 0 {
				t.Fatal("lost original model edge ancestry", e)
			}
			if bySource[e.OriginalEdge] == nil {
				bySource[e.OriginalEdge] = make(map[intervalKey]int)
			}
			key := intervalKey{e.Lower.RatString(), e.Upper.RatString()}
			if previous, ok := bySource[e.OriginalEdge][key]; ok && previous != e.Edge {
				t.Fatal("opposite original coedges must name one exact sewn representative", e, previous)
			}
			bySource[e.OriginalEdge][key] = e.Edge
		}
		if len(bySource) != 12 {
			t.Fatal("must preserve every original model edge", owner, bySource)
		}
		splits := 0
		for _, parts := range bySource {
			if len(parts) == 1 {
				if _, whole := parts[intervalKey{"0", "1"}]; !whole {
					t.Fatal("original whole source clock disappeared", parts)
				}
			} else {
				if _, left := parts[intervalKey{"0", "1/2"}]; !left {
					t.Fatal("left source subinterval missing", parts)
				}
				if _, right := parts[intervalKey{"1/2", "1"}]; !right || len(parts) != 2 {
					t.Fatal("right source subinterval missing", parts)
				}
				splits++
			}
		}
		if splits != 2 {
			t.Fatal("the exact section should split two original edges", owner, splits)
		}
		props, err := v.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || math.Abs(props.Volume-1) > 1e-10 || props.Centroid.Sub(geom.Vec3{X: .5, Y: .5, Z: .75}).Length() > 1e-10 {
			t.Fatal("topology-only imprint changed independent stock volume or centroid", owner, props, err)
		}
	}
	r, err = a.ImprintProjectiveSplineShellFaces(ctx, 5, b, 5, axes, ProjectiveShellImprintOptions{Sew: ExactSplineSewOptions{MaxPairTests: 1}})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Owners[0].Solid.faces) != 0 {
		t.Fatal("torn shell cannot escape if propagation exhausts its work", r, err)
	}
}

func projectiveImprintShellFixture(t *testing.T, shift, fractionalY bool) Solid {
	t.Helper()
	knots := []float64{0, 0, 1, 1}
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var flips []bool
	for axis := range 3 {
		for side := range 2 {
			var pts []geom.Vec3
			var weights []float64
			for i := range 2 {
				for j := range 2 {
					p := [3]float64{}
					p[axis], p[(axis+1)%3], p[(axis+2)%3] = float64(side), float64(i), float64(j)
					z := p[2] + p[0]*p[1]
					if shift {
						z += p[0] - .5
					}
					pts = append(pts, geom.Vec3{X: p[0], Y: p[1], Z: z})
					w := 1.
					if fractionalY && axis != 1 && p[1] == 1 {
						w = 2
					}
					weights = append(weights, w)
				}
			}
			s, err := geom.NewBSplineSurface(1, 1, 2, 2, pts, knots, knots, weights)
			if err != nil {
				t.Fatal(err)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			flips = append(flips, side == 0)
		}
	}
	s, _, err := SewSplineRectangles(context.Background(), surfaces, domains, flips, SewOptions{Tolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
