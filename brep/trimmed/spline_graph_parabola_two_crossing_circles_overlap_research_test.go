//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaTwoCrossingCirclesOverlappingProjections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	f := parabolaTwoCrossingCirclesOverlappingProjections()
	// Independent Green integrals of the original polynomial over the
	// stock, the parabolic strip, and the true circular segments.
	envelopeVolume := [2]float64{
		3077966251710468651171107674069.0/1544949169028154583074107031552 - 24346835150740044425.0*math.Pi/75557863725914323419136,
		3107120598742298359868425660459.0/1544949169028154583074107031552 + 24346835150740044425.0*math.Pi/75557863725914323419136,
	}
	cutVolume := [2][3]float64{
		{
			22285640997159849810594261035.0/3089898338056309166148214063104 + 73040505452220133275.0*math.Pi/151115727451828646838272,
			3157046895013458249341237195.0/6179796676112618332296428126208 - 48693421770505635725.0*math.Pi/302231454903657293676544,
			16494028902037895796875.0/6179796676112618332296428126208 - 248530974453125.0*math.Pi/302231454903657293676544,
		},
		{
			10955329892485735765601559225.0/1029966112685436388716071354368 - 24346835150740044425.0*math.Pi/151115727451828646838272,
			3157046895013458249341237195.0/6179796676112618332296428126208 + 146080265311516907175.0*math.Pi/302231454903657293676544,
			16494028902037895796875.0/6179796676112618332296428126208 + 745592923359375.0*math.Pi/302231454903657293676544,
		},
	}
	for axis := range 2 {
		stocks := f.stocks(t, axis, false)
		for order := range 2 {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				roofs := [2]*geom.BSplineSurface{stocks[1+order], stocks[2-order]}
				for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, err := CombineParabolaTwoCrossingCirclesSplineGraphs(ctx, stocks[0], roofs[0], roofs[1], axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					body := r.Boundary()
					topo := body.Topology()
					if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.InnerWires != 0 ||
						([3]int{topo.Faces, topo.Edges, topo.Vertices}) != ([3]int{13, 36, 25}) {
						t.Fatal("projected overlap changed exact native envelope topology", topo)
					}
					h := r.History()
					for circle, ring := range h.CircleEdges {
						d := f.discs[circle]
						if len(ring) != [2]int{6, 5}[circle] {
							t.Fatal("changed exact circle subdivision count", len(ring))
						}
						added := make(map[int]bool, 1)
						for _, id := range ring {
							e := body.edges[id]
							for _, v := range [2]int{e.Start, e.End} {
								point := body.vertices[v]
								free, dependent := point.X, point.Y
								if axis == 1 {
									free, dependent = dependent, free
								}
								dx, dy := new(big.Rat).Sub(ratInset(free), d.x), new(big.Rat).Sub(ratInset(dependent), d.y)
								if dx.Sign() != 0 && dy.Sign() != 0 {
									added[v] = true
								}
								radiusSquared := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
								if radiusSquared.Cmp(d.radiusSquared) != 0 || point.Z != 2 {
									t.Fatal("subdivision left the original shared circle", circle, point)
								}
							}
						}
						if len(added) != [2]int{2, 1}[circle] {
							t.Fatal("wrong number of exact intermediate circle vertices", circle, added)
						}
					}
					checkOverlappingTwoCrossingCircleVolume(t, body, envelopeVolume[i])
					if axis == 1 && order == 1 {
						checkTwoCrossingCirclesEnvelopeReplay(t, r, false)
					}
				}
				cut, err := CutParabolaTwoCrossingCirclesSplineGraphs(ctx, stocks[0], roofs[0], roofs[1], axis, 1e-9, ParabolaCircleGraphOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for i, part := range cut.Components() {
					checkOverlappingTwoCrossingCircleVolume(t, part, cutVolume[order][i])
				}
				for circle, contacts := range cut.Contacts() {
					for j, contact := range contacts {
						x := f.roots[circle][j]
						if contact.Parameters[axis].Cmp(x) != 0 || contact.Parameters[1-axis].Cmp(f.ordinate(x)) != 0 ||
							contact.Components != [2]int{0, circle + 1} {
							t.Fatal("projected overlap confused native contact owners", contact)
						}
					}
				}
				if axis == 1 && order == 1 {
					checkTwoCrossingCirclesCutReplay(t, cut, false)
				}
			})
		}
	}
}

func checkOverlappingTwoCrossingCircleVolume(t *testing.T, body Solid, want float64) {
	t.Helper()
	mass, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7, MaxSubdivisions: 64})
	if err != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want) > min(1e-9, 2e-5*want) {
		t.Fatal("native material differs from independent signed integral", mass, want, err)
	}
	t.Logf("signed volume %.17g; exact integral %.17g", mass.Volume, want)
}
