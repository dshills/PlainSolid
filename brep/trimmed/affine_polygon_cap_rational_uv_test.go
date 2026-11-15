package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func checkPolygonCapRationalUV(t *testing.T, s Solid) {
	t.Helper()
	found := false
	for _, f := range s.faces {
		for _, u := range f.Outer {
			q, ok := u.Curve.(ExactQuadraticUV)
			if !ok || !q.rationalEndpoints {
				t.Fatal("missing exact native pcurve")
			}
			ends, display := q.Endpoints()
			if display[0] != u.UVStart || display[1] != u.UVEnd {
				t.Fatal("wrong display binding")
			}
			for k := range ends {
				for axis := range ends[k] {
					if ends[k][axis].Cmp(big.NewRat(8, 9)) == 0 {
						found = true
						value := display[k].X
						if axis == 1 {
							value = display[k].Y
						}
						if ratInset(value).Cmp(ends[k][axis]) == 0 {
							t.Fatal("fixture did not distinguish exact and display")
						}
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("lost original 8/9")
	}
	vertices, e := s.RationalVertices(context.Background())
	if e != nil || len(vertices) != len(s.vertices) {
		t.Fatal("exact vertex ledger", len(vertices), e)
	}
	for _, v := range vertices {
		if !sameRationalVertex(v.Point, affineMaterialXYZ(s.vertices[v.Vertex])) {
			t.Fatal("changed represented model vertex")
		}
	}
	report, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !report.Complete {
		t.Fatal(e)
	}
	for _, u := range report.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal("nonexact model agreement")
		}
	}
}
func TestConvexAffinePolygonCapRationalUV(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCapFixtureUnits(t, 0, false, false)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		faces := [2]int{1, 0}
		if order == 1 {
			faces = [2]int{0, 1}
		}
		contact, e := CertifyConvexAffinePolygonCapContact(ctx, pair[0], pair[1], faces, ConvexAffinePolygonCapOptions{})
		if e != nil {
			t.Fatal(e)
		}
		result, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{RationalUVEndpoints: true})
		if e != nil {
			t.Fatal(e)
		}
		s := result.Solid()
		checkPolygonCapRationalUV(t, s)
		if result.ExactVolume().Cmp(big.NewRat(60639, 4096)) != 0 || len(s.vertices)-len(s.edges)+len(s.faces) != 2 {
			t.Fatal("volume/topology")
		}
		for fi, source := range result.FaceSources() {
			if s.faces[fi].Surface != pair[source.Owner].faces[source.Face].Surface {
				t.Fatal("reparameterized original support")
			}
		}
		var buffer bytes.Buffer
		if e = WriteJSON(&buffer, s); e != nil {
			t.Fatal(e)
		}
		data := append([]byte(nil), buffer.Bytes()...)
		decoded, e := ReadJSONContext(ctx, &buffer, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		checkPolygonCapRationalUV(t, decoded)
		var recipe nativeSolid
		if e = json.Unmarshal(data, &recipe); e != nil {
			t.Fatal(e)
		}
		if recipe.Faces[0].Outer[0].ExactQuadratic == nil {
			t.Fatal("missing native exact recipe")
		}
		controls := recipe.Faces[0].Outer[0].ExactQuadratic
		mid, ok := new(big.Rat).SetString(controls[1][0])
		if !ok {
			t.Fatal("coefficient")
		}
		controls[1][0] = mid.Add(mid, big.NewRat(1, 1000000)).RatString()
		tampered, e := json.Marshal(recipe)
		if e != nil {
			t.Fatal(e)
		}
		if loaded, e := ReadJSONContext(ctx, bytes.NewReader(tampered), JSONLimits{}); e == nil || len(loaded.faces) != 0 {
			t.Fatal("changed continuous source use accepted")
		}
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{RationalUVEndpoints: true, MaxOperations: result.ExactOperations() - 1}); e == nil || q != nil {
			t.Fatal("one-short work")
		}
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{RationalUVEndpoints: true, MaxOperations: result.ExactOperations()}); e != nil || q == nil {
			t.Fatal("exact work", e)
		}
		if q, e := contact.Fuse(ctx, ConvexAffinePolygonCapFuseOptions{RationalUVEndpoints: true, Evaluation: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}); e == nil || q != nil {
			t.Fatal("evaluation exhaustion")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if q, e := contact.Fuse(canceled, ConvexAffinePolygonCapFuseOptions{RationalUVEndpoints: true}); e == nil || q != nil {
			t.Fatal("cancellation")
		}

		t.Log("exact UV native", len(s.faces), "faces", result.ExactOperations(), "operations")
	}
}
