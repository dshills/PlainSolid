package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCurvedAlgebraicPartitionFill(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	sub, line := affineAlgebraicPartitionFixture(t, nil)
	o := CurvedAlgebraicPartitionMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	got, e := sub.TessellateCurvedPartitionCertified(ctx, 0, line, o)
	if e != nil {
		t.Fatal(e)
	}
	if got.FillSquaredDistance.Cmp(big.NewRat(1, 2500)) > 0 || len(got.OriginalFragment) != len(got.Mesh.Triangles) {
		t.Fatal("certificate")
	}
	counts := [2]int{}
	volume := 0.
	type incidence struct{ fragment, sign int }
	edges := map[[2]int][]incidence{}
	for i, tri := range got.Mesh.Triangles {
		fragment := got.OriginalFragment[i]
		counts[fragment]++
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		area := b.Sub(a).Cross(c.Sub(a)).Z / 2
		if area <= 0 {
			t.Fatal("graph orientation")
		}
		volume += area * ((a.Z+b.Z+c.Z)/3 - 2)
		u, v := 0., 0.
		for _, id := range tri {
			x, _ := got.ExactUV[id][0].Float64()
			y, _ := got.ExactUV[id][1].Float64()
			u += x / 3
			v += y / 3
		}
		z := 2 + 1./16 - (u-.5)*(u-.5) - 4*(v-.5)*(v-.5)
		dx, dy, dz := (a.X+b.X+c.X)/3-u, (a.Y+b.Y+c.Y)/3-v, (a.Z+b.Z+c.Z)/3-z
		bound, _ := got.SquaredErrors[i].Float64()
		if dx*dx+dy*dy+dz*dz > bound+1e-16 {
			t.Fatal("analytic interpolation witness")
		}
		for j, x := range tri {
			y := tri[(j+1)%3]
			sign := 1
			if x > y {
				x, y = y, x
				sign = -1
			}
			key := [2]int{x, y}
			edges[key] = append(edges[key], incidence{fragment, sign})
		}
	}
	if counts[0] == 0 || counts[1] == 0 {
		t.Fatal("fragment lost")
	}
	cutEdges := 0
	for _, owners := range edges {
		if len(owners) == 2 && owners[0].fragment != owners[1].fragment {
			if owners[0].sign == owners[1].sign {
				t.Fatal("cut orientation")
			}
			cutEdges++
		}
	}
	if cutEdges != got.Subdivisions {
		t.Fatal("unsynchronized cut", cutEdges, got.Subdivisions)
	}
	if math.Abs(volume-math.Pi/1024) > .0004 {
		t.Fatal("independent lens volume integral", volume, math.Pi/1024)
	}
	report, e := collision.SelfIntersections(got.Mesh, collision.Options{MaxPairs: 1000000})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("independent graph contacts", report, e)
	}
	limited := o
	limited.Mesh.MaxTriangleChecks = 1
	if partial, err := sub.TessellateCurvedPartitionCertified(ctx, 0, line, limited); err == nil || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("triangle budget")
	}
	limited = o
	limited.MaxPredicates = 1
	if partial, err := sub.TessellateCurvedPartitionCertified(ctx, 0, line, limited); err == nil || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("predicate budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sub.TessellateCurvedPartitionCertified(cancelled, 0, line, o); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation", err)
	}

	t.Log("triangles", len(got.Mesh.Triangles), "fragments", counts, "cut edges", cutEdges, "volume", volume, "predicates", got.Predicates)
}
