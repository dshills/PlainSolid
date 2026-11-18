package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedMixedConicalBoreMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}
	m, e := TessellateFilletedFrustumBoreSetCertified(ctx, s, 15./16, 15./8, cuts, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxSubdivisions: 16, MaxTriangleChecks: 30000}, MaxTriangles: 20000})
	if e != nil {
		t.Fatal(e)
	}
	t.Log(len(m.Mesh.Triangles), m.TriangleChecks, m.Predicates)
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if m.Source.Topology().Faces != 10 || m.Source.Topology().EulerCharacteristic != 0 || len(m.OriginalFace) != len(m.Mesh.Triangles) {
		t.Fatal("source")
	}
	key := func(a, b int) [2]int {
		if a > b {
			a, b = b, a
		}
		return [2]int{a, b}
	}
	uses := map[[2]int][]int{}
	for i, tri := range m.Mesh.Triangles {
		for k := 0; k < 3; k++ {
			pair := key(tri[k], tri[(k+1)%3])
			uses[pair] = append(uses[pair], m.OriginalFace[i])
		}
	}
	if len(m.Mesh.Vertices)-len(uses)+len(m.Mesh.Triangles) != 0 {
		t.Fatal("display genus")
	}
	checkRow := func(row, edges []int, params []*big.Rat, owners [2]int) {
		t.Helper()
		for j, id := range row {
			u := uses[key(id, row[(j+1)%len(row)])]
			if len(u) != 2 || !((u[0] == owners[0] && u[1] == owners[1]) || (u[0] == owners[1] && u[1] == owners[0])) {
				t.Fatal("rim incidence", u, owners)
			}
			parameter, _ := params[j].Float64()
			point, e := m.Source.edges[edges[j]].Curve.Eval(parameter)
			if e != nil || point.Sub(m.Mesh.Vertices[id]).Length() > 1e-12 {
				t.Fatal("native edge parameter", e)
			}
		}
	}
	for end, row := range m.OuterRims {
		checkRow(row, m.OuterEdges[end], m.OuterParameters[end], [2]int{m.History.Fillets.Blends[end], m.History.Fillets.Caps[end]})
	}
	for i, cut := range m.Cuts {
		h := m.History.Cuts[i]
		for end, row := range cut.Rims {
			neighbor := m.History.Fillets.Caps[end]
			if cuts[i].Depth != 0 && cuts[i].FromUpper != (end == 1) {
				neighbor = h.FloorFace
			}
			checkRow(row, cut.Edges[end], cut.Parameters[end], [2]int{h.BoreFace, neighbor})
		}
	}
	area2 := func(a, b geom.Vec3) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X)))
	}
	checkFill := func(owner int, rows [][]int) {
		t.Helper()
		want := new(big.Rat)
		for role, row := range rows {
			a := new(big.Rat)
			for j, id := range row {
				a.Add(a, area2(m.Mesh.Vertices[id], m.Mesh.Vertices[row[(j+1)%len(row)]]))
			}
			if role == 0 {
				want.Add(want, a)
			} else {
				want.Sub(want, a)
			}
		}
		got := new(big.Rat)
		for i, tri := range m.Mesh.Triangles {
			if m.OriginalFace[i] != owner {
				continue
			}
			a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
			q := new(big.Rat).Add(area2(a, b), area2(b, c))
			q.Add(q, area2(c, a))
			if q.Sign() == 0 || ((q.Sign() < 0) != m.Source.faces[owner].Reversed) {
				t.Fatal("floor/cap orientation", owner)
			}
			got.Add(got, q)
		}
		got.Abs(got)
		if got.Cmp(want) != 0 {
			t.Fatal("exact fill area", owner)
		}
	}
	for end, owner := range m.History.Fillets.Caps {
		rows := [][]int{m.OuterRims[end]}
		for i, c := range cuts {
			if c.Depth == 0 || c.FromUpper == (end == 1) {
				rows = append(rows, m.Cuts[i].Rims[end])
			}
		}
		checkFill(owner, rows)
	}
	for i, h := range m.History.Cuts {
		if h.FloorFace >= 0 {
			end := 1
			if cuts[i].FromUpper {
				end = 0
			}
			checkFill(h.FloorFace, [][]int{m.Cuts[i].Rims[end]})
			z, _ := h.FloorZ.Float64()
			for _, id := range m.Cuts[i].Rims[end] {
				if m.Mesh.Vertices[id].Z != z {
					t.Fatal("floor height")
				}
			}
		}
	}
	r0, r1 := 15./16, 15./8
	M0, M1 := 8-r0/3, 16-3*r1
	theta := -math.Atan2(4, 3)
	arc := func(r, M, a, b float64) float64 {
		F := func(x float64) float64 {
			sn, cs := math.Sin(x), math.Cos(x)
			return math.Pi * r * (M*M*sn + M*r*(x+sn*cs) + r*r*(sn-sn*sn*sn/3))
		}
		return F(b) - F(a)
	}
	z0, z1 := r0/5, 6-9*r1/5
	R0, R1 := 8+4*z0/3, 8+4*z1/3
	truth := arc(r0, M0, -math.Pi/2, theta) + arc(r1, M1, theta, math.Pi/2) + math.Pi*(z1-z0)*(R0*R0+R0*R1+R1*R1)/3 - 11*math.Pi/4
	v, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(v-truth) > .01*truth {
		t.Fatal("independent volume", v, truth)
	}
	if m.SquaredDeflection.Cmp(new(big.Rat).Mul(ratInset(.2), ratInset(.2))) > 0 || m.TriangleChecks > 30000 {
		t.Fatal("deflection/budget")
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, m.Source); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil || back.Topology().Faces != 10 {
		t.Fatal(e)
	}
	bad, e := TessellateFilletedFrustumBoreSetCertified(ctx, s, 15./16, 15./8, cuts, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 1}})
	if e == nil || len(bad.Mesh.Vertices) != 0 || bad.Source.Topology().Faces != 0 {
		t.Fatal("atomic budget")
	}

}
