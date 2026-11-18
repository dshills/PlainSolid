//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func TestCornerTerminalAudit(t *testing.T) {
	slowtest.Skip(t)
	a, af := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, bf := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	ledger, err := CertifySplineFacePointContact(context.Background(), a, b, [2]int{4, 5}, SplineRectangleEmbeddingOptions{})
	if err != nil {
		t.Fatal("point ledger", err)
	}
	if !ledger.PointOnly() || !ledger.PendingFan() || !ledger.Matches(a, b, [2]int{4, 5}) || ledger.Matches(b, a, [2]int{4, 5}) {
		t.Fatal("ledger binding")
	}
	if ledger.Domains()[0] == ledger.Domains()[1] || ledger.OverlapDomains()[0] != ledger.OverlapDomains()[1] {
		t.Fatal("original domain binding")
	}
	point := ledger.Point()
	for _, q := range point {
		if q.Sign() != 0 {
			t.Fatal(q)
		}
	}
	point[0].SetInt64(99)
	if ledger.Point()[0].Sign() != 0 {
		t.Fatal("alias")
	}
	for _, pair := range [][2]int{{-1, 5}, {0, 5}, {4, 0}} {
		if r, e := CertifySplineFacePointContact(context.Background(), a, b, pair, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
			t.Fatal("unsupported contact accepted", pair)
		}
	}
	if r, e := CertifySplineFacePointContact(context.Background(), a, b, [2]int{4, 5}, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || r != nil {
		t.Fatal("budget")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := CertifySplineFacePointContact(canceled, a, b, [2]int{4, 5}, SplineRectangleEmbeddingOptions{}); e != context.Canceled || r != nil {
		t.Fatal("cancel", e)
	}
	var pairs [][2]int
	for i := range af {
		for j := range bf {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	s, _, e := CombineOpenPlaneSplineShells(context.Background(), a, b, parametric.Intersection, pairs, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if e == nil || len(s.Faces()) != 0 {
		t.Fatal("expected current corner rejection")
	}
	t.Log("full assembly", e, "analytic common volume 7/32")
	// Exact isolated terminal ownership already fails independently of discovery.
	p := intersect.Patch{Min: geom.Vec2{}, Max: geom.Vec2{X: 1, Y: 1}}
	hint := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 3)}
	_, e = openPlaneTerminalEndpoint(p, p, hint)
	if e == nil {
		t.Fatal("corner unexpectedly accepted")
	}
	t.Log("ownership", e)
	for i, f := range af {
		if f.axis == 2 && f.fixed != 0 {
			continue
		}
		if f.axis != 2 && f.fixed != 0 {
			continue
		}
		for j, g := range bf {
			if g.axis != 2 || g.fixed != 0 {
				continue
			}
			var patches [2]intersect.Patch
			for k, face := range []Face{a.faces[i], b.faces[j]} {
				d, e := faceUVBounds(face)
				if e != nil {
					t.Fatal(e)
				}
				patches[k], e = intersectSplineFace(face, d)
				if e != nil {
					t.Fatal(e)
				}
			}
			proof, e := intersect.CertifyOpenSplinePlanePair(context.Background(), patches[0], patches[1], intersect.OverlappingGraphTraceOptions{})
			t.Log("pair", i, j, "complete", proof.Complete, "terminals", len(proof.Terminals), "error", e)
		}
	}
}
