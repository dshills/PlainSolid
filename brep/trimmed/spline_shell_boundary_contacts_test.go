//go:build research

package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestSplineSeamIdentityGraphContact(t *testing.T) {
	ctx := context.Background()
	// Edge (t,1/2,1/2) traverses the target graph z=x²+1/4.
	// The root t=1/2 would be simple; the 3/4 edge level instead
	// gives sqrt(1/2), which cannot become a rational split hint.
	a, _ := makeOpenSplineBox(t, [3]float64{0, .5, .75}, [3]float64{1, 1, 1}, 0)
	edge := -1
	for i, e := range a.edges {
		x, y := a.vertices[e.Start], a.vertices[e.End]
		if x.Y == .5 && y.Y == .5 && x.Z == .75 && y.Z == .75 {
			edge = i
			break
		}
	}
	if edge < 0 {
		t.Fatal("missing source edge")
	}
	makeGraph := func(top bool) *geom.BSplineSurface {
		ku, kv := []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}
		var poles []geom.Vec3
		for i := range 3 {
			for j := range 2 {
				z := 0.
				if top {
					z = .25
					if i == 2 {
						z += 1
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j), Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	b, _, e := assembleOpenSplineGraphPair(makeGraph(true), makeGraph(false), 0, true, 1e-9,
		partitionGraphRectangleLoop(geom.ParameterRectangle{UMax: 1, VMax: 1}))
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectSplineSeamFace(ctx, a, edge, b, 0, SplineSeamFaceOptions{EnableIdentityGraph: true})
	if e != nil || !r.Complete || r.Graph == nil || len(r.Graph.Hits) != 1 || len(r.Hits) != 0 || r.ExactOperations < 1 {
		t.Fatal(r, e)
	}
	h := r.Graph.Hits[0]
	if h.EdgeParameter.Lower.Cmp(h.EdgeParameter.Upper) >= 0 || h.Event.Multiplicity() != 1 || h.Signs[0]*h.Signs[1] != -1 {
		t.Fatal(h)
	}
}

func TestDiscoverSplineShellBoundaryContacts(t *testing.T) {
	ctx := context.Background()
	// The first box has a nonplanar bilinear top and bottom. The second
	// has different supports. Each direction has a source edge crossing
	// a face interior, and no selected contact is at a shared corner.
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}, 0)
	r, e := DiscoverSplineShellBoundaryContacts(ctx, a, b, SplineShellBoundaryContactOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r.TestedPairs, len(r.Pairs), e)
	}
	var count [2]int
	for _, p := range r.Pairs {
		if p.Contact.SourceFaces[0] < 0 || p.Contact.SourceFaces[1] < 0 || !p.Contact.Complete {
			t.Fatal(p)
		}
		count[p.Source] += len(p.Contact.Hits)
		for _, h := range p.Contact.Hits {
			if h.Direction == 0 || h.CurveParameter.Min > h.CurveParameter.Max {
				t.Fatal(h)
			}
		}
	}
	if count != ([2]int{3, 3}) || r.TestedPairs >= 144 {
		t.Fatal(count, r.TestedPairs)
	}
	distant, _ := makeOpenSplineBox(t, [3]float64{4, 4, 4}, [3]float64{5, 5, 5}, 0)
	r, e = DiscoverSplineShellBoundaryContacts(ctx, a, distant, SplineShellBoundaryContactOptions{})
	if e != nil || !r.Complete || r.TestedPairs != 0 || len(r.Pairs) != 0 {
		t.Fatal(r, e)
	}
	r, e = DiscoverSplineShellBoundaryContacts(ctx, a, b, SplineShellBoundaryContactOptions{MaxWork: 1})
	if !errors.Is(e, geom.ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
