package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/collision"
	"testing"
)

func mappedChainClosing(t *testing.T) []PCurve {
	t.Helper()
	p := [][2]float64{{1, .25}, {1, .75}, {0, .75}, {0, .25}}
	var out []PCurve
	for i := 0; i < 3; i++ {
		q, e := NewExactQuadraticUV([3][3]*big.Rat{{ratInset(p[i][0]), ratInset(p[i][1]), big.NewRat(1, 1)}, {ratInset((p[i][0] + p[i+1][0]) / 2), ratInset((p[i][1] + p[i+1][1]) / 2), big.NewRat(1, 1)}, {ratInset(p[i+1][0]), ratInset(p[i+1][1]), big.NewRat(1, 1)}})
		if e != nil {
			t.Fatal(e)
		}
		out = append(out, q)
	}
	return out
}
func TestMappedSplineChainFace(t *testing.T) {
	ctx := context.Background()
	a, ca := mappedChainFixture(t, false)
	b, cb := mappedChainFixture(t, true)
	proof, e := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = proof.WriteJSON(ctx, &data, MappedChainNativeOptions{}); e != nil {
		t.Fatal(e)
	}
	proof, e = ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(data.Bytes()), MappedChainNativeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	closing := mappedChainClosing(t)
	o := MappedChainFaceOptions{Boundary: MappedChainPolylineOptions{Deflection: .005}, Face: MixedExactMultiSpanMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 2}, Face: MixedExactCurvedMeshOptions{BoundaryModelBudget: .005, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
	r, e := proof.TessellateFaceCertified(ctx, 0, closing, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Collision.Complete || len(r.Collision.Contacts) > 0 || r.Face.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.04), ratInset(.04))) > 0 {
		t.Fatal("certificate")
	}
	for i, id := range r.LedgerVertices {
		if r.Face.Mesh.Vertices[id] != r.Boundary.Vertices[i].Point {
			t.Fatal("ledger changed")
		}
	}
	for _, s := range r.Samples {
		for owner, c := range proof.sources {
			uv := authoredMixedSplinePoint(t, c.parts[0].source, s.AuthoredParameters[owner])
			x, y := uv[0], uv[1]
			if owner == 1 {
				x, y = y, x
			}
			if r.Face.ExactPoints[s.Vertex][0].Cmp(x) != 0 || r.Face.ExactPoints[s.Vertex][1].Cmp(y) != 0 || r.Face.ExactPoints[s.Vertex][2].Cmp(new(big.Rat).Mul(x, y)) != 0 {
				t.Fatal("original support ownership")
			}
		}
	}
	for _, tri := range r.Face.Mesh.Triangles {
		u, v, z := new(big.Rat), new(big.Rat), new(big.Rat)
		for _, id := range tri {
			u.Add(u, r.Face.ExactUV[id][0])
			v.Add(v, r.Face.ExactUV[id][1])
			z.Add(z, ratInset(r.Face.Mesh.Vertices[id].Z))
		}
		u.Quo(u, big.NewRat(3, 1))
		v.Quo(v, big.NewRat(3, 1))
		z.Quo(z, big.NewRat(3, 1))
		d := new(big.Rat).Sub(new(big.Rat).Mul(u, v), z)
		if new(big.Rat).Mul(d, d).Cmp(r.Face.FillSquaredDistance) > 0 {
			t.Fatal("independent z=xy witness")
		}
	}
	t.Logf("triangles=%d seam samples=%d ledger vertices=%d", len(r.Face.Mesh.Triangles), len(r.Samples), len(r.LedgerVertices))
	for _, bad := range []MappedChainFaceOptions{{Boundary: MappedChainPolylineOptions{MaxSegments: 1}, Face: o.Face}, {Boundary: o.Boundary, Face: o.Face, Collision: collision.Options{MaxPairs: 1}}} {
		if out, e := proof.TessellateFaceCertified(ctx, 0, closing, bad); e == nil || len(out.Face.Mesh.Triangles) != 0 {
			t.Fatal("atomic quota")
		}
	}
	badClosing := append([]PCurve(nil), closing...)
	badClosing[0] = closing[1]
	if out, e := proof.TessellateFaceCertified(ctx, 0, badClosing, o); e == nil || len(out.Face.Mesh.Triangles) != 0 {
		t.Fatal("broken closing ring")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := proof.TessellateFaceCertified(canceled, 0, closing, o); e == nil || len(out.Face.Mesh.Triangles) != 0 {
		t.Fatal("cancel")
	}
}
