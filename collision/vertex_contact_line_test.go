package collision

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"slices"
	"testing"
)

func TestVertexContactAssemblyLinePreservesDistinctMaterialIntervals(t *testing.T) {
	ctx := context.Background()
	x := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	a, err := brep.Box(x(0), x(1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Box(x(1), x(2))
	if err != nil {
		t.Fatal(err)
	}
	parts := []brep.Solid{a, b}
	contact := assemblyContactAt(t, parts, 0, 1, x(1))
	l, err := NewVertexContactAssemblyLocator(ctx, parts, []AssemblyVertexContact{contact}, VertexContactAssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q := func(i int64) *big.Rat { return big.NewRat(i, 1) }
	origin, direction := exact(x(-1)), exact(x(1))
	p, err := l.PartitionRationalLine(ctx, origin, direction, q(0), q(4), RationalLinePartitionOptions{})
	if err != nil || len(p.Stations) != 5 || len(p.Spans) != 4 || p.Work == 0 {
		t.Fatalf("line partition %+v: %v", p, err)
	}
	for i, station := range p.Stations {
		if station.Parameter.Cmp(q(int64(i))) != 0 || station.Location != []Location{Outside, OnBoundary, OnBoundary, OnBoundary, Outside}[i] {
			t.Fatalf("station %d: %+v", i, station)
		}
	}
	for i, span := range p.Spans {
		wantLoc := []Location{Outside, Inside, Inside, Outside}[i]
		wantOwner := [][]int{nil, {0}, {1}, nil}[i]
		if span.Location != wantLoc || !slices.Equal(span.Components, wantOwner) || span.Start != i || span.End != i+1 {
			t.Fatalf("span %d lost original material: %+v", i, span)
		}
	}
	j := p.Stations[2]
	if !slices.Equal(j.Components, []int{0, 1}) || len(j.BoundaryFaces) != 6 || !slices.Equal(j.Contacts, []AssemblyVertexContact{contact}) {
		t.Fatalf("shared station lost its original coowners: %+v", j)
	}
	for i, face := range j.BoundaryFaces {
		if face.Solid != i/3 {
			t.Fatalf("shared station face ancestry %+v", j.BoundaryFaces)
		}
	}
	for _, bounds := range [][2]*big.Rat{{q(-8), q(-4)}, {q(5), q(5)}} {
		r, err := l.PartitionRationalLine(ctx, origin, direction, bounds[0], bounds[1], RationalLinePartitionOptions{})
		if err != nil || r.Stations[0].Location != Outside || r.Stations[len(r.Stations)-1].Location != Outside {
			t.Fatalf("outside finite range %+v %v", r, err)
		}
	}
	if r, err := l.PartitionRationalLine(ctx, origin, direction, q(0), q(4), RationalLinePartitionOptions{MaxWork: 20}); !errors.Is(err, ErrLimit) || r.Stations != nil {
		t.Fatalf("aggregate line budget admitted incomplete material: %+v %v", r, err)
	}
}
