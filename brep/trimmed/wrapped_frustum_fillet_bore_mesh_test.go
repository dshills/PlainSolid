package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedEccentricBoredConicalFilletMesh(t *testing.T) {
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
	m, e := TessellateFilletedFrustumThroughBoreCertified(ctx, s, 15./16, 15./8, geom.Vec2{X: 1, Y: .5}, 1, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxSubdivisions: 16, MaxTriangleChecks: 30000}, MaxTriangles: 20000})
	if e != nil {
		t.Fatal(e)
	}
	t.Log(len(m.Mesh.Triangles), m.TriangleChecks, m.Predicates)
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if m.Source.Topology().EulerCharacteristic != 0 || len(m.OriginalFace) != len(m.Mesh.Triangles) {
		t.Fatal("native owner")
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
			p := key(tri[k], tri[(k+1)%3])
			uses[p] = append(uses[p], m.OriginalFace[i])
		}
	}
	if len(m.Mesh.Vertices)-len(uses)+len(m.Mesh.Triangles) != 0 {
		t.Fatal("genus one")
	}
	twiceArea := func(a, b geom.Vec3) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Mul(ratInset(a.X), ratInset(b.Y)), new(big.Rat).Mul(ratInset(a.Y), ratInset(b.X)))
	}
	for end, rows := range m.Rims {
		area := new(big.Rat)
		for role, row := range rows {
			ringArea := new(big.Rat)
			for i, id := range row {
				other := row[(i+1)%len(row)]
				ringArea.Add(ringArea, twiceArea(m.Mesh.Vertices[id], m.Mesh.Vertices[other]))
				owners := uses[key(id, other)]
				wall := m.History.Fillets.Blends[end]
				if role == 1 {
					wall = m.History.BoreFace
				}
				if len(owners) != 2 || !((owners[0] == wall && owners[1] == m.History.Fillets.Caps[end]) || (owners[1] == wall && owners[0] == m.History.Fillets.Caps[end])) {
					t.Fatal("shared annular rim owner", owners)
				}
				u, _ := m.RimParameters[end][role][i].Float64()
				p, e := m.Source.edges[m.RimEdges[end][role][i]].Curve.Eval(u)
				if e != nil || p.Sub(m.Mesh.Vertices[id]).Length() > 1e-12 {
					t.Fatal("original rim parameter", e)
				}
			}
			if role == 0 {
				area.Add(area, ringArea)
			} else {
				area.Sub(area, ringArea)
			}
		}
		triangles := new(big.Rat)
		for i, tri := range m.Mesh.Triangles {
			if m.OriginalFace[i] != m.History.Fillets.Caps[end] {
				continue
			}
			a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
			v := new(big.Rat).Add(twiceArea(a, b), twiceArea(b, c))
			v.Add(v, twiceArea(c, a))
			triangles.Add(triangles, v)
		}
		triangles.Abs(triangles)
		if triangles.Cmp(area) != 0 {
			t.Fatal("exact cap polygon coverage")
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
	want := arc(r0, M0, -math.Pi/2, theta) + arc(r1, M1, theta, math.Pi/2) + math.Pi*(z1-z0)*(R0*R0+R0*R1+R1*R1)/3 - 6*math.Pi
	volume, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(volume-want) > .01*want {
		t.Fatal("analytic volume", volume, want)
	}
	if m.SquaredDeflection.Cmp(new(big.Rat).Mul(ratInset(.2), ratInset(.2))) > 0 {
		t.Fatal("deflection")
	}
	bad, e := TessellateFilletedFrustumThroughBoreCertified(ctx, s, 15./16, 15./8, geom.Vec2{X: 1, Y: .5}, 1, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 1}})
	if e == nil || len(bad.Mesh.Vertices) != 0 || bad.Source.Topology().Faces != 0 {
		t.Fatal("atomic budget")
	}

}
