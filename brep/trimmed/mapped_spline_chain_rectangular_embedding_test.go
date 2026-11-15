package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestMappedSplineChainRectangularEmbedding(t *testing.T) {
	ctx := context.Background()
	boundary, closing, planar, seams := mappedChainClosedFixture(t)
	reversed := [2]bool{false, true}
	original, err := CertifyMappedSplineChainShell(ctx, boundary, closing, reversed, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var record bytes.Buffer
	if err := original.WriteJSON(ctx, &record, MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadMappedSplineChainShellJSON(ctx, &record, MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := loaded.CertifyRectangularEmbedding(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(3, 4)}}
	for i, interval := range r.Footprint {
		for j, q := range interval {
			if q.Cmp(want[i][j]) != 0 {
				t.Fatal("independent footprint", r.Footprint)
			}
		}
	}
	if r.FloorFace != 6 || r.WallFaces != [4]int{3, 2, 5, 4} || r.FloorHeight.Cmp(big.NewRat(-1, 1)) != 0 ||
		r.Clearance.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("independent source clearance or ownership", r)
	}

	// Reversing all faces preserves complete abstract source incidence.
	// It violates the graph material proof's outward orientation.
	for i := range reversed {
		reversed[i] = !reversed[i]
	}
	for i := range planar {
		planar[i].Reversed = !planar[i].Reversed
	}
	inward, err := CertifyMappedSplineChainShell(ctx, boundary, closing, reversed, planar, seams, MappedChainShellProofOptions{})
	if err != nil || inward.Topology() != original.Topology() {
		t.Fatal("inward control did not retain its closed source incidence", err)
	}
	assertNoEmbedding(t, ctx, inward, geom.SurfacePatchOptions{}, geom.ErrConvergence)

	// An exact global shear preserves every original seam and source region.
	// This placement takes the vertical walls out of the rectangular tier.
	for i := range reversed {
		reversed[i] = !reversed[i]
	}
	for i := range planar {
		planar[i].Reversed = !planar[i].Reversed
	}
	shear := func(surface geom.Surface) geom.Surface {
		t.Helper()
		d := surface.(*geom.BSplineSurface).Data()
		for i := range d.Poles {
			d.Poles[i].X += d.Poles[i].Z / 4
		}
		result, err := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	shiftedChain, err := CertifyMappedSplineChainAgreement(ctx, shear(boundary.supports[0]), boundary.sources[0], shear(boundary.supports[1]), boundary.sources[1], edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range planar {
		planar[i].Surface = shear(planar[i].Surface)
	}
	shifted, err := CertifyMappedSplineChainShell(ctx, shiftedChain, closing, reversed, planar, seams, MappedChainShellProofOptions{})
	if err != nil || shifted.Topology() != original.Topology() {
		t.Fatal("shear changed original incidence", err)
	}
	assertNoEmbedding(t, ctx, shifted, geom.SurfacePatchOptions{}, geom.ErrConvergence)
	t.Run("source roof crosses the floor", func(t *testing.T) {
		_, closing, planar, seams := mappedChainClosedFixture(t)
		// The same world height is represented on both original XY charts.
		// h=x*y-64*x*(1-x)*y*(3/4-y) preserves every outer roof edge
		// and all wall/floor geometry while dipping through the floor.
		dipped := func(swapped bool) geom.Surface {
			xFactor := [3]float64{0, .5, 0}
			yFactor := [3]float64{0, .375, -.25}
			poles := make([]geom.Vec3, 0, 9)
			for u := range 3 {
				for v := range 3 {
					i, j := u, v
					if swapped {
						i, j = j, i
					}
					x, y := float64(i)/2, float64(j)/2
					poles = append(poles, geom.Vec3{X: x, Y: y, Z: x*y - 64*xFactor[i]*yFactor[j]})
				}
			}
			k := []float64{0, 0, 0, 1, 1, 1}
			p, err := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
		a, b := dipped(false), dipped(true)
		chain, err := CertifyMappedSplineChainAgreement(ctx, a, boundary.sources[0], b, boundary.sources[1], edgecheck.Options{})
		if err != nil {
			t.Fatal(err)
		}
		colliding, err := CertifyMappedSplineChainShell(ctx, chain, closing, reversed, planar, seams, MappedChainShellProofOptions{})
		if err != nil || colliding.Topology() != original.Topology() {
			t.Fatal("colliding source must retain valid analytic incidence", err)
		}
		// At x=1/2 the original roof seam has y=1/4. The north chart
		// contains the entire interval from y=3/8 to its y=3/4 edge.
		for _, sample := range []struct {
			y    float64
			want float64
		}{{.375, -2.0625}, {.75, .375}} {
			p, err := a.Eval(.5, sample.y)
			if err != nil || p.Z != sample.want {
				t.Fatal("independent floor-crossing witness", p, err)
			}
		}
		bad, err := colliding.CertifyRectangularEmbedding(ctx, geom.SurfacePatchOptions{})
		if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "clearance") || bad.Clearance != nil {
			t.Fatal("crossing must reject the complete continuous source embedding", err)
		}
	})

	assertNoEmbedding(t, ctx, loaded, geom.SurfacePatchOptions{MaxOperations: 1}, geom.ErrConvergence)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	assertNoEmbedding(t, canceled, loaded, geom.SurfacePatchOptions{}, context.Canceled)
}

func assertNoEmbedding(t *testing.T, ctx context.Context, r MappedSplineChainShell, o geom.SurfacePatchOptions, want error) {
	t.Helper()
	got, err := r.CertifyRectangularEmbedding(ctx, o)
	if !errors.Is(err, want) || got.Clearance != nil || got.Footprint[0][0] != nil {
		t.Fatal("embedding should reject atomically", err)
	}
}
