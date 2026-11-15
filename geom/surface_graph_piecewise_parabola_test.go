package geom

import (
	"context"
	"math/big"
	"testing"
)

func changedParabolaFixture(t *testing.T, axis int) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var out [2]*SurfacePatch
	for owner := 0; owner < 2; owner++ {
		xs := []float64{0, .25, .5, .75, 1}
		knots := []float64{0, 0, 0, .5, .5, 1, 1, 1}
		fs := []float64{.25, .5, .375, .375, .25}
		if owner == 1 {
			xs = []float64{0, .5, 1}
			knots = []float64{0, 0, 0, 1, 1, 1}
		}
		var poles []Vec3
		for i, u := range xs {
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v - fs[i]
				}
				poles = append(poles, Vec3{u, v, z})
			}
		}
		s, e := NewBSplineSurface(2, 2, len(xs), 3, poles, knots, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if axis == 1 {
			d := s.Data()
			poles = nil
			for j := 0; j < d.CountV; j++ {
				for i := 0; i < d.CountU; i++ {
					p := d.Poles[i*d.CountV+j]
					p.X, p.Y = p.Y, p.X
					poles = append(poles, p)
				}
			}
			s, e = NewBSplineSurface(2, 2, d.CountV, d.CountU, poles, d.KnotsV, d.KnotsU, nil)
			if e != nil {
				t.Fatal(e)
			}
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if owner == 1 {
			if axis == 0 {
				p, e = p.InsertUKnot(.75, 1, SurfacePatchOptions{})
			} else {
				p, e = p.InsertVKnot(.75, 1, SurfacePatchOptions{})
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		out[owner] = p
	}
	return out[0], out[1]
}
func TestPiecewiseParabolaOriginalKnotAdjacency(t *testing.T) {
	for _, axis := range []int{0, 1} {
		a, b := changedParabolaFixture(t, axis)
		for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverPiecewiseSplineGraphParabola(context.Background(), pair[0], pair[1], PiecewiseParabolaOptions{FreeAxis: axis})
			if e != nil || !r.Proven || len(r.Pieces) != 3 || len(r.Joins) != 2 {
				t.Fatal(axis, order, r, e)
			}
			want := 1
			if order == 1 {
				want = -1
			}
			if r.PositiveSide != want {
				t.Fatal("occupancy", r.PositiveSide)
			}
			j := r.Joins[0]
			if j.UV[axis].Cmp(big.NewRat(1, 2)) != 0 || j.UV[1-axis].Cmp(big.NewRat(3, 8)) != 0 {
				t.Fatal(j)
			}
			if j.Point[2].Cmp(big.NewRat(3075, 1024)) != 0 || j.Tangents[0][2].Cmp(big.NewRat(1, 512)) != 0 || j.Tangents[1][2].Cmp(big.NewRat(3, 512)) != 0 {
				t.Fatal("original model witnesses", j)
			}
			if j.Tangents[0][1-axis].Cmp(big.NewRat(-1, 2)) != 0 || j.Tangents[1][1-axis].Sign() != 0 {
				t.Fatal("lost C0 kink", j.Tangents)
			}
			// Independently evaluate the authored two formulas at one point per slab.
			for k, piece := range r.Pieces {
				u := []*big.Rat{big.NewRat(1, 4), big.NewRat(5, 8), big.NewRat(7, 8)}[k]
				f := new(big.Rat).Add(piece.Power[0], new(big.Rat).Mul(u, new(big.Rat).Add(piece.Power[1], new(big.Rat).Mul(u, piece.Power[2]))))
				want := big.NewRat(13, 32)
				if k == 1 {
					want = big.NewRat(47, 128)
				}
				if k == 2 {
					want = big.NewRat(39, 128)
				}
				if f.Cmp(want) != 0 {
					t.Fatal("independent polynomial", k, f, want)
				}
			}
			for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
				q, e := DiscoverPiecewiseSplineGraphParabola(context.Background(), pair[0], pair[1], PiecewiseParabolaOptions{FreeAxis: axis, Proof: GraphPointContactOptions{MaxOperations: limit}})
				if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Pieces) > 0 || len(q.Joins) > 0)) {
					t.Fatal("atomic quota", limit, e)
				}
			}
		}
	}
}
func TestPiecewiseParabolaRejectsIncompleteScope(t *testing.T) {
	a, b := changedParabolaFixture(t, 0)
	for _, o := range []PiecewiseParabolaOptions{{MaxPieces: 2}, {FreeAxis: 1}, {Proof: GraphPointContactOptions{MaxCells: 2}}, {Proof: GraphPointContactOptions{MaxOperations: 1}}} {
		r, e := DiscoverPiecewiseSplineGraphParabola(context.Background(), a, b, o)
		if e == nil || r.Proven || len(r.Pieces) > 0 {
			t.Fatal("partial publication", o, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := DiscoverPiecewiseSplineGraphParabola(ctx, a, b, PiecewiseParabolaOptions{}); e == nil || r.Proven {
		t.Fatal(e)
	}
	// The old one-polynomial proof must still reject this genuinely changed graph.
	if r, e := DiscoverSplineGraphParabola(context.Background(), a, b, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("legacy scope widened")
	}
}

func TestPiecewiseParabolaLaterSourceMutation(t *testing.T) {
	a, b := changedParabolaFixture(t, 0)
	source, e := a.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	d := source.Data()
	d.Poles[len(d.Poles)-1].Z += 1.0 / 64
	changed, e := NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(changed, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverPiecewiseSplineGraphParabola(context.Background(), p, b, PiecewiseParabolaOptions{})
	if e == nil || r.Proven || len(r.Pieces) != 0 || len(r.Joins) != 0 {
		t.Fatal("later source branch was omitted", e)
	}
}
