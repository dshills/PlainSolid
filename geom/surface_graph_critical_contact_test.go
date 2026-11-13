package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestGraphCriticalMorseClassification(t *testing.T) {
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	for _, saddle := range []bool{false, true} {
		a, b := graphContactFixture(t, false, saddle)
		for swap, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := ClassifySplineGraphCriticalContact(context.Background(), pair[0], pair[1], uv, GraphPointContactOptions{})
			expected := GraphCriticalMinimum
			if saddle {
				expected = GraphCriticalSaddle
			} else if swap == 1 {
				expected = GraphCriticalMaximum
			}
			if e != nil || !r.Proven || r.Kind != expected || r.ProjectionDeterminant.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal(r, e)
			}
			det := big.NewRat(4, 1)
			if saddle {
				det.Neg(det)
			}
			if r.Determinant.Cmp(det) != 0 {
				t.Fatal(r.Determinant)
			}
		}
	}
	a, b := graphContactFixture(t, false, true)
	// Exact very small saddle curvature is not mistaken for degeneracy.
	scale := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		delta := new(big.Rat).Sub(a.net[i][2], b.net[i][2])
		a.net[i][2] = new(big.Rat).Add(b.net[i][2], new(big.Rat).Mul(scale, delta))
	}
	r, e := ClassifySplineGraphCriticalContact(context.Background(), a, b, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Kind != GraphCriticalSaddle {
		t.Fatal(r, e)
	}
}
func TestGraphCriticalRejectsNonstationaryAndKnotWitness(t *testing.T) {
	a, b := graphContactFixture(t, false, false)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	refined, e := a.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := ClassifySplineGraphCriticalContact(context.Background(), refined, b, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven {
		t.Fatal(r, e)
	}
	refined, e = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = ClassifySplineGraphCriticalContact(context.Background(), refined, b, uv, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("knot differentiability not proved", r, e)
	}
	r, e = ClassifySplineGraphCriticalContact(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	r, e = ClassifySplineGraphCriticalContact(context.Background(), b, b, uv, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	for _, p := range []*SurfacePatch{a, b} {
		for i := range p.net {
			p.net[i][0] = new(big.Rat)
		}
	}
	r, e = ClassifySplineGraphCriticalContact(context.Background(), a, b, uv, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal("singular XY chart", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ClassifySplineGraphCriticalContact(ctx, a, b, uv, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = ClassifySplineGraphCriticalContact(context.Background(), a, b, uv, GraphPointContactOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
