package fillet

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedSelectedRectangleFillets(t *testing.T) {
	selection := []CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}
	r, e := PrismRectangleCornersCertified(context.Background(), geom.Vec2{}, geom.Vec2{X: 4, Y: 3}, -1, 1, selection, RectangleMeshOptions{Deflection: .05})
	if e != nil {
		t.Fatal(e)
	}
	if e := r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !r.Collision.Complete || len(r.Collision.Contacts) != 0 {
		t.Fatal("display embedding")
	}
	if r.IdealBoundarySquaredDistance.Cmp(num(.0025)) > 0 || len(r.OriginalFace) != len(r.Mesh.Triangles) {
		t.Fatal("certificate")
	}
	owners := map[[2]int]map[int]bool{}
	counts := map[int]int{}
	for i, tri := range r.Mesh.Triangles {
		counts[r.OriginalFace[i]]++
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if owners[key] == nil {
				owners[key] = map[int]bool{}
			}
			owners[key][r.OriginalFace[i]] = true
		}
	}
	if len(r.Mesh.Vertices)-len(owners)+len(r.Mesh.Triangles) != 2 {
		t.Fatal("Euler")
	}
	for _, id := range r.Model.StraightFaces {
		if counts[id] == 0 {
			t.Fatal("missing straight ancestry")
		}
	}
	for _, c := range r.Model.Corners {
		if c != nil && counts[c.CylinderFace] == 0 {
			t.Fatal("missing fillet ancestry")
		}
	}
	for _, id := range r.Model.Caps {
		if counts[id] == 0 {
			t.Fatal("missing cap")
		}
	}
	if r.Model.Corners[1] != nil || r.Model.Corners[3] != nil {
		t.Fatal("unselected corners changed")
	}
	ring := r.CapProof.BoundaryLoops[0]
	count := len(r.CapProof.Mesh.Vertices)
	for cap := 0; cap < 2; cap++ {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			a += cap * count
			b += cap * count
			if a > b {
				a, b = b, a
			}
			pair := owners[[2]int{a, b}]
			if len(pair) != 2 || !pair[r.Model.Caps[cap]] {
				t.Fatal("cap seam ownership")
			}
		}
	}
	idealArea := 12 - (1-math.Pi/4)*(.25*.25+.5*.5)
	lower, _ := r.CapProof.Region.SourceUVArea.Lower.Float64()
	upper, _ := r.CapProof.Region.SourceUVArea.Upper.Float64()
	if idealArea < lower || idealArea > upper {
		t.Fatal("independent circular area", lower, idealArea, upper)
	}
	want := 2 * (12 - (1-math.Pi/4)*(.25*.25+.5*.5))
	got, _ := r.SignedDisplayVolume.Float64()
	if math.Abs(got-want) > .002*want {
		t.Fatal(got, want)
	}
	t.Log(len(r.Mesh.Vertices), len(r.Mesh.Triangles), got, want, r.IdealBoundarySquaredDistance.FloatString(12))
}
func TestCertifiedRectangleFilletLimits(t *testing.T) {
	ctx := context.Background()
	min, max := geom.Vec2{}, geom.Vec2{X: 4, Y: 3}
	selection := []CornerRadius{{Corner: 0, Radius: .25}}
	for _, o := range []RectangleMeshOptions{{Deflection: .05, MaxTriangles: 1}, {Deflection: .05, Collision: collision.Options{MaxPairs: 1}}} {
		r, e := PrismRectangleCornersCertified(ctx, min, max, -1, 1, selection, o)
		if e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("partial budget", e)
		}
	}
	for _, bad := range [][]CornerRadius{{{Corner: 0, Radius: 3}}, {{Corner: 0, Radius: .25}, {Corner: 0, Radius: .5}}} {
		r, e := PrismRectangleCornersCertified(ctx, min, max, -1, 1, bad, RectangleMeshOptions{Deflection: .05})
		if e == nil || len(r.Mesh.Vertices) != 0 {
			t.Fatal("invalid radius")
		}
	}
}

func TestCertifiedRectangleFilletExactDimensions(t *testing.T) {
	r, e := PrismRectangleCornersCertified(context.Background(), geom.Vec2{X: .1}, geom.Vec2{X: 4.1, Y: 3}, 0, 2, []CornerRadius{{Corner: 0, Radius: .2}}, RectangleMeshOptions{Deflection: .05})
	if e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("rounded exact tangent dimension accepted", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = PrismRectangleCornersCertified(ctx, geom.Vec2{}, geom.Vec2{X: 4, Y: 3}, 0, 2, []CornerRadius{{Corner: 0, Radius: .25}}, RectangleMeshOptions{Deflection: .05})
	if e != context.Canceled || len(r.Mesh.Vertices) != 0 {
		t.Fatal(e)
	}
}
