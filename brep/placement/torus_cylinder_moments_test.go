package placement

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestTorusCylinderOccurrenceMoments(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	source, _ := geom.NewTorusCylinderMaterial(5, 2, 5, -3, 4, geom.TorusCylinderUnion)
	o := TorusCylinderOccurrenceMomentOptions{Source: geom.TorusCylinderMomentOptions{VolumeTolerance: r(1, 100), SecondMomentTolerance: r(1, 10), CentroidTolerance: r(1, 1000), InertiaTolerance: r(1, 5)}}
	matrices := [][3][4]*big.Rat{{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, {{r(0, 1), r(-1, 1), r(0, 1), r(1, 7)}, {r(0, 1), r(0, 1), r(1, 1), r(-2, 9)}, {r(-1, 1), r(0, 1), r(0, 1), r(3, 11)}}, {{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}}
	var last TorusCylinderOccurrence
	var good TorusCylinderOccurrenceMoments
	for index, m := range matrices {
		pose, e := NewRationalRigid(m, 4096)
		if e != nil {
			t.Fatal(e)
		}
		occ, e := PlaceTorusCylinder(ctx, source, pose)
		if e != nil {
			t.Fatal(e)
		}
		v, e := occ.BoundMoments(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		last, good = occ, v
		same := func(x, y [2]*big.Rat) bool { return x[0].Cmp(y[0]) == 0 && x[1].Cmp(y[1]) == 0 }
		if !same(v.Volume, v.Source.Volume) {
			t.Fatal("volume scale")
		}
		if index < 2 {
			permutation := [3]int{0, 1, 2}
			if index == 1 {
				permutation = [3]int{1, 2, 0}
			}
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					if i == j {
						if !same(v.InertiaCentroid[i][j], v.Source.InertiaCentroid[permutation[i]]) {
							t.Fatal("exact axis equivalence")
						}
					} else if v.InertiaCentroid[i][j][0].Sign() != 0 || v.InertiaCentroid[i][j][1].Sign() != 0 {
						t.Fatal("axis cross")
					}
				}
			}
		}
		// Independent closed c=R meridian antiderivative, then numeric tensor formula.
		R, rr := 5.0, 2.0
		cv := 175 * math.Pi
		tv := 40 * math.Pi * math.Pi
		commonV := 4 * math.Pi * (R*math.Pi*rr*rr/4 - math.Pow(rr, 3)/3)
		V := tv + cv - commonV
		commonX := 2 * math.Pi * (math.Pow(R, 3)*math.Pi*rr*rr/4 - R*R*math.Pow(rr, 3) + 3*R*math.Pi*math.Pow(rr, 4)/16 - 2*math.Pow(rr, 5)/15)
		commonZ := 4 * math.Pi / 3 * (R*3*math.Pi*math.Pow(rr, 4)/16 - math.Pow(rr, 5)/5)
		X := tv*(R*R/2+3*rr*rr/8) + cv*25/4 - commonX
		Z := tv*rr*rr/4 + cv*13/3 - commonZ
		F := cv / 2
		localQ := [3]float64{X, X, Z}
		localI := [3]float64{X + Z - F*F/V, X + Z - F*F/V, 2 * X}
		var mf [3][4]float64
		for i := range m {
			for j := range m[i] {
				mf[i][j], _ = m[i][j].Float64()
			}
		}
		check := func(v [2]*big.Rat, x float64) {
			t.Helper()
			lo, _ := v[0].Float64()
			hi, _ := v[1].Float64()
			if x < lo-1e-8 || x > hi+1e-8 {
				t.Fatal("tensor", index, lo, x, hi)
			}
		}
		for i := 0; i < 3; i++ {
			check(v.First[i], mf[i][2]*F+mf[i][3]*V)
			check(v.Centroid[i], mf[i][3]+mf[i][2]*F/V)
			for j := 0; j < 3; j++ {
				Q, I := 0.0, 0.0
				for k := 0; k < 3; k++ {
					Q += mf[i][k] * mf[j][k] * localQ[k]
					I += mf[i][k] * mf[j][k] * localI[k]
				}
				Q += (mf[i][2]*mf[j][3]+mf[j][2]*mf[i][3])*F + mf[i][3]*mf[j][3]*V
				check(v.Second[i][j], Q)
				check(v.InertiaCentroid[i][j], I)
			}
		}
	}
	bad := o
	bad.MaxTransformOperations = good.MappingOperations - 1
	if v, e := last.BoundMoments(ctx, bad); e == nil || !reflect.DeepEqual(v, TorusCylinderOccurrenceMoments{}) {
		t.Fatal("mapping limit", e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := last.BoundMoments(c, o); e == nil || !reflect.DeepEqual(v, TorusCylinderOccurrenceMoments{}) {
		t.Fatal("cancel")
	}
	good.Volume[0].SetInt64(-99)
	if good.Source.Volume[0].Sign() < 0 {
		t.Fatal("volume alias")
	}
}
