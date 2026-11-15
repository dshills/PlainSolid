package collision

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func concaveContactPrism(t *testing.T, x, z, dz float64, rotation int) brep.Solid {
	t.Helper()
	p := []geom.Vec3{{X: x, Z: z}, {X: x + 3, Z: z}, {X: x + 3, Y: 1, Z: z}, {X: x + 1, Y: 1, Z: z}, {X: x + 1, Y: 3, Z: z}, {X: x, Y: 3, Z: z}}
	s, err := brep.Prism(append(append([]geom.Vec3{}, p[rotation:]...), p[:rotation]...), geom.Vec3{Z: dz})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPlanarFaceContactConcaveCompleteOriginal(t *testing.T) {
	ctx := context.Background()
	a, b := concaveContactPrism(t, 0, 0, 1, 0), concaveContactPrism(t, 0, 1, 1, 3)
	for i, parts := range [][2]brep.Solid{{a, b}, {b, a}} {
		p, err := ValidatePlanarFaceContact(ctx, parts[0], parts[1], 1-i, i, PlanarFaceContactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if p.TwiceProjectedArea.RatString() != "10" || p.SignedVolume.RatString() != "10" || p.DropAxis != 2 || len(p.Edges) != 6 || len(p.Vertices) != 6 || p.BroadPairs == 0 {
			t.Fatalf("unexpected contact: %+v", p)
		}
		for j, v := range p.Vertices {
			for k, part := range parts {
				edge := part.Edges()[p.Edges[j][k]]
				w := p.Vertices[(j+1)%6][k]
				if edge != (brep.Edge{Start: v[k], End: w}) && edge != (brep.Edge{Start: w, End: v[k]}) {
					t.Fatalf("paired edge %d does not connect original vertices %d and %d: %+v", j, v[k], w, edge)
				}
			}
			if parts[0].Vertices()[v[0]] != parts[1].Vertices()[v[1]] {
				t.Fatalf("mismatched original vertices: %v", v)
			}
		}
	}
}

func TestPlanarFaceContactRejectsUnprovedContact(t *testing.T) {
	ctx := context.Background()
	a, b := concaveContactPrism(t, 0, 0, 1, 0), concaveContactPrism(t, 0, 1, 1, 3)
	overlap := concaveContactPrism(t, .25, 1, 1, 0)
	for _, tc := range []struct {
		b       brep.Solid
		i, j    int
		options PlanarFaceContactOptions
		want    error
	}{
		{overlap, 1, 0, PlanarFaceContactOptions{}, ErrInvalid},
		{b, 0, 0, PlanarFaceContactOptions{}, ErrInvalid},
		{a, 1, 1, PlanarFaceContactOptions{}, ErrInvalid},
		{b, 1, 0, PlanarFaceContactOptions{Cross: Options{MaxPairs: 1}}, ErrLimit},
		{b, 1, 0, PlanarFaceContactOptions{MaxPerimeterChecks: 1}, ErrLimit},
	} {
		p, err := ValidatePlanarFaceContact(ctx, a, tc.b, tc.i, tc.j, tc.options)
		if !errors.Is(err, tc.want) || p.SignedVolume != nil {
			t.Fatalf("got %+v, %v; want %v", p, err, tc.want)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if p, err := ValidatePlanarFaceContact(cancelled, a, b, 1, 0, PlanarFaceContactOptions{}); !errors.Is(err, context.Canceled) || p.SignedVolume != nil {
		t.Fatalf("cancelled contact published a proof: %+v %v", p, err)
	}
}
