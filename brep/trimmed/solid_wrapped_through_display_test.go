package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"slices"
	"testing"
)

func TestPreparedWrappedThroughDisplay(t *testing.T) {
	ctx := context.Background()
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 2, 2, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	outer, _, e = geom.DraftWrappedFrustum(ctx, outer, 0, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: .25, Y: .25}, .5, 2, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	body, _, e := NewWrappedEccentricCircularFrustumBore(ctx, outer, inner, 1e-8, geom.SurfacePatchOptions{})
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
	opts := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .5, MaxTriangleChecks: 30000}, MaxTriangles: 20000}
	validation := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000, MaxTriangles: 20000}}
	p, e := body.PrepareWrappedFrustumThroughDisplay(ctx, opts, validation)
	if e != nil {
		t.Fatal(e)
	}
	if p.SquaredDistance().Sign() <= 0 || p.SquaredDistance().Cmp(big.NewRat(1, 4)) > 0 {
		t.Fatal("missing actual source deviation")
	}
	a := p.Ancestry()
	uses := p.Uses()
	if len(uses) != 36 || len(a.EdgeVertices) != 18 || len(a.CornerVertices) != 16 {
		t.Fatal("ownership", len(uses), len(a.EdgeVertices), len(a.CornerVertices))
	}
	for _, u := range uses {
		edge := body.edges[u.Edge]
		if !reflect.DeepEqual(u.Vertices, a.EdgeVertices[u.Edge]) || u.ModelParameters[0].Cmp(ratInset(edge.Range.Min)) != 0 || u.ModelParameters[len(u.ModelParameters)-1].Cmp(ratInset(edge.Range.Max)) != 0 {
			t.Fatal("source range")
		}
		f := body.faces[u.Face]
		loops := append([][]Use{f.Outer}, f.Holes...)
		if loops[u.Loop][u.Use].Edge != u.Edge || loops[u.Loop][u.Use].Reversed != u.Reversed || f.Reversed != u.FaceReversed {
			t.Fatal("occurrence")
		}
	}
	local := p.prepared.display.Mesh()
	for edge, row := range a.EdgeVertices {
		for j, id := range row {
			parameter, _ := a.EdgeParameters[edge][j].Float64()
			point, e := body.edges[edge].Curve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			got := local.Vertices[id]
			if math.Abs(point.X-got.X)+math.Abs(point.Y-got.Y)+math.Abs(point.Z-got.Z) > 1e-12 {
				t.Fatal("sample not on original model edge", edge, j, point, got)
			}
		}
	}
	uses[0].ModelParameters[0].SetInt64(999)
	uses[0].Vertices[0] = -1
	if p.Uses()[0].Vertices[0] < 0 || p.Uses()[0].ModelParameters[0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable proof")
	}
	world := assemblyOccurrence(t, p.prepared, big.NewRat(1, 10), big.NewRat(-2, 5), big.NewRat(3, 8), true)
	if !reflect.DeepEqual(world.Ancestry(), a) || len(world.Display().Shells()) != 1 {
		t.Fatal("placed ancestry")
	}
	m := world.Display().Mesh()
	edges := map[[2]int]bool{}
	volume := 0.0
	for _, tr := range m.Triangles {
		a, b, c := m.Vertices[tr[0]], m.Vertices[tr[1]], m.Vertices[tr[2]]
		volume += (a.X*(b.Y*c.Z-b.Z*c.Y) + a.Y*(b.Z*c.X-b.X*c.Z) + a.Z*(b.X*c.Y-b.Y*c.X)) / 6
		for i := 0; i < 3; i++ {
			x, y := tr[i], tr[(i+1)%3]
			if x > y {
				x, y = y, x
			}
			edges[[2]int{x, y}] = true
		}
	}
	if len(m.Vertices)-len(edges)+len(m.Triangles) != 0 {
		t.Fatal("not genus one")
	}
	truth := 73 * math.Pi / 6
	if math.Abs(volume-truth) > truth*.12 {
		t.Fatal("independent frustum minus cylinder volume", volume, truth)
	}
	t.Log("triangles", len(m.Triangles), "volume", volume, "ideal", truth)
	// Authentic source admission must reject a changed cap hole even when the
	// caller retains the same analytic surfaces and the same source edge IDs.
	bad := body
	bad.faces = body.Faces()
	for i := range bad.faces {
		if len(bad.faces[i].Holes) > 0 {
			bad.faces[i].Holes = nil
			break
		}
	}
	if denied, e := bad.PrepareWrappedFrustumThroughDisplay(ctx, opts, validation); e == nil || denied != nil {
		t.Fatal("missing original hole accepted")
	}
	bad = body
	bad.faces = body.Faces()
	for i := range bad.faces {
		if bad.faces[i].Reversed {
			bad.faces[i].Reversed = false
			break
		}
	}
	if denied, e := bad.PrepareWrappedFrustumThroughDisplay(ctx, opts, validation); e == nil || denied != nil {
		t.Fatal("wrong inward ownership accepted")
	}
	if denied, e := p.Place(ctx, world.Display().Pose(), placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}); e == nil || denied != nil {
		t.Fatal("placed proof quota bypass")
	}
	opts.MaxTriangles = 1
	if denied, e := body.PrepareWrappedFrustumThroughDisplay(ctx, opts, validation); e == nil || denied != nil {
		t.Fatal("non-atomic mesh quota")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if denied, e := body.PrepareWrappedFrustumThroughDisplay(cancelled, opts, validation); e == nil || denied != nil {
		t.Fatal("cancel")
	}
}
