package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
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
		// All stored loops are complete rectangles, so integrate those exact
		// authored domains directly rather than triangulating their UV rectangles.
		var boundary []parametric.Face
		for _, f := range s.faces {
			a, b := f.Outer[0].UVStart, f.Outer[2].UVStart
			boundary = append(boundary, parametric.Face{Surface: f.Surface, U: parametric.Interval{Min: a.X, Max: b.X}, V: parametric.Interval{Min: a.Y, Max: b.Y}, Reversed: f.Reversed})
		}
		mass, e := parametric.IntegrateBoundary(ctx, boundary, geom.Vec3{}, parametric.IntegrationOptions{RelativeTolerance: 1e-5, MaxSamples: 200000})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(mass.Volume-volume) > 1e-4 || math.Abs(mass.Area-area) > 1e-4 {
			t.Fatal("independent OCCT", mass, volume, area)
		}
	}

}
