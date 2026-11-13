package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCurvedPartitionUVBoundary(t *testing.T) {
	ctx := context.Background()
	sub, line := affineAlgebraicPartitionFixture(t, nil)
	o := AlgebraicBoundaryMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .001}}
	got, e := sub.TessellatePartitionUVBoundaryCertified(ctx, 0, line, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Certificates) != 6 || len(got.TopologyVertices) != 6 || got.BoundarySquaredDistance.Cmp(big.NewRat(1, 1000000)) > 0 {
		t.Fatal("coverage")
	}
	for edge, chart := range got.Charts {
		if chart.UV == nil || !chart.Model.Complete || !got.Certificates[edge].Complete {
			t.Fatal("missing source restriction")
		}
	}
	bound, _ := got.BoundarySquaredDistance.Float64()
	for i := 0; i < 256; i++ {
		theta := 2 * math.Pi * float64(i) / 256
		p := geom.Vec2{X: .5 + .25*math.Cos(theta), Y: .5 + .125*math.Sin(theta)}
		best := math.Inf(1)
		for _, path := range got.Edges {
			for j := 0; j+1 < len(path); j++ {
				a, b := got.UV[path[j]], got.UV[path[j+1]]
				dx, dy := b.X-a.X, b.Y-a.Y
				t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				x, y := p.X-a.X-t*dx, p.Y-a.Y-t*dy
				distance := x*x + y*y
				if distance < best {
					best = distance
				}
			}
		}
		if best > bound+1e-18 {
			t.Fatal("independent ellipse witness", best, bound)
		}
	}
	directions := [2]int{}
	for fi, ring := range got.Fragments {
		for j, a := range ring {
			b := ring[(j+1)%len(ring)]
			if a == got.Cut[0] && b == got.Cut[1] {
				directions[fi] = 1
			}
			if b == got.Cut[0] && a == got.Cut[1] {
				directions[fi] = -1
			}
		}
	}
	if directions[0]*directions[1] != -1 {
		t.Fatal("cut incidence")
	}
	low := o
	low.Display.MaxOperations = 1
	if partial, err := sub.TessellatePartitionUVBoundaryCertified(ctx, 0, line, low); err == nil || len(partial.UV) != 0 {
		t.Fatal("operation budget")
	}
	low = o
	low.Display.MaxSegments = 1
	if partial, err := sub.TessellatePartitionUVBoundaryCertified(ctx, 0, line, low); err == nil || len(partial.UV) != 0 {
		t.Fatal("segment budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sub.TessellatePartitionUVBoundaryCertified(cancelled, 0, line, o); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	t.Log("UV vertices", len(got.UV), "segments", got.Segments, "squared bound", bound, "operations", got.ExactOperations)
}
