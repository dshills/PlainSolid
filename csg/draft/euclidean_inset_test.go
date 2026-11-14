package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func insetTriangleFixture(t *testing.T) brep.Solid {
	t.Helper()
	s, err := brep.Prism([]geom.Vec3{{}, {X: 4}, {Y: 4}}, geom.Vec3{Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStableEuclideanInsetIrrationalNormal(t *testing.T) {
	ctx := context.Background()
	source := insetTriangleFixture(t)
	out, err := InsetStableEuclidean(ctx, source, .25, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Radicand.Cmp(big.NewRat(2, 1)) != 0 || out.ExactVolume.Rational.Cmp(big.NewRat(297, 32)) != 0 ||
		out.ExactVolume.Coefficient.Cmp(big.NewRat(-21, 16)) != 0 || out.MaxSquaredDisplacement.Sign() <= 0 {
		t.Fatal("incorrect exact irrational inset volume", out.Radicand, out.ExactVolume, out.MaxSquaredDisplacement)
	}
	if len(out.ExactVertices) != 6 || len(out.FaceSources) != 5 || out.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("original topology was lost", out.Solid.Topology())
	}
	for i, mapped := range out.OldToNew {
		if mapped < 0 || len(out.FaceSources[mapped]) != 1 || out.FaceSources[mapped][0] != i {
			t.Fatal("lost original support identity", i, mapped)
		}
	}
	var maxX float64
	for i, p := range out.Solid.Vertices() {
		maxX = max(maxX, p.X)
		if p.X < .25 || p.Y < .25 || p.Z != .25 && p.Z != 1.75 || p.X+p.Y > 4-.25*math.Sqrt2+1e-15 {
			t.Fatal("offset vertex is outside analytic erosion", i, p)
		}
	}
	if math.Abs(maxX-(3.75-.25*math.Sqrt2)) > 2e-15 {
		t.Fatal("did not move hypotenuse by Euclidean distance", maxX)
	}
	mass, err := out.Solid.MassProperties()
	want := .75 * math.Pow(3.5-.25*math.Sqrt2, 2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
		t.Fatal("native physical volume", mass, want, err)
	}
}

func TestStableEuclideanInsetRationalAndAtomic(t *testing.T) {
	ctx := context.Background()
	box, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	out, err := InsetStableEuclidean(ctx, box, .125, Options{})
	if err != nil || out.ExactVolume.Rational.Cmp(big.NewRat(231, 64)) != 0 || out.ExactVolume.Coefficient.Sign() != 0 || out.MaxSquaredDisplacement.Sign() != 0 {
		t.Fatal("exact rational cuboid", out.ExactVolume, err)
	}
	triangle := insetTriangleFixture(t)
	for _, tc := range []struct {
		ctx      context.Context
		distance float64
		o        Options
		want     error
	}{
		{ctx, 1, Options{}, ErrUnsupported},
		{ctx, -.1, Options{}, ErrInvalid},
		{ctx, .25, Options{MaxWork: 1}, ErrLimit},
		{ctx, .25, Options{MaxCoordinateError: 1e-20}, ErrPrecision},
	} {
		got, err := InsetStableEuclidean(tc.ctx, triangle, tc.distance, tc.o)
		if !errors.Is(err, tc.want) || len(got.ExactVertices) != 0 || got.Radicand != nil {
			t.Fatal("inset failure exposed a partial result", err, tc.want)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := InsetStableEuclidean(cancelled, triangle, .25, Options{}); !errors.Is(err, context.Canceled) || len(got.ExactVertices) != 0 {
		t.Fatal("cancelled inset exposed a partial result", err)
	}
}

func TestStableEuclideanInsetMultipleRadicals(t *testing.T) {
	ctx := context.Background()
	polygon := []geom.Vec3{{}, {X: 4}, {X: 3, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	body, err := brep.Prism(polygon, geom.Vec3{Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out, err := InsetStableEuclidean(ctx, body, .125, Options{}); !errors.Is(err, ErrUnsupported) || len(out.ExactVertices) != 0 {
		t.Fatal("independent irrational normal classes must remain unresolved", err)
	}
}

func TestStableEuclideanInsetAlgebraicTwoIndependentNormals(t *testing.T) {
	ctx := context.Background()
	profile := []geom.Vec3{{}, {X: 4}, {X: 3, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	source, err := brep.Prism(profile, geom.Vec3{Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	const distance = .125
	out, err := InsetStableEuclideanAlgebraic(ctx, source, distance, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Radicands) != 4 || len(out.ExactVertices) != 10 || len(out.FacePlanes) != 7 || out.MaxSquaredDisplacement.Sign() <= 0 {
		t.Fatal("missing independent basis or ideal geometry", out.Radicands, len(out.ExactVertices), len(out.FacePlanes))
	}
	eval := func(q AlgebraicInsetScalar) float64 {
		t.Helper()
		var value float64
		for i, coefficient := range q.Coefficients {
			a, _ := coefficient.Float64()
			d, _ := out.Radicands[i].Float64()
			value += a * math.Sqrt(d)
		}
		return value
	}
	// Independent formula for the five intersections of the inset support lines.
	p, q := 4-distance*math.Sqrt2, 5-distance*math.Sqrt(5)
	upper := 2 - distance
	corners := [5][2]float64{{distance, distance}, {p - distance, distance}, {2*p - q, q - p}, {q - 2*upper, upper}, {distance, upper}}
	var doubleArea float64
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		doubleArea += a[0]*b[1] - a[1]*b[0]
	}
	want := doubleArea * (1 - distance)
	native, err := out.Solid.MassProperties()
	if err != nil || math.Abs(native.Volume-want) > 1e-12 || math.Abs(eval(out.ExactVolume)-want) > 1e-12 {
		t.Fatal("ideal or realized volume disagrees with independent support intersections", eval(out.ExactVolume), native, want, err)
	}
	for _, vertex := range out.ExactVertices {
		x, y, z := eval(vertex[0]), eval(vertex[1]), eval(vertex[2])
		found := false
		for _, corner := range corners {
			found = found || math.Hypot(x-corner[0], y-corner[1]) < 2e-15
		}
		if !found || (z != distance && z != 2-distance) {
			t.Fatal("algebraic offset vertex disagrees with independent supports", x, y, z)
		}
	}
	old, err := InsetStableEuclidean(ctx, insetTriangleFixture(t), .25, Options{})
	if err != nil {
		t.Fatal(err)
	}
	same, err := InsetStableEuclideanAlgebraic(ctx, insetTriangleFixture(t), .25, Options{})
	if err != nil || len(same.Radicands) != 2 || same.ExactVolume.Coefficients[0].Cmp(old.ExactVolume.Rational) != 0 ||
		same.ExactVolume.Coefficients[1].Cmp(old.ExactVolume.Coefficient) != 0 {
		t.Fatal("one-generator result changed", err)
	}
}

func TestInsetRadicalFieldDetectsDependentRoots(t *testing.T) {
	f := insetRadicalField{work: &budget{ctx: context.Background(), o: Options{MaxWork: 10000, MaxBits: 4096}}, radicals: []*big.Rat{big.NewRat(1, 1)}}
	for _, d := range []int64{2, 3, 6, 24} {
		idx, coefficient, err := f.includeRoot(big.NewRat(d, 1), 2)
		if err != nil {
			t.Fatal(err)
		}
		if (d == 6 && (idx != 3 || coefficient.Cmp(big.NewRat(1, 1)) != 0)) || (d == 24 && (idx != 3 || coefficient.Cmp(big.NewRat(2, 1)) != 0)) {
			t.Fatal("failed to share a rational square class", d, idx, coefficient)
		}
	}
	if len(f.radicals) != 4 {
		t.Fatal("dependent generators enlarged the field")
	}
	var a insetAlgebraic
	f.addTerm(&a, 0, big.NewRat(-3, 1))
	f.addTerm(&a, 1, big.NewRat(1, 1))
	f.addTerm(&a, 2, big.NewRat(1, 1))
	if f.sign(a) != 1 || f.sign(f.scaled(a, big.NewRat(-1, 1))) != -1 || f.sign(f.minus(a, a)) != 0 {
		t.Fatal("incorrect sign isolation or exact symbolic cancellation")
	}
}

func TestConvexEuclideanInsetRemovesSupport(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name                   string
		clipA, clipB, distance float64
		want                   *big.Rat
	}{
		{"irrational diagonal already redundant", 3.75, 3.75, .5, big.NewRat(18, 1)},
		{"Pythagorean diagonal at exact extinction", 3.8125, 3.75, .375, big.NewRat(1521, 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := []geom.Vec3{{}, {X: 4}, {X: 4, Y: test.clipA}, {X: test.clipB, Y: 4}, {Y: 4}}
			source, err := brep.Prism(profile, geom.Vec3{Z: 3})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := InsetStableEuclideanAlgebraic(ctx, source, test.distance, Options{}); !errors.Is(err, ErrUnsupported) {
				t.Fatal("stable incidence reader must detect the vanished face", err)
			}
			out, err := InsetConvexEuclideanAlgebraic(ctx, source, test.distance, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.ExactVertices) != 8 || len(out.FacePlanes) != 6 || out.ExactVolume.Coefficients[0].Cmp(test.want) != 0 {
				t.Fatal("incorrect exact topology after inset", out.ExactVolume, out.Solid.Topology())
			}
			for _, coefficient := range out.ExactVolume.Coefficients[1:] {
				if coefficient.Sign() != 0 {
					t.Fatal("vanished support still affects exact volume", out.ExactVolume)
				}
			}
			vanished := 0
			for i, mapped := range out.OldToNew {
				if mapped < 0 {
					vanished++
					plane, err := source.FacePlane(i)
					if err != nil || plane.Normal().X <= 0 || plane.Normal().Y <= 0 {
						t.Fatal("lost incorrect support", i, plane, err)
					}
					continue
				}
				if len(out.FaceSources[mapped]) != 1 || out.FaceSources[mapped][0] != i {
					t.Fatal("retained support history mismatch", i, mapped)
				}
			}
			if vanished != 1 {
				t.Fatal("expected exactly the vanished chamfer", out.OldToNew)
			}
			mass, err := out.Solid.MassProperties()
			want, _ := test.want.Float64()
			if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
				t.Fatal("native box volume changed after face extinction", mass, want, err)
			}
		})
	}
}

func TestConvexEuclideanInsetRegularizedEmpty(t *testing.T) {
	ctx := context.Background()
	source := insetTriangleFixture(t)
	for _, test := range []struct {
		solid    brep.Solid
		distance float64
	}{{source, 1}, {source, 2}, {brep.Solid{}, .125}} {
		out, err := InsetConvexEuclideanAlgebraic(ctx, test.solid, test.distance, Options{})
		if err != nil || len(out.Solid.Faces())+len(out.ExactVertices)+len(out.FacePlanes) != 0 || out.ExactVolume.Coefficients[0].Sign() != 0 || out.MaxSquaredDisplacement.Sign() != 0 {
			t.Fatal("collapsed or empty inset must have exact zero volume", out, err)
		}
		for _, mapped := range out.OldToNew {
			if mapped != -1 {
				t.Fatal("empty inset retained a face", out.OldToNew)
			}
		}
	}
}

func TestConvexEuclideanIndependentSupportOffsets(t *testing.T) {
	ctx := context.Background()
	source := insetTriangleFixture(t)
	diagonal, lowerY := -1, -1
	for i := range source.Faces() {
		p, err := source.FacePlane(i)
		if err != nil {
			t.Fatal(err)
		}
		n := p.Normal()
		if n.X > 0 && n.Y > 0 {
			diagonal = i
		}
		if n == (geom.Vec3{Y: -1}) {
			lowerY = i
		}
	}
	if diagonal < 0 || lowerY < 0 {
		t.Fatal("missing independent original supports", diagonal, lowerY)
	}
	got, err := OffsetConvexEuclideanSupportsAlgebraic(ctx, source, []FaceNormalOffset{{lowerY, .5}, {diagonal, -.25}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Radicands) != 2 || got.Radicands[1].Cmp(big.NewRat(2, 1)) != 0 || len(got.ExactVertices) != 6 ||
		got.ExactVolume.Coefficients[0].Cmp(big.NewRat(163, 8)) != 0 || got.ExactVolume.Coefficients[1].Cmp(big.NewRat(-9, 4)) != 0 {
		t.Fatal("independently shifted irrational triangle has an incorrect exact volume", got.ExactVolume, got.Radicands)
	}
	if moved := got.FacePlanes[got.OldToNew[lowerY]].Offset.Coefficients; moved[0].Cmp(big.NewRat(1, 2)) != 0 || moved[1].Sign() != 0 {
		t.Fatal("outward shift of original lower Y face did not survive", moved)
	}
	if moved := got.FacePlanes[got.OldToNew[diagonal]].Offset.Coefficients; moved[0].Cmp(big.NewRat(4, 1)) != 0 || moved[1].Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatal("diagonal inward shift did not use its unit normal", moved)
	}
	for i, v := range got.ExactVertices {
		if z := v[2].Coefficients; z[1].Sign() != 0 || z[0].Sign() != 0 && z[0].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("a fixed original cap moved", i, z)
		}
	}
	for i, mapped := range got.OldToNew {
		if mapped < 0 || len(got.FaceSources[mapped]) != 1 || got.FaceSources[mapped][0] != i {
			t.Fatal("offset broke original face ancestry", got.OldToNew, got.FaceSources)
		}
	}
	mass, err := got.Solid.MassProperties()
	want := math.Pow(4.5-.25*math.Sqrt2, 2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
		t.Fatal("native solid volume disagrees with independent footprint calculation", mass, want, err)
	}
	collapsed, err := OffsetConvexEuclideanSupportsAlgebraic(ctx, source, []FaceNormalOffset{{lowerY, -4}}, Options{})
	if err != nil || len(collapsed.Solid.Faces()) != 0 || collapsed.ExactVolume.Coefficients[0].Sign() != 0 {
		t.Fatal("touching supports must give a regularized empty solid", collapsed, err)
	}
	for _, face := range collapsed.OldToNew {
		if face != -1 {
			t.Fatal("collapsed boundary retained an original support", collapsed.OldToNew)
		}
	}
}

func TestConvexEuclideanSupportOffsetsCoplanarAndAtomic(t *testing.T) {
	ctx := context.Background()
	box := triangulatedSolidFixture(t, cube(t))
	var top []int
	for i := range box.Faces() {
		p, err := box.FacePlane(i)
		if err != nil {
			t.Fatal(err)
		}
		if p.Normal() == (geom.Vec3{Z: 1}) {
			top = append(top, i)
		}
	}
	if len(top) != 2 {
		t.Fatal("incorrect coplanar source fixture", top)
	}
	got, err := OffsetConvexEuclideanSupportsAlgebraic(ctx, box, []FaceNormalOffset{{top[0], .25}, {top[1], .25}}, Options{})
	if err != nil || len(got.FaceSources) != 6 || got.OldToNew[top[0]] != got.OldToNew[top[1]] || got.ExactVolume.Coefficients[0].Cmp(big.NewRat(5, 4)) != 0 {
		t.Fatal("congruent coplanar offsets should retain both source faces", got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, test := range []struct {
		ctx     context.Context
		offsets []FaceNormalOffset
		want    error
	}{
		{ctx, []FaceNormalOffset{{top[0], .25}}, ErrUnsupported},
		{ctx, []FaceNormalOffset{{top[0], .25}, {top[1], .125}}, ErrUnsupported},
		{ctx, []FaceNormalOffset{{top[0], .25}, {top[0], .25}}, ErrInvalid},
		{ctx, []FaceNormalOffset{{-1, .25}}, ErrInvalid},
		{ctx, []FaceNormalOffset{{top[0], math.Inf(1)}}, ErrInvalid},
		{cancelled, []FaceNormalOffset{{top[0], .25}}, context.Canceled},
	} {
		failed, err := OffsetConvexEuclideanSupportsAlgebraic(test.ctx, box, test.offsets, Options{})
		if !errors.Is(err, test.want) || len(failed.Radicands)+len(failed.ExactVertices)+len(failed.Solid.Faces()) != 0 {
			t.Fatal("failed offset returned partial geometry", failed, err, test.want)
		}
	}
}
