package poly

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"testing"
)

func TestEdgeHistoryIdentity(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	s, h, e := CombineWithTopologyHistory(context.Background(), a, brep.Solid{}, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if len(h.Edges) != len(s.Edges()) {
		t.Fatal("length")
	}
	original, generated := 0, 0
	seen := map[int]bool{}
	for _, v := range h.Edges {
		if len(v.AdjacentFaces) != 2 {
			t.Fatal("adjacency")
		}
		if v.HasGeneratedPortion {
			generated++
			if len(v.Overlaps) != 0 {
				t.Fatal("unexpected overlap")
			}
		} else {
			original++
			if len(v.Overlaps) != 1 {
				t.Fatal(v)
			}
			p := v.Overlaps[0]
			seen[p.Edge] = true
			if p.Operand != OperandA || p.OutputFrom.Sign() != 0 || p.OutputTo.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal(p)
			}
		}
	}
	if original != 12 || generated != 6 || len(seen) != 12 {
		t.Fatalf("original%d generated%d seen%d", original, generated, len(seen))
	}
}
func TestEdgeHistoryCoincidentAndEmpty(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	_, h, e := CombineWithTopologyHistory(context.Background(), a, a, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range h.Edges {
		if !v.HasGeneratedPortion {
			if len(v.Overlaps) != 2 || v.Overlaps[0].Operand != OperandA || v.Overlaps[1].Operand != OperandB {
				t.Fatal(v)
			}
		}
	}
	s, h, e := CombineWithTopologyHistory(context.Background(), a, a, Difference, Options{})
	if e != nil || len(s.Faces()) != 0 || len(h.Edges) != 0 {
		t.Fatal(e, h)
	}
}
func TestEdgeHistoryExactIntervals(t *testing.T) {
	a := box(t, 0, 0, 0, 3, 3, 3)
	b := box(t, 1, -1, -1, 4, 4, 4)
	s, h, e := CombineWithTopologyHistory(context.Background(), a, b, Intersection, Options{})
	if e != nil {
		t.Fatal(e)
	}
	vs := s.Vertices()
	foundThird := false
	for i, entry := range h.Edges {
		edge := s.Edges()[i]
		x, y := vec(vs[edge.Start]), vec(vs[edge.End])
		for _, ov := range entry.Overlaps {
			in := a
			if ov.Operand == OperandB {
				in = b
			}
			iv := in.Vertices()
			ie := in.Edges()[ov.Edge]
			p, q := vec(iv[ie.Start]), vec(iv[ie.End])
			for _, pair := range [][2]*big.Rat{{ov.OutputFrom, ov.InputFrom}, {ov.OutputTo, ov.InputTo}} {
				u := interpolateEdge(x, y, pair[0])
				v := interpolateEdge(p, q, pair[1])
				if !equal(u, v) {
					t.Fatal("parameter witness mismatch")
				}
				if pair[1].Cmp(big.NewRat(1, 3)) == 0 || pair[1].Cmp(big.NewRat(2, 3)) == 0 {
					foundThird = true
				}
			}
		}
	}
	if !foundThird {
		t.Fatal("expected nonbinary exact interval")
	}
}
func TestEdgeHistoryFailures(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	s, h, e := CombineWithTopologyHistory(context.Background(), a, a, Union, Options{MaxWork: 1})
	if !errors.Is(e, ErrLimit) || len(s.Faces()) != 0 || len(h.Edges) != 0 {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, e = CombineWithTopologyHistory(ctx, a, a, Union, Options{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func interpolateEdge(a, b point, t *big.Rat) point {
	return point{add(a[0], mul(sub(b[0], a[0]), t)), add(a[1], mul(sub(b[1], a[1]), t)), add(a[2], mul(sub(b[2], a[2]), t))}
}
