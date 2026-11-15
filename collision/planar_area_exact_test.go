package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestExactPlanarAreaBoundsPreserveIrrationalFaceAreas(t *testing.T) {
	ctx := context.Background()
	profile := []geom.Vec3{{}, {X: 2}, {X: 3, Y: 2}, {X: 1, Y: 2}}
	s, err := brep.Prism(profile, geom.Vec3{Z: 3})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := l.SurfaceAreaExactBounds(ctx, ExactAreaOptions{PrecisionBits: 60})
	if err != nil || len(r.Faces) != 6 || r.Operations <= 0 {
		t.Fatalf("irrational face area proof: %+v, %v", r, err)
	}
	const prec = 220
	f := func(x int64) *big.Float { return new(big.Float).SetPrec(prec).SetInt64(x) }
	sqrt5 := new(big.Float).SetPrec(prec).Sqrt(f(5))
	expected, _ := new(big.Float).SetPrec(prec).Add(f(20), new(big.Float).SetPrec(prec).Mul(f(6), sqrt5)).Rat(nil)
	if r.Area.Lower.Cmp(expected) >= 0 || r.Area.Upper.Cmp(expected) <= 0 || new(big.Rat).Sub(r.Area.Upper, r.Area.Lower).Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 60))) > 0 {
		t.Fatalf("bounds do not tightly enclose 20+6√5: %+v", r.Area)
	}
	sumLo, sumHi, exactly := new(big.Rat), new(big.Rat), 0
	for i, face := range r.Faces {
		if face.Solid != 0 || face.Face != i {
			t.Fatalf("face ancestry not stable: %+v", face)
		}
		sumLo.Add(sumLo, face.Area.Lower)
		sumHi.Add(sumHi, face.Area.Upper)
		if face.Area.Lower.Cmp(face.Area.Upper) == 0 {
			exactly++
		}
	}
	if sumLo.Cmp(r.Area.Lower) != 0 || sumHi.Cmp(r.Area.Upper) != 0 || exactly != 4 {
		t.Fatalf("surface terms not stable: %+v", r)
	}
	if r, err := l.SurfaceAreaExactBounds(ctx, ExactAreaOptions{Arithmetic: ExactMassOptions{MaxCoefficientBits: 16}}); !errors.Is(err, ErrLimit) || r.Area.Lower != nil || r.Faces != nil {
		t.Fatalf("incomplete area bound published: %+v %v", r, err)
	}
}

func TestExactPlanarAreaCountsCavitiesAndZeroAreaVertexContacts(t *testing.T) {
	ctx := context.Background()
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	box := func(lo, hi float64) brep.Solid {
		t.Helper()
		s, err := brep.Box(xyz(lo), xyz(hi))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := locationJoin(t, box(0, 8), box(2, 6).Reversed(), box(3, 5))
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := l.SurfaceAreaExactBounds(ctx, ExactAreaOptions{})
	if err != nil || len(r.Faces) != 18 {
		t.Fatalf("nested area %+v: %v", r, err)
	}
	testMassRational(t, r.Area.Lower, big.NewRat(504, 1))
	testMassRational(t, r.Area.Upper, big.NewRat(504, 1))
	parts := []brep.Solid{box(0, 1), box(1, 2)}
	c, err := NewVertexContactAssemblyLocator(ctx, parts, []AssemblyVertexContact{assemblyContactAt(t, parts, 0, 1, xyz(1))}, VertexContactAssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = c.SurfaceAreaExactBounds(ctx, ExactAreaOptions{})
	if err != nil || len(r.Faces) != 12 {
		t.Fatalf("compound area %+v: %v", r, err)
	}
	testMassRational(t, r.Area.Lower, big.NewRat(12, 1))
	testMassRational(t, r.Area.Upper, big.NewRat(12, 1))
}
