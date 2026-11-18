package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func anisotropicOccurrenceMatrices() [][3][4]*big.Rat {
	r := big.NewRat
	return [][3][4]*big.Rat{
		{{r(0, 1), r(0, 1), r(1, 1), r(1, 3)}, {r(-1, 1), r(0, 1), r(0, 1), r(-2, 7)}, {r(0, 1), r(-1, 1), r(0, 1), r(5, 11)}},
		{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}},
	}
}

// Independent rational matrix multiplication; no placement helper is used.
func independentAnisotropicWorld(m [3][4]*big.Rat, p [3]*big.Rat, vector bool) (q [3]*big.Rat) {
	for i := range q {
		q[i] = new(big.Rat)
		if !vector {
			q[i].Set(m[i][3])
		}
		for j := range p {
			q[i].Add(q[i], new(big.Rat).Mul(m[i][j], p[j]))
		}
	}
	return
}

func TestAnisotropicOccurrenceQueries(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	bore := anisotropicMeasuredBore(t)
	filled := anisotropicMassFixture(t, true, geom.Vec3{Z: -4})
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-10}}
	cases := []struct {
		p, d   [3]*big.Rat
		lo, hi *big.Rat
	}{
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(-4, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-4, 1), r(4, 1)},
		{[3]*big.Rat{r(-7, 4), r(-3, 8), r(-4, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 1)}, r(0, 1), r(3, 1)},
		{[3]*big.Rat{r(3, 4), r(-1, 2), r(-4, 1)}, [3]*big.Rat{r(-1, 8), r(0, 1), r(1, 1)}, r(0, 1), r(2, 1)},
	}
	originals := make([]WrappedAnisotropicBoreLine, len(cases))
	for i, c := range cases {
		q, e := bore.ClipWrappedAnisotropicStationBoreLineRational(ctx, c.p, c.d, c.lo, c.hi, o)
		if e != nil {
			t.Fatal(e)
		}
		originals[i] = q
	}
	point := [3]*big.Rat{r(5, 4), r(-1, 2), r(-3, 1)}
	localPoint, e := filled.LocateWrappedAnisotropicStationRational(ctx, point, WrappedRationalStationPointOptions{})
	if e != nil {
		t.Fatal(e)
	}
	capOriginal, e := filled.ClipWrappedAnisotropicStationCapLineRational(ctx, cases[0].p, cases[0].d, cases[0].lo, cases[0].hi, o)
	if e != nil {
		t.Fatal(e)
	}
	filledLine, e := filled.ClipWrappedAnisotropicStationLineRational(ctx, cases[1].p, cases[1].d, cases[1].lo, cases[1].hi, o)
	if e != nil {
		t.Fatal(e)
	}
	horizontalPoint := [3]*big.Rat{r(1, 4), r(-1, 2), r(-7, 2)}
	horizontalOptions := WrappedRationalStationLineOptions{Geometry: o.Geometry}
	horizontal, e := filled.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, horizontalPoint, cases[0].d, r(-4, 1), r(4, 1), horizontalOptions)
	if e != nil {
		t.Fatal(e)
	}
	sourceProof, e := filled.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	generatorPoint := [3]*big.Rat{r(13, 4), r(-1, 2), r(-4, 1)}
	generatorDirection := [3]*big.Rat{r(-2, 1), r(0, 1), r(1, 1)}
	generator, e := filled.ClipWrappedAnisotropicStationGeneratorRational(ctx, sourceProof.History.WallFaces[0], generatorPoint, generatorDirection, r(0, 1), r(1, 1), WrappedAnisotropicGeneratorOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range anisotropicOccurrenceMatrices() {
		pose, e := placement.NewRationalRigid(m, 16384)
		if e != nil {
			t.Fatal(e)
		}
		q, e := bore.PlaceWrappedAnisotropicStationBore(ctx, pose, WrappedRationalStationOccurrenceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		f, e := filled.PlaceWrappedAnisotropicStation(ctx, pose, WrappedRationalStationOccurrenceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i, c := range cases {
			p, d := independentAnisotropicWorld(m, c.p, false), independentAnisotropicWorld(m, c.d, true)
			got, e := q.ClipLineRational(ctx, p, d, c.lo, c.hi, o)
			if e != nil || !reflect.DeepEqual(got, originals[i]) {
				t.Fatal("world line/root/owner invariance", i, e)
			}
			location, e := q.LocateRational(ctx, p, WrappedRationalStationPointOptions{})
			local, e2 := bore.LocateWrappedAnisotropicStationBoreRational(ctx, c.p, WrappedRationalStationPointOptions{})
			if e != nil || e2 != nil || location != local {
				t.Fatal("material invariance", e, e2)
			}
		}
		world := independentAnisotropicWorld(m, point, false)
		got, e := f.LocateRational(ctx, world, WrappedRationalStationPointOptions{})
		if e != nil || !reflect.DeepEqual(got, localPoint) || len(got.Faces) != 2 || len(got.RimEdges) != 2 {
			t.Fatal("filled crease owners", e, got)
		}
		capGot, e := f.ClipCapLineRational(ctx, independentAnisotropicWorld(m, cases[0].p, false), independentAnisotropicWorld(m, cases[0].d, true), cases[0].lo, cases[0].hi, o)
		if e != nil || !reflect.DeepEqual(capGot, capOriginal) {
			t.Fatal("filled world cap", e)
		}
		filledGot, e := f.ClipLineRational(ctx, independentAnisotropicWorld(m, cases[1].p, false), independentAnisotropicWorld(m, cases[1].d, true), cases[1].lo, cases[1].hi, o)
		if e != nil || !reflect.DeepEqual(filledGot, filledLine) {
			t.Fatal("filled oblique", e)
		}
		horizontalGot, e := f.ClipHorizontalLineRational(ctx, independentAnisotropicWorld(m, horizontalPoint, false), independentAnisotropicWorld(m, cases[0].d, true), r(-4, 1), r(4, 1), horizontalOptions)
		if e != nil || !reflect.DeepEqual(horizontalGot, horizontal) {
			t.Fatal("filled horizontal", e)
		}
		generatorGot, e := f.ClipGeneratorRational(ctx, sourceProof.History.WallFaces[0], independentAnisotropicWorld(m, generatorPoint, false), independentAnisotropicWorld(m, generatorDirection, true), r(0, 1), r(1, 1), WrappedAnisotropicGeneratorOptions{})
		if e != nil || !reflect.DeepEqual(generatorGot, generator) {
			t.Fatal("filled generator", e)
		}
		actual, e := q.Pose()
		if e != nil || !reflect.DeepEqual(actual, m) {
			t.Fatal("pose recovery", e)
		}
		actual[0][0].SetInt64(999)
		again, e := q.Pose()
		if e != nil || !reflect.DeepEqual(again, m) {
			t.Fatal("mutable pose", e)
		}
		if len(q.Source().faces) != len(bore.faces) {
			t.Fatal("source identity")
		}
		if _, e = bore.PlaceWrappedAnisotropicStation(ctx, pose, WrappedRationalStationOccurrenceOptions{}); e == nil {
			t.Fatal("bore accepted as filled")
		}
		if _, e = filled.PlaceWrappedAnisotropicStationBore(ctx, pose, WrappedRationalStationOccurrenceOptions{}); e == nil {
			t.Fatal("filled accepted as bore")
		}
	}
}

func TestAnisotropicOccurrenceDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMeasuredBore(t)
	local := [3]*big.Rat{r(1, 1), r(-1, 2), r(-17, 4)}
	o := WrappedAnisotropicBoreDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: 1e-4}}
	for _, m := range anisotropicOccurrenceMatrices() {
		pose, e := placement.NewRationalRigid(m, 16384)
		if e != nil {
			t.Fatal(e)
		}
		q, e := s.PlaceWrappedAnisotropicStationBore(ctx, pose, WrappedRationalStationOccurrenceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p := independentAnisotropicWorld(m, local, false)
		got, e := q.BoundBoundaryDistanceRational(ctx, p, o)
		if e != nil {
			t.Fatal(e)
		}
		if got.Original.Lower.Cmp(r(1, 16)) != 0 || got.World.Lower.Cmp(got.Original.Lower) != 0 || got.World.Upper.Cmp(got.Original.Upper) != 0 || got.World.WitnessFace != got.Original.WitnessFace || got.World.ExactOperations != got.Original.ExactOperations {
			t.Fatal("bound invariance")
		}
		for i, face := range got.World.PerFace {
			original := got.Original.PerFace[i]
			want := independentAnisotropicWorld(m, original.Witness, false)
			if !reflect.DeepEqual(face.Witness, want) || face.Face != original.Face || face.Lower.Cmp(original.Lower) != 0 || face.Upper.Cmp(original.Upper) != 0 {
				t.Fatal("world witness/owner")
			}
			distance := new(big.Rat)
			for axis := range p {
				d := new(big.Rat).Sub(p[axis], face.Witness[axis])
				distance.Add(distance, new(big.Rat).Mul(d, d))
			}
			if distance.Cmp(face.Upper) != 0 {
				t.Fatal("independent world squared distance")
			}
		}
		got.Original.Lower.SetInt64(-1)
		got.Original.Witness[0].SetInt64(999)
		if got.World.Lower.Sign() < 0 || got.World.Witness[0].Cmp(r(999, 1)) == 0 {
			t.Fatal("alias")
		}
		badOptions := o
		badOptions.Search.MaxOperations = 1
		bad, e := q.BoundBoundaryDistanceRational(ctx, p, badOptions)
		if e == nil || bad.Original.Lower != nil || bad.World.Lower != nil {
			t.Fatal("atomic source quota", e)
		}
		bounded, e := s.PlaceWrappedAnisotropicStationBore(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
		if e != nil {
			t.Fatal(e)
		}
		huge := p
		huge[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 100))
		bad, e = bounded.BoundBoundaryDistanceRational(ctx, huge, o)
		if e == nil || bad.Original.Lower != nil || bad.World.Lower != nil {
			t.Fatal("transform quota", e)
		}
	}
	zero := WrappedAnisotropicBoreOccurrence{}
	if _, e := zero.BoundBoundaryDistanceRational(ctx, local, o); e == nil {
		t.Fatal("zero occurrence")
	}
}

func TestAnisotropicOccurrenceVoidDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	build := func(scale [2]float64) Solid {
		w := anisotropicStationBand(t, -1, 1, scale, scale, .75)
		s, _, e := NewWrappedAnisotropicStationLoft(ctx, []*geom.WrappedSurfacePatch{w}, 1e-8, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s, _, e := CutWrappedAnisotropicStationLofts(ctx, build([2]float64{3, 2}), build([2]float64{1, .5}), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m := anisotropicOccurrenceMatrices()[1]
	pose, e := placement.NewRationalRigid(m, 16384)
	if e != nil {
		t.Fatal(e)
	}
	q, e := s.PlaceWrappedAnisotropicStationBore(ctx, pose, WrappedRationalStationOccurrenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := independentAnisotropicWorld(m, [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}, false)
	o := WrappedAnisotropicBoreDistanceOptions{Search: geom.SurfaceDistanceOptions{SquaredTolerance: .01}}
	result, e := q.BoundBoundaryDistanceRational(ctx, p, o)
	if e != nil {
		t.Fatal(e)
	}
	a, b := result.Original, result.World
	if a.Lower.Cmp(a.Upper) >= 0 || a.Lower.Cmp(b.Lower) != 0 || a.Upper.Cmp(b.Upper) != 0 || b.Lower.Cmp(r(1, 4)) > 0 || b.Upper.Cmp(r(1, 4)) < 0 || a.WitnessFace != b.WitnessFace || a.ExactOperations != b.ExactOperations || a.VisitedCells != b.VisitedCells {
		t.Fatal("nonzero enclosure invariance")
	}
	want := independentAnisotropicWorld(m, a.Witness, false)
	if !reflect.DeepEqual(want, b.Witness) {
		t.Fatal("noncircular witness world map")
	}
	distance := new(big.Rat)
	for i := range p {
		d := new(big.Rat).Sub(p[i], b.Witness[i])
		distance.Add(distance, new(big.Rat).Mul(d, d))
	}
	if distance.Cmp(b.Upper) != 0 {
		t.Fatal("exact world upper")
	}
	cancel, c := context.WithCancel(ctx)
	c()
	empty, e := q.BoundBoundaryDistanceRational(cancel, p, o)
	if e == nil || empty.Original.Lower != nil || empty.World.Lower != nil {
		t.Fatal("cancel")
	}
}
