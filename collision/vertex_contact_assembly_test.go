package collision

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func assemblyContactAt(t *testing.T, s []brep.Solid, a, b int, point geom.Vec3) AssemblyVertexContact {
	t.Helper()
	find := func(i int) AssemblyVertex {
		for j, v := range s[i].Vertices() {
			if v == point {
				return AssemblyVertex{i, j}
			}
		}
		t.Fatalf("component %d missing contact vertex %v", i, point)
		return AssemblyVertex{}
	}
	return AssemblyVertexContact{find(a), find(b)}
}

func TestVertexContactAssemblyVerifiesTwoOriginalPointContacts(t *testing.T) {
	ctx := context.Background()
	// Two raised arms have distinct vertices on the two tips of the diamond.
	// The solids share only those two projected points at the common z level.
	outline := func(z float64, xy ...[2]float64) []geom.Vec3 {
		var p []geom.Vec3
		for _, v := range xy {
			p = append(p, geom.Vec3{X: v[0], Y: v[1], Z: z})
		}
		return p
	}
	type xy = [2]float64
	a, err := brep.Prism(outline(0, xy{0, 0}, xy{6, 0}, xy{6, 4}, xy{5, 4}, xy{4, 3}, xy{5, 2}, xy{5, 1}, xy{1, 1}, xy{1, 2}, xy{2, 3}, xy{1, 4}, xy{0, 4}), geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Prism(outline(1, xy{2, 3}, xy{3, 2.75}, xy{4, 3}, xy{3, 3.25}), geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	parts := []brep.Solid{a, b}
	p := assemblyContactAt(t, parts, 1, 0, geom.Vec3{X: 2, Y: 3, Z: 1})
	q := assemblyContactAt(t, parts, 0, 1, geom.Vec3{X: 4, Y: 3, Z: 1})
	r, err := ValidateVertexContactAssembly(ctx, parts, []AssemblyVertexContact{p, q}, VertexContactAssemblyOptions{})
	if err != nil || len(r.Contacts) != 2 || r.SignedVolume.Cmp(big.NewRat(29, 2)) != 0 || r.ContactPairs < 2 || r.BroadPairs < r.ContactPairs {
		t.Fatalf("component contact proof %+v, %v", r, err)
	}
	for _, entry := range r.Contacts {
		if entry.A.Solid != 0 || entry.B.Solid != 1 {
			t.Fatalf("noncanonical component order: %+v", entry)
		}
	}
	if _, err := ValidateAssembly(ctx, parts, AssemblyOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("strict assembly accepted touching components: %v", err)
	}
	for _, declarations := range [][]AssemblyVertexContact{{p}, {p, p}, {{p.A, q.A}}} {
		if r, err := ValidateVertexContactAssembly(ctx, parts, declarations, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) || r.SignedVolume != nil || r.Contacts != nil {
			t.Fatalf("accepted invalid ledger %+v: %+v %v", declarations, r, err)
		}
	}
}

func TestVertexContactAssemblyRejectsSegmentsCrossingAndContainment(t *testing.T) {
	ctx := context.Background()
	box := func(a, b geom.Vec3) brep.Solid {
		t.Helper()
		s, err := brep.Box(a, b)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	stock := box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	segment := box(geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 2, Y: 2, Z: 1})
	parts := []brep.Solid{stock, segment}
	a := assemblyContactAt(t, parts, 0, 1, geom.Vec3{X: 1, Y: 1})
	b := assemblyContactAt(t, parts, 0, 1, geom.Vec3{X: 1, Y: 1, Z: 1})
	if _, err := ValidateVertexContactAssembly(ctx, parts, []AssemblyVertexContact{a, b}, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("segment contact accepted as two vertex contacts: %v", err)
	}
	inside := box(geom.Vec3{X: .25, Y: .25, Z: .25}, geom.Vec3{X: .5, Y: .5, Z: .5})
	crossing := box(geom.Vec3{X: .5, Y: .5, Z: .5}, geom.Vec3{X: 2, Y: 2, Z: 2})
	face := box(geom.Vec3{X: 1}, geom.Vec3{X: 2, Y: 1, Z: 1})
	for _, body := range []brep.Solid{inside, crossing, face} {
		if _, err := ValidateVertexContactAssembly(ctx, []brep.Solid{stock, body}, nil, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("overlap/containment accepted: %v", err)
		}
	}
	tip := box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 2})
	parts[1] = tip
	declared := []AssemblyVertexContact{assemblyContactAt(t, parts, 0, 1, geom.Vec3{X: 1, Y: 1, Z: 1})}
	if r, err := ValidateVertexContactAssembly(ctx, parts, declared, VertexContactAssemblyOptions{}); err != nil || r.SignedVolume.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatalf("valid corner contact: %+v %v", r, err)
	}
	for _, o := range []VertexContactAssemblyOptions{{MaxCrossPairs: 1}, {Validation: LocationOptions{MaxTriangles: 23}}, {MaxSolids: 1}} {
		if r, err := ValidateVertexContactAssembly(ctx, parts, declared, o); !errors.Is(err, ErrLimit) || r.SignedVolume != nil {
			t.Fatalf("unfinished proof %+v: %+v %v", o, r, err)
		}
	}
	if _, err := ValidateVertexContactAssembly(ctx, []brep.Solid{stock.Reversed()}, nil, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inward shell accepted: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ValidateVertexContactAssembly(cancelled, parts, declared, VertexContactAssemblyOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
