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

func TestPreparedWrappedStockDisplay(t *testing.T) {
	for _, upper := range []float64{2, 3} {
		t.Run(map[float64]string{2: "descending-frustum", 3: "cylinder"}[upper], func(t *testing.T) {
			ctx := context.Background()
			embedding := geom.SurfacePatchOptions{}
			w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 2, embedding)
			if e != nil {
				t.Fatal(e)
			}
			w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, upper-3, embedding)
			if e != nil {
				t.Fatal(e)
			}
			body, _, e := NewWrappedFrustum(ctx, w, 1e-8, embedding)
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
			opts := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 30000}, MaxTriangles: 20000}
			validation := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxTriangles: 20000, MaxCollisionPairs: 1000000}}
			p, e := body.PrepareWrappedFrustumStockDisplay(ctx, opts, validation)
			if e != nil {
				t.Fatal(e)
			}
			if p.SquaredDistance().Sign() <= 0 || p.SquaredDistance().Cmp(big.NewRat(1, 4)) > 0 {
				t.Fatal("actual source error")
			}
			a, uses := p.Ancestry(), p.Uses()
			if len(a.EdgeVertices) != 9 || len(a.CornerVertices) != 8 || len(uses) != 18 {
				t.Fatal("original ownership")
			}
			local := p.display.prepared.display.Mesh()
			owners := p.display.prepared.display.Faces()
			seen := map[int]int{}
			edgeSet := map[[2]int]bool{}
			volume := 0.
			for i, tr := range local.Triangles {
				x, y, z := local.Vertices[tr[0]], local.Vertices[tr[1]], local.Vertices[tr[2]]
				nz := (y.X-x.X)*(z.Y-x.Y) - (y.Y-x.Y)*(z.X-x.X)
				owner := owners[i]
				seen[owner]++
				// Reversed face order: upper cap 0, lower cap 1, original wall 2.
				if owner == 0 && (x.Z != 2 || y.Z != 2 || z.Z != 2 || nz <= 0) {
					t.Fatal("upper cap source")
				}
				if owner == 1 && (x.Z != 0 || y.Z != 0 || z.Z != 0 || nz >= 0) {
					t.Fatal("lower cap source")
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
			if len(seen) != 3 || len(local.Vertices)-len(edgeSet)+len(local.Triangles) != 2 {
				t.Fatal("closed original faces")
			}
			truth := 2 * (9 + 3*upper + upper*upper) * math.Pi / 3
			if math.Abs(volume-truth) > truth*.04 {
				t.Fatal("independent conical integral", volume, truth)
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
						t.Fatal("model sample", edge, j)
					}
				}
			}
			for _, u := range uses {
				original := body.faces[u.Face].Outer[u.Use]
				if u.Loop != 0 || original.Edge != u.Edge || original.Reversed != u.Reversed || body.faces[u.Face].Reversed != u.FaceReversed || !reflect.DeepEqual(u.Vertices, a.EdgeVertices[u.Edge]) {
					t.Fatal("original occurrence")
				}
			}
			world := assemblyOccurrence(t, p.display.prepared, big.NewRat(1, 10), big.NewRat(-2, 5), big.NewRat(3, 8), true)
			actual, e := p.Place(ctx, world.Display().Pose(), validation)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(actual.Display().Mesh(), world.Display().Mesh()) || !reflect.DeepEqual(actual.Ancestry(), a) {
				t.Fatal("world identity")
			}
			uses[0].ModelParameters[0].SetInt64(999)
			uses[0].Vertices[0] = -1
			if p.Uses()[0].Vertices[0] < 0 || p.Uses()[0].ModelParameters[0].Cmp(big.NewRat(999, 1)) == 0 {
				t.Fatal("mutable result")
			}
			bad := body
			bad.faces = body.Faces()
			bad.faces[0].Surface = bad.faces[1].Surface
			if denied, e := bad.PrepareWrappedFrustumStockDisplay(ctx, opts, validation); e == nil || denied != nil {
				t.Fatal("forged cap accepted")
			}
			bad = body
			bad.faces = body.Faces()
			bad.faces[2].Outer = bad.faces[2].Outer[:9]
			if denied, e := bad.PrepareWrappedFrustumStockDisplay(ctx, opts, validation); e == nil || denied != nil {
				t.Fatal("missing seam accepted")
			}
			opts.MaxTriangles = 1
			if denied, e := body.PrepareWrappedFrustumStockDisplay(ctx, opts, validation); e == nil || denied != nil {
				t.Fatal("mesh quota")
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if denied, e := body.PrepareWrappedFrustumStockDisplay(cancelled, opts, validation); e == nil || denied != nil {
				t.Fatal("cancel")
			}
			t.Log("triangles", len(local.Triangles), "volume", volume, "ideal", truth)
		})
	}
}
