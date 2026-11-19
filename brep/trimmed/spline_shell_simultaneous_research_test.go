//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSectionedSplineShellsSimultaneousEdge(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, 0)
	// At p=(1,1,1/2), A's x-max and y-max faces meet the
	// B edge along w=(1,1,0). B's other directions are chosen
	// so only opposite pairs enter both domains.
	b = splineSimultaneousTransform(t, b, geom.Affine{{0, .25, .25, .875}, {.25, 0, .25, .875}, {.25, .25, 0, .5}})
	o := SplineShellCombineOptions{}
	r, e := SectionSplineShells(ctx, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Complete || len(r.Wires) != 1 {
		t.Fatal(r)
	}
	seen := false
	for _, w := range r.Wires {
		for _, p := range w.Pieces {
			r, _, _ := p.Segment.TrimA().Recipe()
			for _, c := range r.Data().Cells {
				seen = seen || c.BoundaryAnchor != nil
			}
		}
	}
	if !seen {
		t.Fatal("simultaneous terminal did not use a boundary-anchored tube")
	}
	// The certified result must satisfy the complementary Boolean identities.
	var volumes [3]float64
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineSplineShells(ctx, a, b, op, o)
		if e != nil {
			t.Fatal(op, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		volumes[i] = p.Volume
		if op == parametric.Intersection {
			if h.Method != "affine-open-wire" || len(h.SectionEdges) == 0 {
				t.Fatal(h.Method, len(h.SectionEdges))
			}
			var buf bytes.Buffer
			if e := WriteJSON(&buf, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSONContext(ctx, &buf, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			q, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(q.Volume-p.Volume) > 1e-12 {
				t.Fatal(q, e)
			}
		}
	}
	pa, _ := a.MassProperties(ctx, parametric.IntegrationOptions{})
	pb, _ := b.MassProperties(ctx, parametric.IntegrationOptions{})
	if volumes[1] <= 0 || math.Abs(volumes[0]+volumes[1]-pa.Volume-pb.Volume) > 1e-10 || math.Abs(volumes[2]+volumes[1]-pa.Volume) > 1e-10 {
		t.Fatal(volumes, pa.Volume, pb.Volume)
	}
	// In B's coordinates the material overlap is
	// u+w<=1/2, v+w<=1/2. Its volume before the affine map
	// is the integral of (1/2-w)^2 from 0 to 1/2: 1/24.
	if math.Abs(volumes[1]-1.0/768) > 1e-12 {
		t.Fatal("independent overlap volume", volumes[1])
	}
}
