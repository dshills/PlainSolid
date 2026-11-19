//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestCertifiedEccentricFrustumThroughBoreMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	embedding := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, embedding)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, embedding)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25}, 1, 4, embedding)
	if e != nil {
		t.Fatal(e)
	}
	body, source, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, tool, 1e-8, embedding)
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .15}}
	m, e := body.TessellateCanonicalWrappedFrustumThroughBoreNormalsCertified(ctx, .4, o)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.OriginalFace) != len(m.Mesh.Triangles) ||
		len(m.SquaredErrors) != len(m.Mesh.Triangles) ||
		m.SourceImageSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.15), ratInset(.15))) > 0 ||
		m.SquaredNormalTangent.Sign() <= 0 || m.SquaredNormalTangent.Cmp(new(big.Rat).Mul(ratInset(.4), ratInset(.4))) > 0 {
		t.Fatal("incomplete through-bore certificate")
	}
	id := func(i int) int { return len(faces) - 1 - i }
	endOwners := [2]int{id(source.OuterFaces[1]), id(source.OuterFaces[2])}
	counts := [4]int{}
	allEdges := make(map[[2]int]bool)
	capEdges := [2]map[[2]int]int{make(map[[2]int]int), make(map[[2]int]int)}
	for i, tri := range m.Mesh.Triangles {
		owner := m.OriginalFace[i]
		counts[owner]++
		end := slices.Index(endOwners[:], owner)
		if end >= 0 {
			a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
			n := b.Sub(a).Cross(c.Sub(a))
			if n.X != 0 || n.Y != 0 || n.Z == 0 || (n.Z > 0) != (end == 1) {
				t.Fatal("annular cap material orientation", end, n)
			}
		}
		for j, a := range tri {
			b := tri[(j+1)%3]
			key := [2]int{min(a, b), max(a, b)}
			allEdges[key] = true
			if end >= 0 {
				capEdges[end][key]++
			}
		}
	}
	if len(m.Mesh.Vertices)-len(allEdges)+len(m.Mesh.Triangles) != 0 ||
		counts[id(source.OuterFaces[0])] == 0 || counts[id(source.InnerFaces[0])] == 0 {
		t.Fatal("through bore must have genus one and both native wall owners", counts)
	}
	for end, owner := range endOwners {
		if counts[owner] != 2*len(m.RimVertices[0][end]) {
			t.Fatal("annulus contains a disk fan or lost a facet", end, counts)
		}
		for wall := range 2 {
			row := m.RimVertices[wall][end]
			for j, a := range row {
				b := row[(j+1)%len(row)]
				if capEdges[end][[2]int{min(a, b), max(a, b)}] != 1 {
					t.Fatal("annulus did not share an original rim edge", end, wall, j)
				}
			}
		}
	}
	volume, _ := m.SignedDisplayVolume.Float64()
	want := 48 * math.Pi // frustum radii 2 and 5, minus the radius 1 cylinder
	if math.Abs(volume-want) > .04*want {
		t.Fatal("display volume differs from the independent analytic volume", volume, want)
	}
	o.Patch.MaxTriangleChecks = 1
	if q, e := body.TessellateCanonicalWrappedFrustumThroughBoreCertified(ctx, o); !errors.Is(e, geom.ErrConvergence) ||
		len(q.Mesh.Vertices) != 0 || len(q.OriginalFace) != 0 {
		t.Fatal("through-bore budget published a partial certificate", e)
	}
}

func TestCertifiedEccentricFrustumCavityMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	embedding := geom.SurfacePatchOptions{}
	base, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 4, embedding)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := geom.DraftWrappedFrustum(ctx, base, 0, 3, embedding)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: .5}, 1, 3, embedding)
	if e != nil {
		t.Fatal(e)
	}
	body, source, e := NewWrappedEccentricCircularFrustumBore(ctx, stock, tool, 1e-8, embedding)
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .15}}
	m, e := body.TessellateCanonicalWrappedFrustumCavityNormalsCertified(ctx, .4, o)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.CavityShells) != 2 ||
		m.CavityShells[0].Depth != 0 || m.CavityShells[1].Parent != 0 || m.CavityShells[1].Depth != 1 ||
		m.CavityShells[0].SignedVolume.Sign() != 1 || m.CavityShells[1].SignedVolume.Sign() != -1 {
		t.Fatal("display did not prove strictly nested shells", m.CavityShells, m.Collision)
	}
	id := func(i int) int { return len(faces) - 1 - i }
	for i, shell := range m.CavityShells {
		from := source.OuterFaces
		if i == 1 {
			from = source.InnerFaces
		}
		owners := [3]int{id(from[0]), id(from[1]), id(from[2])}
		var counts [3]int
		for _, j := range shell.Triangles {
			owner := slices.Index(owners[:], m.OriginalFace[j])
			if owner < 0 {
				t.Fatal("face owner on wrong native shell", j, m.OriginalFace[j], owners)
			}
			counts[owner]++
			if owner == 0 {
				continue
			}
			tri := m.Mesh.Triangles[j]
			a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
			n := b.Sub(a).Cross(c.Sub(a))
			if n.X != 0 || n.Y != 0 || n.Z == 0 || (n.Z > 0) != ((owner == 2) != (i == 1)) {
				t.Fatal("native cap material orientation", i, owner, n)
			}
		}
		if counts[0] == 0 || counts[1] == 0 || counts[2] == 0 {
			t.Fatal("incomplete native material face ownership", counts)
		}
	}
	volume, _ := m.SignedDisplayVolume.Float64()
	want := 49 * math.Pi
	if math.Abs(volume-want) > .04*want {
		t.Fatal("display volume disagrees with the outer frustum minus enclosed cutter", volume, want)
	}
	if r, e := body.TessellateCanonicalWrappedFrustumThroughBoreCertified(ctx, o); !errors.Is(e, geom.ErrConvergence) || len(r.Mesh.Vertices) != 0 {
		t.Fatal("a fully enclosed cavity was mistaken for a through bore", e)
	}
}

func TestCertifiedWrappedFrustumOpenBoreMesh(t *testing.T) {
	slowtest.Skip(t)
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
	slowtest.Skip(t)
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
	slowtest.Skip(t)
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
