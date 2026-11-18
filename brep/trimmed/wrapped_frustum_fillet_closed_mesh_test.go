package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestClosedConicalFilletMesh(t *testing.T) {
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
	m, e := TessellateFilletedFrustumClosedCertified(ctx, s, 15./16, 15./8, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxSubdivisions: 16, MaxTriangleChecks: 30000}, MaxTriangles: 20000})
	if e != nil {
		t.Fatal(e)
	}
	t.Log(len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks)
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if len(m.OriginalFace) != len(m.Mesh.Triangles) || len(m.SquaredErrors) != len(m.Mesh.Triangles) || m.Source.Topology().Faces != 5 {
		t.Fatal("source ownership")
	}
	// Independent analytic meridian integral, not source quadrature or mesh.
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
	exactVolume := arc(r0, M0, -math.Pi/2, theta) + arc(r1, M1, theta, math.Pi/2) + math.Pi*(z1-z0)*(R0*R0+R0*R1+R1*R1)/3
	displayVolume, _ := m.SignedDisplayVolume.Float64()
	if displayVolume <= 0 || math.Abs(displayVolume-exactVolume) > exactVolume*.01 {
		t.Fatal("independent volume", displayVolume, exactVolume)
	}
	tolerance := new(big.Rat).Mul(new(big.Rat).SetFloat64(.2), new(big.Rat).SetFloat64(.2))
	if m.SquaredDeflection.Cmp(tolerance) > 0 || m.TriangleChecks > 30000 {
		t.Fatal("certificate budget")
	}
	type use struct {
		count  int
		owners map[int]bool
	}
	uses := map[[2]int]*use{}
	key := func(a, b int) [2]int {
		if a > b {
			a, b = b, a
		}
		return [2]int{a, b}
	}
	for i, tri := range m.Mesh.Triangles {
		if m.SquaredErrors[i].Cmp(tolerance) > 0 {
			t.Fatal("per-triangle bound")
		}
		for k := 0; k < 3; k++ {
			pair := key(tri[k], tri[(k+1)%3])
			if uses[pair] == nil {
				uses[pair] = &use{owners: map[int]bool{}}
			}
			uses[pair].count++
			uses[pair].owners[m.OriginalFace[i]] = true
		}
	}
	for end, rows := range m.Rims {
		for role, row := range rows {
			for i, a := range row {
				u := uses[key(a, row[(i+1)%len(row)])]
				neighbor := m.History.Caps[end]
				if role == 1 {
					neighbor = m.History.Lateral
				}
				if u == nil || u.count != 2 || !u.owners[m.History.Blends[end]] || !u.owners[neighbor] || len(u.owners) != 2 {
					t.Fatal("shared rim incidence/ancestry", end, role, u)
				}
			}
		}
	}
	coarse, e := TessellateFilletedFrustumClosedCertified(ctx, s, 15./16, 15./8, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .8, MaxSubdivisions: 16, MaxTriangleChecks: 30000}, MaxTriangles: 20000})
	if e != nil {
		t.Fatal(e)
	}
	cv, _ := coarse.SignedDisplayVolume.Float64()
	if len(coarse.Mesh.Triangles) >= len(m.Mesh.Triangles) || math.Abs(cv-exactVolume) <= math.Abs(displayVolume-exactVolume) {
		t.Fatal("independent convergence", cv, displayVolume, exactVolume)
	}
	bad, e := TessellateFilletedFrustumClosedCertified(ctx, s, 15./16, 15./8, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 1}})
	if e == nil || len(bad.Mesh.Vertices) != 0 || bad.Source.Topology().Faces != 0 {
		t.Fatal("atomic failure")
	}

}
