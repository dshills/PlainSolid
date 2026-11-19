//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestAlgebraicGraphOccurrenceBoundsAndMass(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := graphMaterialBooleanFixture(t, false, false)
	var pose [3][4]*big.Rat
	for i := range pose {
		for j := range pose[i] {
			pose[i][j] = new(big.Rat)
		}
	}
	pose[0][0] = big.NewRat(-2, 1)
	pose[0][1] = big.NewRat(1, 3)
	pose[0][3] = big.NewRat(1, 7)
	pose[1][0] = big.NewRat(1, 5)
	pose[1][2] = big.NewRat(-3, 1)
	pose[1][3] = big.NewRat(2, 7)
	pose[2][1] = big.NewRat(1, 1)
	pose[2][3] = big.NewRat(-1, 11)
	occurrence, e := NewAlgebraicGraphOccurrence(ctx, source, pose, [2]int{0, 2}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bounds, e := occurrence.BoundsRational(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(bounds.Faces) != 4 {
		t.Fatal("whole/retained face bounds", bounds, e)
	}
	image := func(p [3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat).Set(pose[i][3])
			for j := 0; j < 3; j++ {
				r[i].Add(r[i], new(big.Rat).Mul(pose[i][j], p[j]))
			}
		}
		return r
	}
	for _, x := range []*big.Rat{big.NewRat(5, 16), big.NewRat(11, 16)} {
		for _, z := range []*big.Rat{new(big.Rat), big.NewRat(25, 4096)} {
			p := image(queryPoint(x, big.NewRat(1, 2), z))
			location, e := occurrence.LocateRational(ctx, p, geom.ExactCurvePlaneEventOptions{})
			if e != nil || len(location.BoundaryFaces) != 1 {
				t.Fatal("analytic boundary point", e)
			}
			found := false
			for _, face := range bounds.Faces {
				if face.Face == location.BoundaryFaces[0] {
					found = true
					for axis, v := range p {
						if v.Cmp(face.Bounds[axis].Lower) < 0 || v.Cmp(face.Bounds[axis].Upper) > 0 || v.Cmp(bounds.Whole[axis].Lower) < 0 || v.Cmp(bounds.Whole[axis].Upper) > 0 {
							t.Fatal("world hull misses original face witness", face.Face, axis)
						}
					}
				}
			}
			if !found {
				t.Fatal("native face not bounded", location.BoundaryFaces)
			}
		}
	}
	options := AlgebraicGraphOccurrenceMassOptions{Original: AlgebraicGraphWholeMassOptions{MomentOrder: 2, Boundary: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}}
	mass, e := occurrence.BoundWholeMassProperties(ctx, options)
	if e != nil || !mass.HasCentroidal || mass.AbsoluteDeterminant.Cmp(big.NewRat(6, 1)) != 0 || mass.Pose[0][1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("exact rational reflected mass", e)
	}
	rr := 25.0 / 4096
	v := 6 * math.Pi * rr * rr
	sourceMean := [3]float64{.5, .5, rr / 3}
	variance := [3]float64{9.0/256 + rr/6, rr / 6, rr * rr / 18}
	var mean [3]float64
	var covariance [3][3]float64
	var a [3][4]float64
	for i := range a {
		for j := range a[i] {
			a[i][j], _ = pose[i][j].Float64()
		}
	}
	for i := 0; i < 3; i++ {
		mean[i] = a[i][3]
		for k := 0; k < 3; k++ {
			mean[i] += a[i][k] * sourceMean[k]
		}
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				covariance[i][j] += a[i][k] * a[j][k] * variance[k]
			}
		}
	}
	contains := func(label string, b [2]*big.Rat, truth float64) {
		t.Helper()
		lo, _ := b[0].Float64()
		hi, _ := b[1].Float64()
		if truth < lo || truth > hi {
			t.Fatal(label, truth, lo, hi)
		}
	}
	contains("volume", mass.Volume, v)
	for i := 0; i < 3; i++ {
		contains("centroid", mass.Centroid[i], mean[i])
		contains("first", mass.FirstMoments[i], v*mean[i])
	}
	trace := covariance[0][0] + covariance[1][1] + covariance[2][2]
	for i, p := range [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}} {
		raw := v * (covariance[p[0]][p[1]] + mean[p[0]]*mean[p[1]])
		central := -covariance[p[0]][p[1]]
		if p[0] == p[1] {
			central += trace
		}
		contains("rawsecond", mass.RawSecondMoments[i], raw)
		contains("central inertia", mass.CentroidalInertia[i], v*central)
	}
	for _, face := range bounds.Faces {
		found := false
		for _, id := range mass.Original.Whole.Boundary.Owners[face.Operand].NativeFaces {
			found = found || id == face.Face
		}
		if !found {
			t.Fatal("mass/bounds original ownership mismatch")
		}
	}
	var native bytes.Buffer
	if e = WriteAlgebraicGraphOccurrenceJSON(&native, occurrence); e != nil {
		t.Fatal(e)
	}
	section := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	boolean := AlgebraicShellBooleanOptions{}
	boolean.Imprint.Exact.MaxOperations = 512000000
	back, e := ReadAlgebraicGraphOccurrenceJSON(ctx, &native, JSONLimits{}, section, boolean, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	backBounds, e := back.BoundsRational(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !reflect.DeepEqual(backBounds, bounds) {
		t.Fatal("reloaded hull/IDs", e)
	}
	backMass, e := back.BoundWholeMassProperties(ctx, options)
	if e != nil || !reflect.DeepEqual(backMass.Volume, mass.Volume) || !reflect.DeepEqual(backMass.FirstMoments, mass.FirstMoments) || !reflect.DeepEqual(backMass.RawSecondMoments, mass.RawSecondMoments) || !reflect.DeepEqual(backMass.Centroid, mass.Centroid) {
		t.Fatal("reloaded exact image integrals", e)
	}
	bounds.Faces[0].Bounds[0].Lower.SetInt64(999)
	mass.Pose[0][0].SetInt64(999)
	fresh, e := occurrence.BoundsRational(ctx, geom.ExactCurvePlaneEventOptions{})
	if e != nil || fresh.Faces[0].Bounds[0].Lower.Cmp(big.NewRat(999, 1)) == 0 || occurrence.Pose()[0][0].Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("returned rational alias", e)
	}
	fail := options
	fail.Transform.MaxOperations = 1
	if bad, e := occurrence.BoundWholeMassProperties(ctx, fail); !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil || bad.HasFirst {
		t.Fatal("atomic transform budget", e)
	}
	if bad, e := occurrence.BoundsRational(ctx, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, geom.ErrConvergence) || len(bad.Faces) > 0 || bad.Whole[0].Lower != nil {
		t.Fatal("atomic bounds budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := occurrence.BoundWholeMassProperties(cancelled, options); !errors.Is(e, context.Canceled) || bad.Volume[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("reflection/nonbinary pose determinant6; %d hull ops, %d image mass ops; %d original partition attempts", backBounds.ExactOperations, backMass.TransformOperations, backMass.Original.Whole.Boundary.Partition.CellAttempts)
}
