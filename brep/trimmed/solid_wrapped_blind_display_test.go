package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPreparedWrappedBlindDisplayBothEntries(t *testing.T) {
	for _, upper := range []bool{false, true} {
		t.Run(map[bool]string{false: "lower", true: "upper"}[upper], func(t *testing.T) {
			ctx := context.Background()
			embedding := geom.SurfacePatchOptions{}
			outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 2, embedding)
			if e != nil {
				t.Fatal(e)
			}
			outer, _, e = geom.DraftWrappedFrustum(ctx, outer, 0, 1, embedding)
			if e != nil {
				t.Fatal(e)
			}
			z := 0.
			if upper {
				z = 1
			}
			inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25, Z: z}, .5, 1, embedding)
			if e != nil {
				t.Fatal(e)
			}
			body, _, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-8, embedding)
			if e != nil {
				t.Fatal(e)
			}
			faces := body.Faces()
			slices.Reverse(faces)
			body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			var buf bytes.Buffer
			if e = WriteJSON(&buf, body); e != nil {
				t.Fatal(e)
			}
			body, e = ReadJSONContext(ctx, &buf, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			proof, e := body.ValidateCanonicalWrappedFrustumBore(ctx, embedding)
			if e != nil {
				t.Fatal(e)
			}
			floor := proof.InnerFaces[2]
			floorSign := -1.
			if upper {
				floor = proof.InnerFaces[1]
				floorSign = 1
			}
			opts := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .5, MaxTriangleChecks: 30000}, MaxTriangles: 20000}
			val := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000, MaxTriangles: 20000}}
			p, e := body.PrepareWrappedFrustumBlindDisplay(ctx, opts, val)
			if e != nil {
				t.Fatal(e)
			}
			if p.SquaredDistance().Sign() <= 0 || p.SquaredDistance().Cmp(big.NewRat(1, 4)) > 0 {
				t.Fatal("actual distance")
			}
			a := p.Ancestry()
			uses := p.Uses()
			if len(uses) != 36 || len(a.EdgeVertices) != 18 || len(a.CornerVertices) != 16 {
				t.Fatal("source ownership")
			}
			local := p.display.prepared.display.Mesh()
			owners := p.display.prepared.display.Faces()
			edgeSet := map[[2]int]bool{}
			volume := 0.
			floorTriangles := 0
			for i, tr := range local.Triangles {
				x, y, z := local.Vertices[tr[0]], local.Vertices[tr[1]], local.Vertices[tr[2]]
				nz := (y.X-x.X)*(z.Y-x.Y) - (y.Y-x.Y)*(z.X-x.X)
				if owners[i] == floor {
					floorTriangles++
					if x.Z != 1 || y.Z != 1 || z.Z != 1 || floorSign*nz <= 0 {
						t.Fatal("original inward floor orientation", i)
					}
				}
				volume += (x.X*(y.Y*z.Z-y.Z*z.Y) + x.Y*(y.Z*z.X-y.X*z.Z) + x.Z*(y.X*z.Y-y.Y*z.X)) / 6
				for j := 0; j < 3; j++ {
					a, b := tr[j], tr[(j+1)%3]
					if a > b {
						a, b = b, a
					}
					edgeSet[[2]int{a, b}] = true
				}
			}
			if floorTriangles == 0 || len(local.Vertices)-len(edgeSet)+len(local.Triangles) != 2 {
				t.Fatal("closed blind topology")
			}
			truth := 149 * math.Pi / 12
			if math.Abs(volume-truth) > truth*.04 {
				t.Fatal("analytic retained volume", volume, truth)
			}
			for edge, row := range a.EdgeVertices {
				for j, id := range row {
					q, _ := a.EdgeParameters[edge][j].Float64()
					want, e := body.edges[edge].Curve.Eval(q)
					if e != nil {
						t.Fatal(e)
					}
					got := local.Vertices[id]
					if math.Abs(want.X-got.X)+math.Abs(want.Y-got.Y)+math.Abs(want.Z-got.Z) > 1e-12 {
						t.Fatal("original parameter sample", edge, j)
					}
				}
			}
			floorUses := 0
			for _, u := range uses {
				f := body.faces[u.Face]
				loops := append([][]Use{f.Outer}, f.Holes...)
				original := loops[u.Loop][u.Use]
				if original.Edge != u.Edge || original.Reversed != u.Reversed || f.Reversed != u.FaceReversed || !reflect.DeepEqual(a.EdgeVertices[u.Edge], u.Vertices) {
					t.Fatal("original coedge identity")
				}
				if u.Face == floor {
					floorUses++
				}
			}
			if floorUses != 4 {
				t.Fatal("floor rim ownership")
			}
			world := assemblyOccurrence(t, p.display.prepared, big.NewRat(1, 10), big.NewRat(-2, 5), big.NewRat(3, 8), true)
			placed, e := p.Place(ctx, world.Display().Pose(), val)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(placed.Ancestry(), a) || !reflect.DeepEqual(placed.Display().Mesh(), world.Display().Mesh()) {
				t.Fatal("world ownership")
			}
			uses[0].Vertices[0] = -1
			uses[0].ModelParameters[0].SetInt64(999)
			if p.Uses()[0].Vertices[0] < 0 || p.Uses()[0].ModelParameters[0].Cmp(big.NewRat(999, 1)) == 0 {
				t.Fatal("mutable result")
			}
			bad := body
			bad.faces = body.Faces()
			bad.faces[floor].Surface = body.faces[proof.OuterFaces[1]].Surface
			if denied, e := bad.PrepareWrappedFrustumBlindDisplay(ctx, opts, val); e == nil || denied != nil {
				t.Fatal("forged floor accepted")
			}
			bad = body
			bad.faces = body.Faces()
			for i := range bad.faces {
				if len(bad.faces[i].Holes) > 0 {
					bad.faces[i].Holes = nil
					break
				}
			}
			if denied, e := bad.PrepareWrappedFrustumBlindDisplay(ctx, opts, val); e == nil || denied != nil {
				t.Fatal("missing opening trim accepted")
			}
			if denied, e := body.PrepareWrappedFrustumThroughDisplay(ctx, opts, val); e == nil || denied != nil {
				t.Fatal("through domain widened")
			}
			opts.MaxTriangles = 1
			if denied, e := body.PrepareWrappedFrustumBlindDisplay(ctx, opts, val); e == nil || denied != nil {
				t.Fatal("mesh quota")
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if denied, e := body.PrepareWrappedFrustumBlindDisplay(cancelled, opts, val); e == nil || denied != nil {
				t.Fatal("cancel")
			}
			t.Log("triangles", len(local.Triangles), "floor triangles", floorTriangles, "volume", volume, "ideal", truth)
		})
	}
}
