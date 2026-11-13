package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func quadraticClipStock(t *testing.T) (Solid, []*geom.BSplineSurface) {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	knots := []float64{0, 0, 0, 1, 1, 1}
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		if axis == 1 {
			u, v = 0, 2
		}
		for side := 0; side < 2; side++ {
			var poles []geom.Vec3
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					var p [3]float64
					p[axis] = float64(side)
					p[u] = float64(i) / 2
					p[v] = float64(j) / 2
					x2 := p[0] * p[0]
					if u == 0 {
						x2 = 0
						if i == 2 {
							x2 = 1
						}
					}
					if v == 0 {
						x2 = 0
						if j == 2 {
							x2 = 1
						}
					}
					p[2] += x2/8 + p[1]/4
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			rev := side == 0
			if axis == 1 {
				rev = !rev
			}
			reversed = append(reversed, rev)
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, surfaces
}
func TestQuadraticSplinePlaneClip(t *testing.T) {
	ctx := context.Background()
	stock, _ := quadraticClipStock(t)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal("clip", op, e)
		}
		want := 29. / 640
		center := geom.Vec3{X: 65. / 174, Y: 527. / 2436, Z: 325. / 2436}
		if op == parametric.Difference {
			want = 611. / 640
			center = geom.Vec3{X: 1855. / 3666, Y: 26353. / 51324, Z: 35515. / 51324}
		}
		mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-want) > 1e-9 || mass.Centroid.Sub(center).Length() > 1e-9 {
			t.Fatal("integral", mass, e, want)
		}
		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate original edge")
				}
				owned[id] = true
			}
		}
		for _, id := range r.SectionEdges {
			if owned[id] {
				t.Fatal("section/original edge collision")
			}
			owned[id] = true
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("incomplete ancestry")
		}
		if len(r.SectionEdges) != 4 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("topology")
		}
		if len(r.StockFaces[4]) != 1 || r.Solid.faces[r.StockFaces[4][0]].Surface != stock.faces[4].Surface {
			t.Fatal("original curved support")
		}
		cap := r.Solid.faces[r.ToolFaces[5][0]]
		if cap.Surface != tool.faces[5].Surface || cap.Reversed != (op == parametric.Difference) {
			t.Fatal("cap support")
		}
		curved := false
		for _, u := range cap.Outer {
			q, ok := u.Curve.(ExactQuadraticUV)
			if !ok {
				t.Fatal("exact native trim")
			}
			h := q.HomogeneousControls()
			for axis := 0; axis < 2; axis++ {
				v := new(big.Rat).Sub(new(big.Rat).Add(h[0][axis], h[2][axis]), new(big.Rat).Mul(big.NewRat(2, 1), h[1][axis]))
				curved = curved || v.Sign() != 0
			}
		}
		if !curved {
			t.Fatal("cap was flattened")
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		lm, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(lm.Volume-want) > 1e-9 {
			t.Fatal("reload", e)
		}
		corrupt := r.Solid
		corrupt.faces = append([]Face(nil), r.Solid.faces...)
		capID := r.ToolFaces[5][0]
		corrupt.faces[capID] = cloneFace(corrupt.faces[capID])
		q := corrupt.faces[capID].Outer[0].Curve.(ExactQuadraticUV)
		h := q.HomogeneousControls()
		h[1][1].Add(h[1][1], big.NewRat(1, 32))
		bad, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		corrupt.faces[capID].Outer[0].Curve = bad
		var invalid bytes.Buffer
		if e = WriteJSON(&invalid, corrupt); e == nil {
			t.Fatal("writer accepted mismatched curved trace")
		}
		var valid bytes.Buffer
		if e = WriteJSON(&valid, r.Solid); e != nil {
			t.Fatal(e)
		}
		var data nativeSolid
		if e = json.Unmarshal(valid.Bytes(), &data); e != nil {
			t.Fatal(e)
		}
		net := data.Faces[capID].Outer[0].ExactQuadratic
		c, ok := new(big.Rat).SetString(net[1][1])
		if !ok {
			t.Fatal("native rational")
		}
		net[1][1] = c.Add(c, big.NewRat(1, 32)).RatString()
		tampered, e := json.Marshal(data)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadJSONContext(ctx, bytes.NewReader(tampered), JSONLimits{}); e == nil {
			t.Fatal("native reload accepted mismatched curved interior")
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal("mesh", e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
	for _, face := range []int{2, 4} {
		proof, e := CertifyQuadraticPlaneFaceSection(ctx, stock, face, 3./16, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		ends, _ := proof.Trim().Endpoints()
		parameters := proof.Parameters()
		for i, id := range proof.Edges() {
			jet, e := proof.patch.EvaluateRational(ctx, ends[i][0], ends[i][1], geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			parameter, _ := parameters[i].Float64()
			point, e := stock.edges[id].Curve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			for axis, x := range []float64{point.X, point.Y, point.Z} {
				if jet.Point[axis].Cmp(ratInset(x)) != 0 {
					t.Fatal("original edge parameter")
				}
			}
		}
	}
}
