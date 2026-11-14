package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestNativeObliqueTorusCavity(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, q, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, pose, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.faces) != 19 {
		t.Fatal(len(s.faces))
	}
	for i := 3; i < 19; i++ {
		if !s.faces[i].Reversed {
			t.Fatal("cavity orientation")
		}
	}
	v, e := s.RationalVertices(ctx)
	if e != nil || len(v) != 16 {
		t.Fatal(len(v), e)
	}
	for _, p := range v {
		loc, mask, e := q.ClassifyRational(ctx, p.Point, geom.SurfacePatchOptions{})
		if e != nil || loc != geom.BoreBoundary || mask != geom.TorusCylinderTorus {
			t.Fatal(loc, mask, e)
		}
	}
	// Each torus edge has exactly two opposite oriented face incidences.
	type incidence struct{ count, direction int }
	uses := map[int]incidence{}
	for _, f := range s.faces[3:] {
		for _, u := range f.Outer {
			x := uses[u.Edge]
			x.count++
			sign := 1
			if u.Reversed {
				sign = -sign
			}
			if f.Reversed {
				sign = -sign
			}
			x.direction += sign
			uses[u.Edge] = x
		}
	}
	if len(uses) != 32 {
		t.Fatal("torus edges", len(uses))
	}
	for _, x := range uses {
		if x.count != 2 || x.direction != 0 {
			t.Fatal("open or unoriented seam", x)
		}
	}
	if len(v)-len(uses)+16 != 0 {
		t.Fatal("torus Euler")
	}
	if _, _, err := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, pose, 1e-9, geom.SurfacePatchOptions{MaxOperations: 1}); err == nil {
		t.Fatal("budget")
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(bytes.NewReader(b.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	restored, e := reloaded.RationalVertices(ctx)
	if e != nil || len(restored) != len(v) {
		t.Fatal(e)
	}
	for i := range restored {
		if restored[i].Vertex != v[i].Vertex || !sameRationalVertex(restored[i].Point, v[i].Point) {
			t.Fatal("reload source binding")
		}
	}
	var c bytes.Buffer
	if e = WriteJSON(&c, reloaded); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(b.Bytes(), c.Bytes()) {
		t.Fatal("unstable reload")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY"); oracle != "" {
		data, e := exec.Command(oracle).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		var volume, area float64
		if _, e = fmt.Fscan(strings.NewReader(string(data)), &volume, &area); e != nil {
			t.Fatal(e, string(data))
		}
		measured, e := BoundObliqueTorusCavityArea(ctx, s, 3, 1, 8, -6, 6, pose, geom.TorusCylinderAreaOptions{AreaTolerance: r(1, 1000000000)})
		if e != nil {
			t.Fatal(e)
		}
		lo, _ := measured.Lower.Float64()
		hi, _ := measured.Upper.Float64()
		if area < lo-1e-6 || area > hi+1e-6 || math.Abs(volume-(768*math.Pi-6*math.Pi*math.Pi)) > 1e-6 {
			t.Fatal("independent OCCT", volume, area, lo, hi)
		}

	}

}
