package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestCertifySplineEdgePairIncidenceWarpedShell(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, 1.0625}, [3]float64{1.5, 1.5, 2}, 0)
	find := func(s Solid, x, y geom.Vec3) int {
		t.Helper()
		for i, e := range s.edges {
			p, q := s.vertices[e.Start], s.vertices[e.End]
			if p == x && q == y || p == y && q == x {
				return i
			}
		}
		t.Fatalf("missing edge from %v to %v", x, y)
		return -1
	}
	ea := find(a, geom.Vec3{X: 1, Z: 1}, geom.Vec3{X: 1, Y: 1, Z: 1.125})
	eb := find(b, geom.Vec3{X: .5, Y: .5, Z: 1.0625}, geom.Vec3{X: 1.5, Y: .5, Z: 1.0625})
	contact, e := IntersectSplineEdges(ctx, a, ea, b, eb, geom.ExactCurveSegmentOptions{})
	if e != nil || !contact.Complete || len(contact.Contacts) != 1 || !contact.Contacts[0].Splittable {
		t.Fatal(contact, e)
	}
	anchors := contact.Contacts[0].Anchors
	p, e := CertifySplineEdgePairIncidence(ctx, a, b, anchors, geom.TubeEndpointEdgeOptions{})
	if e != nil || !p.Complete || len(p.Germs) != 4 ||
		p.Point[0].Cmp(big.NewRat(1, 1)) != 0 || p.Point[1].Cmp(big.NewRat(1, 2)) != 0 || p.Point[2].Cmp(big.NewRat(17, 16)) != 0 {
		t.Fatal(p, e)
	}
	// The plane z=17/16 cuts the warped top along xy=1/2.
	// It also cuts the x=1 side for y>1/2; neither pairing with
	// the y=1/2 face enters both material charts.
	want := map[[2]int]bool{{1, 4}: true, {5, 4}: true}
	for _, g := range p.Germs {
		pair := [2]int{g.Faces[0].Face, g.Faces[1].Face}
		if g.Inward != want[pair] {
			t.Fatal("unexpected continuation", pair, g.Inward)
		}
		delete(want, pair)
	}
	if len(want) != 0 {
		t.Fatal(want)
	}
	q, e := CertifySplineEdgePairIncidence(ctx, b, a, [2]EdgeAnchor{anchors[1], anchors[0]}, geom.TubeEndpointEdgeOptions{})
	if e != nil || !q.Complete || !exactPointCoordinatesEqual(p.Point, q.Point) {
		t.Fatal(q, e)
	}
	anchors[1].Parameter += .125
	bad, e := CertifySplineEdgePairIncidence(ctx, a, b, anchors, geom.TubeEndpointEdgeOptions{})
	if !errors.Is(e, geom.ErrConvergence) || bad.Complete || len(bad.Germs) != 0 {
		t.Fatal(bad, e)
	}
}
