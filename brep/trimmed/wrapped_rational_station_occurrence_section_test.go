package trimmed

import (
	"context"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestRationalStationOccurrenceSections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	body := stationMassFixture(t, false, 0)
	faces := body.Faces()
	slices.Reverse(faces)
	body, e := NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 3)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 7)}, {r(-4, 5), r(0, 1), r(3, 5), r(5, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	q, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Add(r(1, 3), new(big.Rat).Add(new(big.Rat).Mul(r(3, 5), p[0]), new(big.Rat).Mul(r(4, 5), p[2]))), new(big.Rat).Add(r(-2, 7), p[1]), new(big.Rat).Add(r(5, 11), new(big.Rat).Add(new(big.Rat).Mul(r(-4, 5), p[0]), new(big.Rat).Mul(r(3, 5), p[2])))}
	}
	for _, height := range []*big.Rat{r(0, 1), r(1, 3), r(1, 1), r(7, 3), r(3, 1)} {
		section, e := q.SectionRational(ctx, height, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		original, e := body.SectionWrappedRationalStationLoftRational(ctx, height, geom.SurfacePatchOptions{})
		if e != nil || section.Original.Cap != original.Cap || !reflect.DeepEqual(section.Original.Outer.Faces, original.Outer.Faces) {
			t.Fatal("original owners", section, e)
		}
		lo, hi := section.Curve.Domain()
		if lo != 0 || hi != 4 {
			t.Fatal("original U domain", lo, hi)
		}
		if section.Normal[0].Cmp(r(4, 5)) != 0 || section.Normal[1].Sign() != 0 || section.Normal[2].Cmp(r(3, 5)) != 0 {
			t.Fatal("world section plane")
		}
		for axis := 0; axis < 3; axis++ {
			j, k := (axis+1)%3, (axis+2)%3
			cross := new(big.Rat).Sub(new(big.Rat).Mul(section.U[j], section.V[k]), new(big.Rat).Mul(section.U[k], section.V[j]))
			if cross.Cmp(section.Normal[axis]) != 0 {
				t.Fatal("oriented exact basis")
			}
		}
		var first, last [4]*big.Rat
		for quarter := 0; quarter < 4; quarter++ {
			piece, e := section.Curve.Restrict(float64(quarter), float64(quarter+1), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			net, _, e := piece.BezierControlNet(ctx, geom.SurfacePatchOptions{})
			if e != nil || len(net.Control) != 3 {
				t.Fatal(e)
			}
			// Every homogeneous control lies exactly in the claimed world plane:
			// linearity and positive weights prove the entire rational curve does.
			for _, h := range net.Control {
				plane := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					d := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(section.Origin[axis], h[3]))
					plane.Add(plane, new(big.Rat).Mul(section.Normal[axis], d))
				}
				if plane.Sign() != 0 {
					t.Fatal("full original curve left world plane")
				}
			}
			if quarter == 0 {
				first = net.Control[0]
			} else {
				for axis := 0; axis < 3; axis++ {
					if new(big.Rat).Mul(last[axis], net.Control[0][3]).Cmp(new(big.Rat).Mul(net.Control[0][axis], last[3])) != 0 {
						t.Fatal("lost exact quadrant junction")
					}
				}
			}
			last = net.Control[2]
			if quarter == 0 {
				var h [4]*big.Rat
				for k := range h {
					h[k] = new(big.Rat)
					for j, w := range []*big.Rat{r(1, 4), r(1, 2), r(1, 4)} {
						h[k].Add(h[k], new(big.Rat).Mul(w, net.Control[j][k]))
					}
				}
				offset := new(big.Rat).Mul(r(5, 7), original.Outer.Section.Radius)
				want := world([3]*big.Rat{new(big.Rat).Add(r(1, 4), offset), new(big.Rat).Add(r(-1, 2), offset), height})
				for axis := range want {
					if new(big.Rat).Quo(h[axis], h[3]).Cmp(want[axis]) != 0 {
						t.Fatal("exact noncircular world midpoint")
					}
				}
			}
		}
		for axis := 0; axis < 3; axis++ {
			if new(big.Rat).Mul(first[axis], last[3]).Cmp(new(big.Rat).Mul(last[axis], first[3])) != 0 {
				t.Fatal("open world section")
			}
		}
		// A returned plane vector cannot mutate immutable curve coefficients.
		saved, _ := section.Curve.Eval(.5)
		section.Origin[0].SetInt64(99)
		again, _ := section.Curve.Eval(.5)
		if saved != again {
			t.Fatal("plane/curve alias")
		}
	}
	for _, height := range []*big.Rat{r(-1, 1), r(4, 1)} {
		if got, e := q.SectionRational(ctx, height, geom.SurfacePatchOptions{}); e == nil || got.Curve != nil || got.Original.Outer.Section.Curve != nil {
			t.Fatal("out-of-domain partial", e)
		}
	}
	if got, e := q.SectionRational(ctx, r(1, 3), geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || got.Curve != nil {
		t.Fatal("source quota", e)
	}
	low, e := body.PlaceWrappedRationalStation(ctx, pose, WrappedRationalStationOccurrenceOptions{MaxTransformBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	if _, e := body.SectionWrappedRationalStationLoftRational(ctx, tiny, geom.SurfacePatchOptions{}); e != nil {
		t.Fatal(e)
	}
	if got, e := low.SectionRational(ctx, tiny, geom.SurfacePatchOptions{}); e == nil || got.Curve != nil || got.Original.Outer.Section.Curve != nil {
		t.Fatal("atomic world coefficient quota", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := q.SectionRational(cancelled, r(1, 3), geom.SurfacePatchOptions{}); e == nil || got.Curve != nil {
		t.Fatal("cancel")
	}
}
