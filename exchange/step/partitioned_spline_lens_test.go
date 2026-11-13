package step

import (
	"bytes"
	"math/big"
	"os"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func partitionedLensFixture(t testing.TB) SplineLensBody {
	m := splineLensFixture(t)
	r := 15. / 64
	m.Radii = geom.Vec2{X: r, Y: r}
	m.Name = "Rational seam partitions"
	var supports [2]*geom.BSplineSurface
	for side, sign := range []float64{1, -1} {
		p := []geom.Vec3{}
		sq := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 2 + sign*(r*r-sq[i]-sq[j])/2})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		supports[side] = s
	}
	m.Upper, m.Lower = supports[0], supports[1]
	p, e := geom.NewSurfacePatch(m.Lower, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(41./64, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestPartitionedLensPlan(t *testing.T) {
	m := partitionedLensFixture(t)
	o, _ := (CurvedOvalOptions{}).defaults()
	pieces, e := lensPiecePlan(m, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(pieces) != 6 {
		t.Fatal(len(pieces))
	}
	third := false
	half := false
	for _, p := range pieces {
		third = third || p.End.Cmp(big.NewRat(1, 3)) == 0
		half = half || p.End.Cmp(big.NewRat(1, 2)) == 0
		if _, _, e = exactBezierFloats(p.UV, o.patch()); e != nil {
			t.Fatal(e)
		}
		if _, _, e = exactBezierFloats(p.Model, o.patch()); e != nil {
			t.Fatal(e)
		}
	}
	if !third || !half {
		t.Fatal("lost rational cuts")
	}
}
func TestPartitionedSplineLens(t *testing.T) {
	m := partitionedLensFixture(t)
	var b bytes.Buffer
	if e := WritePartitionedSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadPartitionedSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Pieces) != 6 || len(got.Body.Solid.Edges()) != 4 || got.Body.SourceEdges != ([4]int64{}) {
		t.Fatal("conflated source and native topology")
	}
	if _, e = ReadSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("four-edge reader broadened")
	}
	seen := map[int64]bool{}
	for _, p := range got.Pieces {
		if p.SourceEdgeID == 0 || seen[p.SourceEdgeID] {
			t.Fatal("source ancestry")
		}
		seen[p.SourceEdgeID] = true
		if p.Cells[0] == p.Cells[1] {
			t.Fatal("lost lower knot lineage")
		}
	}
}

func TestIndependentPartitionedLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedSplineLensBody(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Pieces) != 6 {
		t.Fatal(len(m.Pieces))
	}
	var b bytes.Buffer
	if e = WritePartitionedSplineLensBody(&b, m.Body, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	copy, e := ReadPartitionedSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(copy.Body.Lower.Data(), m.Body.Lower.Data()) {
		t.Fatal("lost original lower chart")
	}
}
