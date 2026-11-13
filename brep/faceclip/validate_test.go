package faceclip

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func boundaryFor(t *testing.T) Boundary {
	t.Helper()
	r, e := Clip(context.Background(), holed(t), 1, Halfspace{geom.Vec3{X: 1}, 4}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestValidateMergedRegion(t *testing.T) {
	b := boundaryFor(t)
	if e := ValidateBoundary(context.Background(), b, LoopOptions{}); e != nil {
		t.Fatal(e)
	}
	for _, o := range []LoopOptions{{MaxWork: 1}, {MaxVertices: 1}, {MaxBits: 1}} {
		if e := ValidateBoundary(context.Background(), b, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
}
func TestValidateRegionCorruption(t *testing.T) {
	for _, mode := range []string{"area", "plane", "parent", "winding"} {
		b := boundaryFor(t)
		switch mode {
		case "area":
			b.ProjectedArea = big.NewRat(13, 1)
		case "plane":
			b.Loops[0].Vertices[0][2] = big.NewRat(2, 1)
		case "parent":
			for i := range b.Loops {
				if b.Loops[i].Hole {
					b.Loops[i].Parent = -1
				}
			}
		case "winding":
			for i := range b.Loops {
				if b.Loops[i].Hole {
					v := b.Loops[i].Vertices
					for j, k := 0, len(v)-1; j < k; j, k = j+1, k-1 {
						v[j], v[k] = v[k], v[j]
					}
				}
			}
		}
		if e := ValidateBoundary(context.Background(), b, LoopOptions{}); e == nil {
			t.Fatal("accepted", mode)
		}
	}
}
func TestValidateCrossingAndBacktracking(t *testing.T) {
	for _, vertices := range [][]geom.Vec3{{{}, {X: 3, Y: 2}, {Y: 3}, {X: 2}}, {{}, {X: 2}, {X: 1}, {X: 2, Y: 2}, {Y: 2}}} {
		loop := Loop{Parent: -1}
		for _, v := range vertices {
			loop.Vertices = append(loop.Vertices, vec(v))
		}
		b := Boundary{Loops: []Loop{loop}, Axis: 2, ProjectedArea: big.NewRat(1, 1)}
		if e := ValidateBoundary(context.Background(), b, LoopOptions{}); !errors.Is(e, ErrNonmanifold) {
			t.Fatal(e)
		}
	}
}
