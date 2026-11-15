package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestVertexContactAssemblyWithCavities(t *testing.T) {
	ctx := context.Background()
	c := func(x float64) geom.Vec3 { return geom.Vec3{X: x, Y: x, Z: x} }
	outer, cavity := dbox(t, c(0), c(8)), dbox(t, c(2), c(6))
	stock := locationJoin(t, outer, cavity.Reversed())
	unit := dbox(t, c(0), c(1))
	placed := func(offset float64) brep.Solid {
		v := unit.Vertices()
		for i, p := range v {
			v[i] = geom.Vec3{
				X: offset + (p.X+p.Y+p.Z)/2,
				Y: offset + (p.X+2*p.Y+p.Z)/2,
				Z: offset + (p.X+p.Y+2*p.Z)/2,
			}
		}
		b, err := brep.NewSolid(v, unit.Edges(), unit.Faces())
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	fill := placed(2)
	solids := []brep.Solid{stock, fill}
	if _, err := DiscoverVertexContactAssembly(ctx, solids, VertexContactAssemblyOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("default one-shell admission changed: %v", err)
	}
	o := VertexContactAssemblyOptions{AllowCavities: true}
	p, err := DiscoverVertexContactAssembly(ctx, solids, o)
	if err != nil || len(p.Contacts) != 1 || p.SignedVolume.Cmp(big.NewRat(3585, 8)) != 0 {
		t.Fatalf("cavity corner and independent material measures: %+v %v", p, err)
	}
	hit := p.Contacts[0]
	if stock.Vertices()[hit.A.Vertex] != c(2) || fill.Vertices()[hit.B.Vertex] != c(2) {
		t.Fatalf("cavity contact lost original ownership: %+v", hit)
	}
	l, err := NewVertexContactAssemblyLocator(ctx, solids, p.Contacts, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, trial := range []struct {
		point         geom.Vec3
		location      Location
		owners, owner int
	}{
		{c(2), OnBoundary, 2, 0},
		{c(5), Outside, 0, -1},
		{c(1), Inside, 1, 0},
		{geom.Vec3{X: 2.75, Y: 3, Z: 3}, Inside, 1, 1},
	} {
		have, err := l.Locate(ctx, trial.point)
		if err != nil || have.Location != trial.location || len(have.Owners) != trial.owners ||
			trial.owners > 0 && have.Owners[0].Solid != trial.owner {
			t.Fatalf("cavity assembly query %v: %+v %v", trial.point, have, err)
		}
	}
	volume, err := l.MassPropertiesExact(ctx, ExactMassOptions{})
	if err != nil || volume.Volume.Cmp(p.SignedVolume) != 0 {
		t.Fatalf("exact locator and signed shell sums disagree: %+v %v", volume, err)
	}
	for _, bad := range [][]brep.Solid{
		{stock, placed(1.5)},                                  // crosses the cavity wall
		{stock, dbox(t, c(.5), c(1))},                         // contained in material
		{locationJoin(t, outer, cavity), fill},                // wrongly oriented void
		{locationJoin(t, outer, dbox(t, c(10), c(11))), fill}, // two exteriors
	} {
		if got, err := DiscoverVertexContactAssembly(ctx, bad, o); !errors.Is(err, ErrInvalid) || got.SignedVolume != nil {
			t.Fatalf("bad material published an assembly: %+v %v", got, err)
		}
	}
	r, err := DiscoverVertexContactAssembly(ctx, []brep.Solid{fill, stock}, o)
	if err != nil || len(r.Contacts) != 1 || r.SignedVolume.Cmp(p.SignedVolume) != 0 {
		t.Fatalf("operand order changed material proof: %+v %v", r, err)
	}
}
