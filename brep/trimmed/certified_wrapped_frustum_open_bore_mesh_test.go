package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestCertifiedWrappedFrustumOpenBoreMesh(t *testing.T) {
	ctx := context.Background()
	patch := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: -1, Y: 2, Z: 3}, 2, 4, patch)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, patch)
	if e != nil {
		t.Fatal(e)
	}
	for _, upper := range []bool{false, true} {
		tool, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, stock, .5, upper, patch)
		if e != nil {
			t.Fatal(e)
		}
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .15}}
		m, e := TessellateWrappedFrustumOpenBoreNormalsCertified(ctx, stock, tool, .4, o)
		if e != nil {
			t.Fatal(upper, "closed open-ended wall", e)
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.Mesh.Triangles) != len(m.OriginalFace) ||
			len(m.SquaredErrors) != len(m.Mesh.Triangles) || m.SourceImageSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.15), ratInset(.15))) > 0 {
			t.Fatal("incomplete display evidence")
		}
		if m.SquaredNormalTangent.Sign() <= 0 || m.SquaredNormalTangent.Cmp(new(big.Rat).Mul(ratInset(.4), ratInset(.4))) > 0 {
			t.Fatal("continuous material normal certificate", m.SquaredNormalTangent)
		}
		cap := 0
		a, b := 11.0/8, 4.0
		if upper {
			cap, a, b = 1, 7.0/4, 35.0/8
		}
		owners := [5]int{}
		incident := make(map[[2]int]int)
		for i, tri := range m.Mesh.Triangles {
			owner := m.OriginalFace[i]
			owners[owner]++
			if owner == 4 {
				a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
				n := b.Sub(a).Cross(c.Sub(a))
				if n.Z > 0 != upper || n.X != 0 || n.Y != 0 {
					t.Fatal("blind floor normal points away from the opening", n, upper)
				}
			}
			if cap+1 == owner {
				for j, x := range tri {
					y := tri[(j+1)%3]
					key := [2]int{min(x, y), max(x, y)}
					incident[key]++
				}
			}
		}
		for wall := range 2 {
			row := m.RimVertices[wall][cap]
			for i, x := range row {
				y := row[(i+1)%len(row)]
				if incident[[2]int{min(x, y), max(x, y)}] != 1 {
					t.Fatal("opening did not use the exact display rim edge", wall, i)
				}
			}
		}
		if owners[0] == 0 || owners[1] == 0 || owners[2] == 0 || owners[3] == 0 || owners[4] == 0 ||
			owners[cap+1] != 2*len(m.RimVertices[0][cap]) {
			t.Fatal("lost native material faces", owners)
		}
		want := math.Pi * (52 - 3.5*(a*a+a*b+b*b)/3)
		volume, _ := m.SignedDisplayVolume.Float64()
		if volume <= 0 || math.Abs(volume-want) > .04*want {
			t.Fatal("analytic frustum wall volume", upper, volume, want)
		}
		if bad, e := TessellateWrappedFrustumOpenBoreCertified(ctx, stock, tool, CertifiedWrappedFrustumMeshOptions{
			Patch: CertifiedPatchMeshOptions{Deflection: .15, MaxTriangleChecks: 1},
		}); !errors.Is(e, geom.ErrConvergence) || len(bad.Mesh.Vertices) != 0 {
			t.Fatal("triangle budget returned partial geometry", e)
		}
	}
}

func TestCertifiedPlacedFrustumOpenBore(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, opts)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 3, 0, opts)
	if e != nil {
		t.Fatal(e)
	}
	tool, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, stock, .5, false, opts)
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .15}}
	pose, e := placement.New([3]int{-3, 1, -2}, geom.Vec3{X: 8})
	if e != nil {
		t.Fatal(e)
	}
	m, e := TessellatePlacedWrappedFrustumOpenBoreCertified(ctx, stock, tool, pose, .4, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range m.Mesh.Vertices {
		q := m.Local.Mesh.Vertices[i]
		if want := (geom.Vec3{X: 8 - q.Z, Y: q.X, Z: -q.Y}); p != want {
			t.Fatal("world display differs from the signed coordinate map", p, want)
		}
	}
	for i, t0 := range m.Mesh.Triangles {
		if t0 != m.Local.Mesh.Triangles[i] {
			t.Fatal("changed source incidence")
		}
		if m.Local.OriginalFace[i] == 4 {
			a, b, c := m.Mesh.Vertices[t0[0]], m.Mesh.Vertices[t0[1]], m.Mesh.Vertices[t0[2]]
			n := b.Sub(a).Cross(c.Sub(a))
			if n.X <= 0 || n.Y != 0 || n.Z != 0 {
				t.Fatal("placed floor must face toward the opening", n)
			}
		}
	}
	report, e := collision.SelfIntersections(m.Mesh, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 || m.Mesh.ValidateSolid() != nil ||
		math.Abs(m.Mesh.SignedVolume()-m.Local.Mesh.SignedVolume()) > 1e-10 {
		t.Fatal("independent placed contact, closure, and signed volume checks", e)
	}
	inexact, e := placement.New([3]int{1, 2, 3}, geom.Vec3{Z: 1e20})
	if e != nil {
		t.Fatal(e)
	}
	if bad, e := TessellatePlacedWrappedFrustumOpenBoreCertified(ctx, stock, tool, inexact, .4, o); !errors.Is(e, geom.ErrUnrepresentable) ||
		len(bad.Mesh.Vertices) != 0 || len(bad.Local.Mesh.Vertices) != 0 {
		t.Fatal("failed placement exposed partial mesh", e)
	}
}

func TestCertifiedEccentricFrustumOpenBore(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, opts)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, opts)
	if e != nil {
		t.Fatal(e)
	}
	tool, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, stock, .5, true, opts)
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Identity()
	pose[0][3], pose[1][3] = .25, .25
	tool, _, e = tool.Transformed(ctx, pose, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .15}}
	m, e := TessellateWrappedEccentricFrustumOpenBoreNormalsCertified(ctx, stock, tool, .4, o)
	if e != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || m.Mesh.ValidateSolid() != nil {
		t.Fatal("eccentric complete display", e)
	}
	// The original seam points on the two circles are retained independently.
	witness := [2]geom.Vec3{{X: 5, Z: 4}, {X: 37.0 / 8, Y: .25, Z: 4}}
	for i, p := range witness {
		if m.Mesh.Vertices[m.RimVertices[i][1][0]] != p {
			t.Fatal("eccentric capacity shifted a source rim", i, m.Mesh.Vertices[m.RimVertices[i][1][0]], p)
		}
	}
	want := math.Pi * (52 - 3.5*(1.75*1.75+1.75*4.375+4.375*4.375)/3)
	v, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(v-want) > .04*want {
		t.Fatal("eccentric cup volume", v, want)
	}
	if bad, e := TessellateWrappedFrustumOpenBoreCertified(ctx, stock, tool, o); !errors.Is(e, geom.ErrConvergence) || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("legacy coaxial scope must stay strict", e)
	}
}
