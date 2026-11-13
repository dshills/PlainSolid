package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestPerpendicularCylinderIntersection(t *testing.T) {
	for _, radii := range [][2]float64{{1, 2}, {2, 1}, {2, 2}} {
		rx, rz := radii[0], radii[1]
		q, e := IntersectPerpendicularCylinders(context.Background(), Vec3{X: 7, Y: 2, Z: 3}, rx, Vec3{X: -1, Y: 2, Z: 8}, rz, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || len(q.Arcs) != 4 {
			t.Fatal(q, e)
		}
		if rx == rz {
			if q.Topology != PerpendicularCylinderTwoNodalEllipses || len(q.Nodes) != 2 || len(q.Components) != 1 || q.Components[0].CycleRank != 3 {
				t.Fatal(q)
			}
			for _, n := range q.Nodes {
				if !n.SurfaceTangent || len(n.IncidentArcs) != 4 {
					t.Fatal(n)
				}
			}
		} else {
			if q.Topology != PerpendicularCylinderTwoLoops || len(q.Components) != 2 || len(q.Nodes) != 4 {
				t.Fatal(q)
			}
			for _, c := range q.Components {
				if c.CycleRank != 1 {
					t.Fatal(c)
				}
			}
		}
		for arc := 0; arc < 4; arc++ {
			for _, u := range []float64{.1, .4, .8} {
				p, d, e := q.EvaluateArc(arc, u)
				if e != nil {
					t.Fatal(e)
				}
				x, y, z := p.X+1, p.Y-2, p.Z-3
				if math.Abs(y*y+z*z-rx*rx) > 1e-12 || math.Abs(x*x+y*y-rz*rz) > 1e-12 || math.Abs(y*d.Y+z*d.Z) > 1e-12 || math.Abs(x*d.X+y*d.Y) > 1e-12 || d.Y <= 0 {
					t.Fatal(p, d)
				}
			}
		}
	}
	q, e := IntersectPerpendicularCylinders(context.Background(), Vec3{}, 1, Vec3{}, 2, SurfacePatchEvaluationOptions{})
	if e != nil || q.Nodes[0].Radicands[0].Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectPerpendicularCylinders(context.Background(), Vec3{}, 1, Vec3{Y: math.SmallestNonzeroFloat64}, 2, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectPerpendicularCylinders(context.Background(), Vec3{}, 1, Vec3{}, 2, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Nodes) != 0 {
		t.Fatal(q, e)
	}
}
