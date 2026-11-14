package placement

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestTorusCylinderOccurrenceDistance(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose, e := NewRationalRigid([3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		source, _ := geom.NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		occ, e := PlaceTorusCylinder(ctx, source, pose)
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range [][3]*big.Rat{{r(4, 1), r(1, 1), r(3, 2)}, {r(0, 1), r(0, 1), r(0, 1)}} {
			p, _ := pose.TransformPoint(ctx, q, false, 4096)
			o := geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000)}
			v, e := occ.BoundBoundaryDistance(ctx, p, o)
			if e != nil {
				t.Fatal(e)
			}
			original, e := source.BoundBoundaryDistance(ctx, q, o)
			if e != nil || !reflect.DeepEqual(v.Base, original) {
				t.Fatal("source unchanged", e)
			}
			for i, w := range v.Witnesses {
				if v.Base.Supports[i].Owner == 0 {
					continue
				}
				dot := new(big.Rat)
				norm := new(big.Rat)
				for k := 0; k < 3; k++ {
					dot.Add(dot, new(big.Rat).Mul(w.Axis[k], w.RadialVector[k]))
					norm.Add(norm, new(big.Rat).Mul(w.Axis[k], w.Axis[k]))
				}
				if dot.Sign() != 0 || norm.Cmp(r(1, 1)) != 0 {
					t.Fatal("orthonormal axis")
				}
				rho, _ := w.Base.RadialSquared.Float64()
				height, _ := w.Base.HeightBase.Float64()
				hr, _ := w.Base.HeightRadicand.Float64()
				height += float64(w.Base.HeightSign) * math.Sqrt(hr)
				qrad, _ := w.QueryRadialSquared.Float64()
				d := 0.0
				for k := 0; k < 3; k++ {
					origin, _ := w.Origin[k].Float64()
					axis, _ := w.Axis[k].Float64()
					radial, _ := w.RadialVector[k].Float64()
					if w.Base.WholeCircle {
						radial, _ = w.CircleDirections[0][k].Float64()
						radial *= math.Sqrt(rho)
					} else {
						radial *= math.Sqrt(rho / qrad)
					}
					point := origin + axis*height + radial
					query, _ := p[k].Float64()
					d += (point - query) * (point - query)
				}
				up, _ := v.Base.Supports[i].Upper.Float64()
				if d > up+1e-10 {
					t.Fatal("world attained upper", d, up)
				}
				if w.Base.WholeCircle {
					for j := 0; j < 2; j++ {
						n := new(big.Rat)
						cross := new(big.Rat)
						for k := 0; k < 3; k++ {
							n.Add(n, new(big.Rat).Mul(w.CircleDirections[j][k], w.CircleDirections[j][k]))
							cross.Add(cross, new(big.Rat).Mul(w.CircleDirections[j][k], w.Axis[k]))
						}
						if n.Cmp(r(1, 1)) != 0 || cross.Sign() != 0 {
							t.Fatal("circle basis")
						}
					}
				}
			}
			v.Witnesses[0].Base.RadialSquared.SetInt64(999)
			if !reflect.DeepEqual(v.Base, original) {
				t.Fatal("base alias")
			}
			again, e := occ.BoundBoundaryDistance(ctx, p, o)
			if e != nil || !reflect.DeepEqual(again.Base, original) {
				t.Fatal("source alias", e)
			}
			c, cancel := context.WithCancel(ctx)
			cancel()
			if bad, e := occ.BoundBoundaryDistance(c, p, o); e == nil || !reflect.DeepEqual(bad, TorusCylinderOccurrenceDistance{}) {
				t.Fatal("cancel")
			}
		}
	}
}
