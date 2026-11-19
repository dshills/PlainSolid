//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedClosedAlgebraicLensHalves(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := algebraicCapFixture(t, big.NewRat(9, 16))
	before := source.Source.Vertices()
	o := ClosedAlgebraicLensMeshOptions{Boundary: AlgebraicLensBoundaryOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .001}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}}
	got, e := source.TessellateClosedHalvesCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	total := math.Pi / (512 * math.Sqrt2)
	t0 := math.Sqrt2 / 4
	primitive := (3*math.Asin(t0) + t0*math.Sqrt(1-t0*t0)*(5-2*t0*t0)) / 8
	negative := (primitive + 3*math.Pi/16) / (192 * math.Sqrt2)
	sum := 0.
	for i, half := range []CertifiedClosedLensHalf{got.Negative, got.Positive} {
		if e := half.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !half.Collision.Complete || len(half.Collision.Contacts) != 0 {
			t.Fatal("closed display embedding")
		}
		if half.SourceImageSquaredDistance.Sign() <= 0 {
			t.Fatal("source boundary image bound")
		}
		edges := map[[2]int]int{}
		for _, tri := range half.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}]++
			}
		}
		if len(half.Mesh.Vertices)-len(edges)+len(half.Mesh.Triangles) != 2 {
			t.Fatal("half Euler")
		}
		for network, local := range half.NetworkVertices {
			if half.Mesh.Vertices[local] != got.Boundary.Vertices[network] {
				t.Fatal("boundary sample changed")
			}
		}
		volume, _ := half.SignedDisplayVolume.Float64()
		want := negative
		if i == 1 {
			want = total - negative
		}
		if math.Abs(volume-want) > .15*want {
			t.Fatal("independent half volume", i, volume, want)
		}
		sum += volume
		// A separate sweep broadphase checks the accepted closed display.
		contacts, e := collision.SelfIntersections(half.Mesh, collision.Options{MaxPairs: 2000000})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("independent contacts", e)
		}
		t.Log("side", half.Side, "triangles", len(half.Mesh.Triangles), "volume", volume, "analytic", want)
	}
	if math.Abs(sum-total) > .1*total {
		t.Fatal("conserved lens volume", sum, total)
	}
	for i, v := range source.Source.Vertices() {
		if v != before[i] {
			t.Fatal("source mutated")
		}
	}
	low := o
	low.Interior.MaxTriangleChecks = 1
	if partial, err := source.TessellateClosedHalvesCertified(ctx, low); err == nil || len(partial.Negative.Mesh.Triangles) != 0 {
		t.Fatal("partial closed half")
	}
	low = o
	low.Collision.MaxPairs = 1
	if partial, err := source.TessellateClosedHalvesCertified(ctx, low); err == nil || len(partial.Negative.Mesh.Triangles) != 0 {
		t.Fatal("collision budget accepted uncertified shell")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := source.TessellateClosedHalvesCertified(cancelled, o); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

	t.Log("network vertices", len(got.Boundary.Vertices), "predicates", got.Predicates, "triangle checks", got.TriangleChecks)
}
