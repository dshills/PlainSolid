package parametric

import (
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func sphereOccurrencePose(t *testing.T) placement.RationalRigid {
	t.Helper()
	r := big.NewRat
	p, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSpherePairOccurrenceNearest(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := sphereOccurrencePose(t)
	for _, tc := range []struct {
		name string
		a, b Ball
		op   BooleanOp
		p    [3]*big.Rat
	}{
		{"irrational rim", Ball{Center: geom.Vec3{X: -1}, Radius: 2}, Ball{Center: geom.Vec3{X: 1}, Radius: 2}, Union, [3]*big.Rat{r(0, 1), r(1, 1), r(1, 1)}},
		{"circle", Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, Union, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}},
		{"cap", Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, Union, [3]*big.Rat{r(-3, 1), r(0, 1), r(0, 1)}},
		{"poles", Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, Intersection, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}},
		{"separated", Ball{Radius: 2}, Ball{Center: geom.Vec3{X: 6}, Radius: 1}, Union, [3]*big.Rat{r(7, 2), r(0, 1), r(0, 1)}},
		{"cavity family", Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 1}, Radius: 1}, Difference, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}},
		{"coincident family", Ball{Radius: 2}, Ball{Radius: 2}, Union, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := NewSpherePairSet(ctx, tc.a, tc.b, tc.op, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			occ, e := s.PlaceRigid(ctx, pose)
			if e != nil {
				t.Fatal(e)
			}
			wp, e := pose.TransformPoint(ctx, tc.p, false, 4096)
			if e != nil {
				t.Fatal(e)
			}
			got, e := occ.NearestBoundaryRational(ctx, wp, geom.SphereFullBoundaryDistanceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			base, e := s.NearestBoundaryRational(ctx, tc.p, geom.SphereFullBoundaryDistanceOptions{})
			if e != nil || !reflect.DeepEqual(base, got.Base) {
				t.Fatal("source changed", e)
			}
			if got.World.Lower.Cmp(base.Geometry.Lower) != 0 || got.World.Upper.Cmp(base.Geometry.Upper) != 0 || got.World.Owners != base.Geometry.Owners {
				t.Fatal("distance/owners changed")
			}
			// Pull each world symbolic witness/family back. This checks complete fields,
			// including plane inequalities, algebraic vectors, normal signs and all ties.
			m, _ := pose.Matrix()
			var inv [3][4]*big.Rat
			for i := 0; i < 3; i++ {
				inv[i][3] = new(big.Rat)
				for j := 0; j < 3; j++ {
					inv[i][j] = new(big.Rat).Set(m[j][i])
					inv[i][3].Sub(inv[i][3], new(big.Rat).Mul(m[j][i], m[j][3]))
				}
			}
			ip, e := placement.NewRationalRigid(inv, 4096)
			if e != nil {
				t.Fatal(e)
			}
			back, e := mapSphereNearest(ctx, ip, got.World)
			if e != nil || !sphereOccurrenceEqual(back, base.Geometry) {
				t.Fatal("symbolic inverse mismatch", e)
			}
			// Independent coordinate formula for the common circle center and normal.
			if got.World.Axis != nil {
				f := got.World.Axis.Families[2]
				if f.Center[0].Cmp(r(1, 7)) != 0 || f.Center[1].Cmp(r(-2, 9)) != 0 || f.Center[2].Cmp(r(3, 11)) != 0 {
					t.Fatal("circle translation")
				}
				if f.Normal[0].Cmp(r(18, 13)) != 0 || f.Normal[1].Cmp(r(216, 65)) != 0 || f.Normal[2].Cmp(r(-24, 5)) != 0 {
					t.Fatal("circle plane rotation")
				}
			}
			got.World.Lower.SetInt64(-99)
			if got.World.Isolated != nil {
				got.World.Isolated.Candidates[2].Radicand.SetInt64(99)
				got.World.Isolated.Candidates[2].Center[0].SetInt64(99)
			}
			if got.World.Axis != nil {
				got.World.Axis.Families[2].RadiusSquared.SetInt64(99)
				got.World.Axis.Families[2].Normal[0].SetInt64(99)
			}
			again, e := occ.NearestBoundaryRational(ctx, wp, geom.SphereFullBoundaryDistanceOptions{})
			if e != nil || !reflect.DeepEqual(again.Base, base) || got.Base.Geometry.Lower.Sign() < 0 {
				t.Fatal("result aliases source/base", e)
			}
		})
	}
}

func TestSpherePairOccurrenceLineAndGuards(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := sphereOccurrencePose(t)
	s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: -3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3}, Radius: 5}, Difference, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	occ, e := s.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
	d := [3]*big.Rat{r(2, 3), r(0, 1), r(0, 1)}
	wp, _ := pose.TransformPoint(ctx, p, false, 4096)
	wd, _ := pose.TransformVector(ctx, d, false, 4096)
	g, e := occ.ClipRationalLine(ctx, wp, wd, r(-20, 1), r(20, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.ClipRationalLine(ctx, p, d, r(-20, 1), r(20, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || !reflect.DeepEqual(g, b) || len(g.Intervals) != 1 {
		t.Fatal("raw t/owner change", e)
	}
	if _, e = s.PlaceRigid(ctx, placement.RationalRigid{}); e == nil {
		t.Fatal("zero pose")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if g, e := occ.NearestBoundaryRational(c, wp, geom.SphereFullBoundaryDistanceOptions{}); e == nil || !reflect.DeepEqual(g, SpherePairOccurrenceNearest{}) {
		t.Fatal("cancellation")
	}
	huge := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4097))
	wp[0] = huge
	if g, e := occ.NearestBoundaryRational(ctx, wp, geom.SphereFullBoundaryDistanceOptions{}); e == nil || !reflect.DeepEqual(g, SpherePairOccurrenceNearest{}) {
		t.Fatal("pose bit limit")
	}
	reflection, _ := pose.Matrix()
	for j := 0; j < 3; j++ {
		reflection[0][j].Neg(reflection[0][j])
	}
	if _, e := placement.NewRationalRigid(reflection, 4096); e == nil {
		t.Fatal("reflection accepted")
	}
	m, _ := occ.Pose()
	m[0][0].SetInt64(17)
	mm, _ := occ.Pose()
	if mm[0][0].Cmp(r(3, 13)) != 0 {
		t.Fatal("pose alias")
	}
}

func sphereOccurrenceEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
