//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPreparedWrappedStationBoreDisplayUnequalPartitions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	embedding := geom.SurfacePatchOptions{}
	outer := []WrappedLoftStation{{Z: -2, Radius: 2}, {Z: 0, Radius: 2.5}, {Z: 3, Radius: 1.5}}
	inner := []WrappedLoftStation{{Z: -2, Radius: .75}, {Z: -.5, Radius: 1}, {Z: 1, Radius: .5}, {Z: 3, Radius: .75}}
	a, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .125, Y: -.25}, outer, 1e-8, embedding)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .125, Y: -.25}, inner, 1e-8, embedding)
	if e != nil {
		t.Fatal(e)
	}
	body, _, e := CombineCoaxialWrappedStationLofts(ctx, a, b, parametric.Difference, embedding)
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
	opts := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .3, MaxTriangleChecks: 50000}, MaxTriangles: 20000}}
	val := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxTriangles: 20000, MaxCollisionPairs: 1000000}}
	p, e := body.PrepareWrappedStationBoreDisplay(ctx, opts, val)
	if e != nil {
		t.Fatal(e)
	}
	ancestry, uses, walls, caps := p.Ancestry(), p.Uses(), p.Walls(), p.Caps()
	if len(walls[0]) != 2 || len(walls[1]) != 3 || len(uses) != 66 || len(ancestry.EdgeVertices) != 33 || len(ancestry.CornerVertices) != 28 {
		t.Fatal("original unequal families")
	}
	local := p.display.prepared.display.Mesh()
	owners := p.display.prepared.display.Faces()
	baseline, e := body.TessellateWrappedStationBoreCertified(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(local, baseline.Mesh) || !reflect.DeepEqual(owners, baseline.OriginalFace) || p.SquaredDistance().Cmp(baseline.SourceImageSquaredDistance) != 0 {
		t.Fatal("capture changed certificate")
	}
	wallSign := map[int]float64{}
	for family, row := range walls {
		sign := 1.
		if family == 1 {
			sign = -1
		}
		for _, id := range row {
			wallSign[id] = sign
		}
	}
	edgeOwners := map[[2]int]map[int]bool{}
	volume := 0.
	for i, tr := range local.Triangles {
		x, y, z := local.Vertices[tr[0]], local.Vertices[tr[1]], local.Vertices[tr[2]]
		nx := (y.Y-x.Y)*(z.Z-x.Z) - (y.Z-x.Z)*(z.Y-x.Y)
		ny := (y.Z-x.Z)*(z.X-x.X) - (y.X-x.X)*(z.Z-x.Z)
		nz := (y.X-x.X)*(z.Y-x.Y) - (y.Y-x.Y)*(z.X-x.X)
		owner := owners[i]
		if sign, ok := wallSign[owner]; ok && sign*(nx*((x.X+y.X+z.X)/3-.125)+ny*((x.Y+y.Y+z.Y)/3+.25)) <= 0 {
			t.Fatal("original outward/inward wall orientation")
		}
		if owner == caps[0] && (x.Z != -2 || y.Z != -2 || z.Z != -2 || nz >= 0) {
			t.Fatal("lower annulus")
		}
		if owner == caps[1] && (x.Z != 3 || y.Z != 3 || z.Z != 3 || nz <= 0) {
			t.Fatal("upper annulus")
		}
		volume += (x.X*(y.Y*z.Z-y.Z*z.Y) + x.Y*(y.Z*z.X-y.X*z.Z) + x.Z*(y.X*z.Y-y.Y*z.X)) / 6
		for j := 0; j < 3; j++ {
			a, b := tr[j], tr[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if edgeOwners[key] == nil {
				edgeOwners[key] = map[int]bool{}
			}
			edgeOwners[key][owner] = true
		}
	}
	if len(local.Vertices)-len(edgeOwners)+len(local.Triangles) != 0 {
		t.Fatal("hollow genus one")
	}
	truth := 627 * math.Pi / 32
	if math.Abs(volume-truth) > truth*.04 {
		t.Fatal("independent outer minus inner conical integrals", volume, truth)
	}
	sourceOwners := map[int]map[int]bool{}
	for _, u := range uses {
		f := body.faces[u.Face]
		loops := append([][]Use{f.Outer}, f.Holes...)
		original := loops[u.Loop][u.Use]
		if original.Edge != u.Edge || original.Reversed != u.Reversed || f.Reversed != u.FaceReversed || !reflect.DeepEqual(u.Vertices, ancestry.EdgeVertices[u.Edge]) {
			t.Fatal("source occurrence")
		}
		if sourceOwners[u.Edge] == nil {
			sourceOwners[u.Edge] = map[int]bool{}
		}
		sourceOwners[u.Edge][u.Face] = true
	}
	stationEdges := 0
	for edge, row := range ancestry.EdgeVertices {
		wantOwners := sourceOwners[edge]
		allWalls := len(wantOwners) == 2
		for face := range wantOwners {
			if _, ok := wallSign[face]; !ok {
				allWalls = false
			}
		}
		if allWalls {
			stationEdges++
		}
		for j, id := range row {
			q, _ := ancestry.EdgeParameters[edge][j].Float64()
			want, e := body.edges[edge].Curve.Eval(q)
			if e != nil {
				t.Fatal(e)
			}
			got := local.Vertices[id]
			if math.Abs(want.X-got.X)+math.Abs(want.Y-got.Y)+math.Abs(want.Z-got.Z) > 1e-12 {
				t.Fatal("original edge parameter", edge, j)
			}
			if j > 0 {
				x, y := row[j-1], id
				if x > y {
					x, y = y, x
				}
				if !reflect.DeepEqual(edgeOwners[[2]int{x, y}], wantOwners) {
					t.Fatal("shared original incidence", edge, j)
				}
			}
		}
	}
	if stationEdges != 12 {
		t.Fatal("original unequal station boundaries", stationEdges)
	}
	world := assemblyOccurrence(t, p.display.prepared, big.NewRat(1, 10), big.NewRat(-2, 5), big.NewRat(3, 8), true)
	actual, e := p.Place(ctx, world.Display().Pose(), val)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(actual.Ancestry(), ancestry) || !reflect.DeepEqual(actual.Display().Mesh(), world.Display().Mesh()) {
		t.Fatal("world ownership")
	}
	walls[1][0] = -1
	uses[0].Vertices[0] = -1
	if p.Walls()[1][0] < 0 || p.Uses()[0].Vertices[0] < 0 {
		t.Fatal("mutable proof")
	}
	bad := body
	bad.faces = body.Faces()
	bad.faces[caps[0]].Holes = nil
	if denied, e := bad.PrepareWrappedStationBoreDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("missing annulus hole")
	}
	bad = body
	bad.faces = body.Faces()
	bad.faces[p.Walls()[1][0]].Reversed = false
	if denied, e := bad.PrepareWrappedStationBoreDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("wrong inner orientation")
	}
	opts.MaxStationSamples = 1
	if denied, e := body.PrepareWrappedStationBoreDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("cumulative station quota")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if denied, e := body.PrepareWrappedStationBoreDisplay(cancelled, opts, val); e == nil || denied != nil {
		t.Fatal("cancel")
	}
	t.Log("triangles", len(local.Triangles), "volume", volume, "ideal", truth, "checks", baseline.TriangleChecks)
}
