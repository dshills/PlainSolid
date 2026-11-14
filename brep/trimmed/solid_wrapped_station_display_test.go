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

func TestPreparedWrappedStationDisplayCrease(t *testing.T) {
	ctx := context.Background()
	embedding := geom.SurfacePatchOptions{}
	stations := []WrappedLoftStation{{Z: 0, Radius: 3}, {Z: 1, Radius: 1}, {Z: 3, Radius: 2}}
	body, _, e := LoftWrappedCircularStations(ctx, geom.Vec2{X: .25, Y: -.5}, stations, 1e-8, embedding)
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
	opts := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 30000}, MaxTriangles: 20000}}
	val := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxTriangles: 20000, MaxCollisionPairs: 1000000}}
	p, e := body.PrepareWrappedStationDisplay(ctx, opts, val)
	if e != nil {
		t.Fatal(e)
	}
	a, uses, h := p.Ancestry(), p.Uses(), p.Stations()
	if len(uses) != 28 || len(a.EdgeVertices) != 14 || len(a.CornerVertices) != 12 || len(h.WallFaces) != 2 || len(h.StationRims) != 3 {
		t.Fatal("original station ownership")
	}
	local := p.display.prepared.display.Mesh()
	owners := p.display.prepared.display.Faces()
	baseline, e := body.TessellateWrappedStationLoftCertified(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(local, baseline.Mesh) || !reflect.DeepEqual(owners, baseline.OriginalFace) || p.SquaredDistance().Cmp(baseline.SourceImageSquaredDistance) != 0 {
		t.Fatal("capture changed prior certificate")
	}
	edgeOwners := map[[2]int]map[int]bool{}
	volume := 0.
	for i, tr := range local.Triangles {
		x, y, z := local.Vertices[tr[0]], local.Vertices[tr[1]], local.Vertices[tr[2]]
		nx := (y.Y-x.Y)*(z.Z-x.Z) - (y.Z-x.Z)*(z.Y-x.Y)
		ny := (y.Z-x.Z)*(z.X-x.X) - (y.X-x.X)*(z.Z-x.Z)
		nz := (y.X-x.X)*(z.Y-x.Y) - (y.Y-x.Y)*(z.X-x.X)
		owner := owners[i]
		if owner == h.CapFaces[0] && (x.Z != 0 || y.Z != 0 || z.Z != 0 || nz >= 0) {
			t.Fatal("lower original cap")
		}
		if owner == h.CapFaces[1] && (x.Z != 3 || y.Z != 3 || z.Z != 3 || nz <= 0) {
			t.Fatal("upper original cap")
		}
		for band, face := range h.WallFaces {
			if owner == face {
				if nx*((x.X+y.X+z.X)/3-.25)+ny*((x.Y+y.Y+z.Y)/3+.5) <= 0 {
					t.Fatal("inward original wall")
				}
				if band == 0 && nz <= 0 || band == 1 && nz >= 0 {
					t.Fatal("lost physical slope crease")
				}
			}
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
	if len(local.Vertices)-len(edgeOwners)+len(local.Triangles) != 2 {
		t.Fatal("closed station topology")
	}
	truth := 9 * math.Pi
	if math.Abs(volume-truth) > truth*.04 {
		t.Fatal("independent piecewise frustum integral", volume, truth)
	}
	for _, edge := range h.StationRims[1] {
		row := a.EdgeVertices[edge]
		for i := 1; i < len(row); i++ {
			x, y := row[i-1], row[i]
			if x > y {
				x, y = y, x
			}
			got := edgeOwners[[2]int{x, y}]
			if len(got) != 2 || !got[h.WallFaces[0]] || !got[h.WallFaces[1]] {
				t.Fatal("station not shared by both original bands")
			}
		}
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
				t.Fatal("original model parameter", edge, j)
			}
		}
	}
	for _, u := range uses {
		original := body.faces[u.Face].Outer[u.Use]
		if u.Loop != 0 || original.Edge != u.Edge || original.Reversed != u.Reversed || body.faces[u.Face].Reversed != u.FaceReversed || !reflect.DeepEqual(a.EdgeVertices[u.Edge], u.Vertices) {
			t.Fatal("original coedge")
		}
	}
	world := assemblyOccurrence(t, p.display.prepared, big.NewRat(1, 10), big.NewRat(-2, 5), big.NewRat(3, 8), true)
	actual, e := p.Place(ctx, world.Display().Pose(), val)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(actual.Ancestry(), a) || !reflect.DeepEqual(actual.Display().Mesh(), world.Display().Mesh()) {
		t.Fatal("world source binding")
	}
	h.WallFaces[0] = -1
	h.StationRims[1][0] = -1
	if p.Stations().WallFaces[0] < 0 || p.Stations().StationRims[1][0] < 0 {
		t.Fatal("mutable station history")
	}
	h = p.Stations()
	bad := body
	bad.faces = body.Faces()
	bad.faces[h.WallFaces[1]].Surface = bad.faces[h.WallFaces[0]].Surface
	if denied, e := bad.PrepareWrappedStationDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("wrong original adjacent band accepted")
	}
	bad = body
	bad.faces = body.Faces()
	wall := h.WallFaces[1]
	bad.faces[wall].Outer[0].Edge = bad.faces[wall].Outer[1].Edge
	if denied, e := bad.PrepareWrappedStationDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("forged shared rim accepted")
	}
	opts.MaxStationSamples = 1
	if denied, e := body.PrepareWrappedStationDisplay(ctx, opts, val); e == nil || denied != nil {
		t.Fatal("station quota")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if denied, e := body.PrepareWrappedStationDisplay(cancelled, opts, val); e == nil || denied != nil {
		t.Fatal("cancel")
	}
	t.Log("triangles", len(local.Triangles), "volume", volume, "ideal", truth, "source checks", baseline.TriangleChecks)
}
