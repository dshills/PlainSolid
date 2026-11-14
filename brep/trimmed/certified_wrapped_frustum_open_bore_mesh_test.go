package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

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
		m, e := TessellateWrappedFrustumOpenBoreCertified(ctx, stock, tool, o)
		if e != nil {
			t.Fatal(upper, "closed open-ended wall", e)
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) != 0 || len(m.Mesh.Triangles) != len(m.OriginalFace) ||
			len(m.SquaredErrors) != len(m.Mesh.Triangles) || m.SourceImageSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.15), ratInset(.15))) > 0 {
			t.Fatal("incomplete display evidence")
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
