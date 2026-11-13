package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestExactLineCandidateSharedSplineEdge(t *testing.T) {
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	uv := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	q := []ExactSplineLineCandidate{
		{Face: 1, UV: uv(1, 2)}, // x=1 lateral face, (y,z)=(1/2,1)
		{Face: 5, UV: uv(2, 1)}, // roof, (x,y)=(1,1/2)
		{Face: 5, UV: uv(1, 1)}, // off the selected vertical line
		{Face: 5, UV: uv(2, 1)}, // duplicate
	}
	h, e := s.VerifySplineLineCandidates(context.Background(), geom.Vec3{X: 1, Y: .5}, geom.Vec3{Z: 3}, 0, 1, q, SplineFacePointOptions{})
	if e != nil || len(h.Points) != 1 || len(h.Rejected) != 1 || h.Rejected[0] != 2 || len(h.Unresolved) != 0 {
		t.Fatal(h, e)
	}
	p := h.Points[0]
	if p.Parameter.Cmp(big.NewRat(17, 48)) != 0 || len(p.Faces) != 2 || !p.OnTrim[0] || !p.OnTrim[1] {
		t.Fatal(p)
	}
}
