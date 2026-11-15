package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestPlanarEdgeContactKeepsBothOriginalOwners(t *testing.T) {
	ctx := context.Background()
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b := dbox(t, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 2, Y: 2, Z: 1})
	p, err := DiscoverPlanarEdgeContact(ctx, a, b, PlanarEdgeContactOptions{})
	p0, p1 := geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: 1, Z: 1}
	if err != nil || p.SignedVolume.RatString() != "2" ||
		!(p.Start == p0 && p.End == p1 || p.Start == p1 && p.End == p0) || p.ContactPairs == 0 {
		t.Fatalf("complete original edge not recovered: %+v %v", p, err)
	}
	for i, s := range []brep.Solid{a, b} {
		edge, ids := [2]int{p.EdgeA, p.EdgeB}[i], [2][2]int{p.FacesA, p.FacesB}[i]
		faces := s.Faces()
		if ids[0] == ids[1] {
			t.Fatal("original edge incidence displaced an owner", ids)
		}
		for _, f := range ids {
			found := false
			for _, use := range faces[f].Boundary {
				found = found || use.Edge == edge
			}
			if !found {
				t.Fatalf("part %d face %d does not own original edge %d", i, f, edge)
			}
		}
	}
	edges, faces := b.Edges(), b.Faces()
	edge := &edges[p.EdgeB]
	edge.Start, edge.End = edge.End, edge.Start
	for i := range faces {
		for j := range faces[i].Boundary {
			u := &faces[i].Boundary[j]
			if u.Edge == p.EdgeB {
				u.Reversed = !u.Reversed
			}
		}
	}
	reversed, err := brep.NewSolid(b.Vertices(), edges, faces)
	if err != nil {
		t.Fatal(err)
	}
	q, err := ValidatePlanarEdgeContact(ctx, a, reversed, p.EdgeA, p.EdgeB, PlanarEdgeContactOptions{})
	if err != nil || q.Reversed == p.Reversed || q.Start != p.Start || q.End != p.End {
		t.Fatalf("original edge direction not preserved: %+v %v", q, err)
	}
	if r, err := DiscoverPlanarEdgeContact(ctx, reversed, a, PlanarEdgeContactOptions{}); err != nil || r.EdgeA != p.EdgeB || r.EdgeB != p.EdgeA {
		t.Fatalf("operand order changed actual edge identities: %+v %v", r, err)
	}
	if _, err := DiscoverVertexContactAssembly(ctx, []brep.Solid{a, b}, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("separate vertex-only contract widened: %v", err)
	}
}

func TestPlanarEdgeContactRejectsPartialOrOtherContact(t *testing.T) {
	ctx := context.Background()
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	d := geom.Vec3{X: 1, Y: 1, Z: 1}
	for _, low := range []geom.Vec3{
		{X: 1, Y: 1, Z: .5},   // proper subset of an original edge
		{X: 1},                // shared face
		{X: 1, Y: 1, Z: 1},    // isolated corner
		{X: .5, Y: .5, Z: .5}, // actual cross-boundary intersection
		{X: 4, Y: 4, Z: 4},    // disjoint
	} {
		b := dbox(t, low, low.Add(d))
		if p, err := DiscoverPlanarEdgeContact(ctx, a, b, PlanarEdgeContactOptions{}); !errors.Is(err, ErrInvalid) || p.SignedVolume != nil {
			t.Fatalf("unqualified contact published a report at %v: %+v %v", low, p, err)
		}
	}
	b := dbox(t, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 2, Y: 2, Z: 1})
	if p, err := DiscoverPlanarEdgeContact(ctx, a, b, PlanarEdgeContactOptions{Cross: Options{MaxPairs: 1}}); !errors.Is(err, ErrLimit) || p.SignedVolume != nil {
		t.Fatalf("incomplete sweep published a report: %+v %v", p, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if p, err := DiscoverPlanarEdgeContact(cancelled, a, b, PlanarEdgeContactOptions{}); !errors.Is(err, context.Canceled) || p.SignedVolume != nil {
		t.Fatalf("cancelled proof published a report: %+v %v", p, err)
	}
}

func TestPlanarEdgeContactExactLocationAndLinePartition(t *testing.T) {
	ctx := context.Background()
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b := dbox(t, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 2, Y: 2, Z: 1})
	l, err := NewPlanarEdgeContactLocator(ctx, a, b, PlanarEdgeContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := l.Locate(ctx, geom.Vec3{X: 1, Y: 1, Z: .5})
	if err != nil || p.Location != OnBoundary || !p.SharedEdge || len(p.Owners) != 2 || p.Owners[0].Solid != 0 || p.Owners[1].Solid != 1 {
		t.Fatalf("shared edge point lost one original owner: %+v %v", p, err)
	}
	p, err = l.Locate(ctx, geom.Vec3{X: 1, Y: .5, Z: .5})
	if err != nil || p.Location != OnBoundary || p.SharedEdge || len(p.Owners) != 1 {
		t.Fatalf("ordinary boundary was reported as a shared edge: %+v %v", p, err)
	}
	rat := big.NewRat
	part, err := l.PartitionRationalLine(ctx,
		[3]*big.Rat{rat(1, 1), rat(1, 1), rat(-1, 1)},
		[3]*big.Rat{rat(0, 1), rat(0, 1), rat(3, 1)},
		rat(0, 1), rat(1, 1), RationalLinePartitionOptions{})
	if err != nil || len(part.Stations) != 4 || len(part.Spans) != 3 {
		t.Fatalf("shared edge line partition: %+v %v", part, err)
	}
	if part.Stations[1].Parameter.RatString() != "1/3" || part.Stations[2].Parameter.RatString() != "2/3" ||
		!part.Stations[1].SharedEdge || !part.Stations[2].SharedEdge || len(part.Stations[1].BoundaryFaces) != 6 {
		t.Fatalf("lost shared rational endpoints or source face owners: %+v", part.Stations)
	}
	if s := part.Spans[1]; !s.SharedEdge || s.Location != OnBoundary || len(s.Components) != 2 || len(s.BoundaryFaces) != 4 ||
		part.Spans[0].Location != Outside || part.Spans[2].Location != Outside {
		t.Fatalf("lost shared open interval and original face owners: %+v", part.Spans)
	}
	part, err = l.PartitionRationalLine(ctx,
		[3]*big.Rat{rat(1, 2), rat(1, 2), rat(1, 3)}, // nonbinary point on the original edge
		[3]*big.Rat{rat(1, 1), rat(1, 1), rat(0, 1)},
		rat(0, 1), rat(1, 1), RationalLinePartitionOptions{})
	if err != nil || len(part.Stations) != 3 || !part.Stations[1].SharedEdge ||
		part.Stations[1].Parameter.RatString() != "1/2" || len(part.Stations[1].BoundaryFaces) != 4 ||
		part.Spans[0].Location != Inside || part.Spans[1].Location != Inside ||
		len(part.Spans[0].Components) != 1 || part.Spans[0].Components[0] != 0 ||
		len(part.Spans[1].Components) != 1 || part.Spans[1].Components[0] != 1 {
		t.Fatalf("shared station must separate distinct interiors without rounding: %+v %v", part, err)
	}
	restricted, err := NewPlanarEdgeContactLocator(ctx, a, b, PlanarEdgeContactOptions{MaxQueryWork: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p, err := restricted.Locate(ctx, geom.Vec3{X: 1, Y: 1, Z: .5}); !errors.Is(err, ErrLimit) || p.Owners != nil || p.SharedEdge {
		t.Fatalf("incomplete lookup published a partial owner: %+v %v", p, err)
	}
}
