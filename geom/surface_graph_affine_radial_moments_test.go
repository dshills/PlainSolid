package geom_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineRadialPhysicalAnnulusMoments(t *testing.T) {
	ctx := context.Background()
	// Independent degree-four Bernstein identities on physical [2,4]x[-1,2].
	// Both roofs include the nonradial height 7+X/3-Y/5. Their difference is
	// H=3(q-a)(b-q), q=(X-3)^2+(Y-1/2)^2, a=25/1024,b=25/256.
	x2 := []int64{3, 0, -1, 0, 3}
	x4 := []int64{1, -1, 1, -1, 1}
	build := func(top, swap bool) *geom.SurfacePatch {
		net := make([][4]*big.Rat, 25)
		for i := range 5 {
			for j := range 5 {
				x := big.NewRat(int64(4+i), 2)
				y := big.NewRat(int64(-4+3*j), 4)
				z := new(big.Rat).Add(big.NewRat(7, 1), new(big.Rat).Quo(x, big.NewRat(3, 1)))
				z.Sub(z, new(big.Rat).Quo(y, big.NewRat(5, 1)))
				if top {
					q := new(big.Rat).Add(big.NewRat(x2[i], 3), big.NewRat(3*x2[j], 4))
					q2 := new(big.Rat).Add(big.NewRat(x4[i], 1), big.NewRat(81*x4[j], 16))
					q2.Add(q2, big.NewRat(x2[i]*x2[j], 2))
					h := new(big.Rat).Sub(q2, new(big.Rat).Mul(big.NewRat(125, 1024), q))
					h.Add(h, big.NewRat(625, 262144))
					h.Mul(h, big.NewRat(-3, 1))
					z.Add(z, h)
				}
				index := i*5 + j
				if swap {
					index = j*5 + 4 - i
				}
				net[index] = [4]*big.Rat{x, y, z, big.NewRat(1, 1)}
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 4, 4, net, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	for _, swap := range []bool{false, true} {
		proof, e := geom.CertifyAffineRadialGraphContours(ctx, build(false, swap), build(true, false), [2]*big.Rat{big.NewRat(3, 1), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(5, 32), big.NewRat(5, 16)}, geom.GraphPointContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := proof.MassMomentsBands(ctx, []int{1}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Independent beta integrals: integral H dq=L^3/2,
		// integral H^2 dq=3L^5/10 and mean q=(a+b)/2.
		L := big.NewRat(75, 1024)
		vol := new(big.Rat).Mul(L, L)
		vol.Mul(vol, L)
		vol.Quo(vol, big.NewRat(2, 1))
		zc := new(big.Rat).Mul(L, L)
		zc.Mul(zc, big.NewRat(3, 10))
		zc.Add(zc, big.NewRat(79, 10))
		eq := func(label string, a, b *big.Rat) {
			t.Helper()
			if a == nil || a.Cmp(b) != 0 {
				t.Fatal(label, a, b)
			}
		}
		eq("volume", m.Planar.Total.VolumePiCoefficient, vol)
		eq("area", m.Planar.Total.ProjectedAreaPiCoefficient, L)
		eq("centroid X", m.Planar.Total.CentroidXY[0], big.NewRat(3, 1))
		eq("centroid Y", m.Planar.Total.CentroidXY[1], big.NewRat(1, 2))
		eq("centroid Z", m.Height.CentroidZ, zc)
		eq("first Z", m.Height.FirstZPiCoefficient, new(big.Rat).Mul(vol, zc))
		for k, f := range []*big.Rat{new(big.Rat).Add(big.NewRat(9, 1), big.NewRat(125, 4096)), big.NewRat(3, 2), new(big.Rat).Add(big.NewRat(1, 4), big.NewRat(125, 4096))} {
			eq("second XY", m.Planar.Total.SecondMomentPiCoefficients[k], new(big.Rat).Mul(vol, f))
		}
		tensor, e := proof.MassTensorBands(ctx, []int{1}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Independent angular/beta integrals. Q=mean(q), E[H]=3L²/5,
		// E[H²]/3=9L⁴/70 for volume-weighted H.
		Q := big.NewRat(125, 2048)
		xz := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 1), zc), new(big.Rat).Mul(big.NewRat(1, 6), Q))
		yz := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(1, 2), zc), new(big.Rat).Mul(big.NewRat(1, 10), Q))
		zz := new(big.Rat).Add(big.NewRat(6241, 100), new(big.Rat).Mul(big.NewRat(17, 225), Q))
		l2 := new(big.Rat).Mul(L, L)
		zz.Add(zz, new(big.Rat).Mul(big.NewRat(237, 50), l2))
		zz.Add(zz, new(big.Rat).Mul(big.NewRat(9, 70), new(big.Rat).Mul(l2, l2)))
		want := [3][3]*big.Rat{}
		want[0][0] = new(big.Rat).Set(m.Planar.Total.SecondMomentPiCoefficients[0])
		want[0][1] = new(big.Rat).Set(m.Planar.Total.SecondMomentPiCoefficients[1])
		want[1][1] = new(big.Rat).Set(m.Planar.Total.SecondMomentPiCoefficients[2])
		want[0][2] = new(big.Rat).Mul(vol, xz)
		want[1][2] = new(big.Rat).Mul(vol, yz)
		want[2][2] = new(big.Rat).Mul(vol, zz)
		for i := range 3 {
			for j := 0; j < i; j++ {
				want[i][j] = new(big.Rat).Set(want[j][i])
			}
		}
		trace := new(big.Rat).Add(want[0][0], want[1][1])
		trace.Add(trace, want[2][2])
		for i := range 3 {
			for j := range 3 {
				eq("physical tensor", tensor.Total.SecondMomentPiCoefficients[i][j], want[i][j])
				inertia := new(big.Rat).Neg(want[i][j])
				if i == j {
					inertia.Add(inertia, trace)
				}
				eq("origin inertia", tensor.Total.OriginInertiaPiCoefficients[i][j], inertia)
			}
		}
		if _, e = proof.MassTensorBands(ctx, []int{1}, geom.RadialGraphMomentOptions{MaxOperations: tensor.ExactOperations}); e != nil {
			t.Fatal(e)
		}
		if bad, e := proof.MassTensorBands(ctx, []int{1}, geom.RadialGraphMomentOptions{MaxOperations: tensor.ExactOperations - 1}); e == nil || len(bad.Bands) != 0 || bad.Total.SecondMomentPiCoefficients[0][0] != nil {
			t.Fatal("tensor quota", e)
		}
		v, e := proof.BoundVolumeBands(ctx, []int{1}, geom.RadialGraphVolumeOptions{})
		if e != nil {
			t.Fatal(e)
		}
		eq("volume bound coefficient", v.PiCoefficient, vol)
		if _, e = proof.MassMomentsBands(ctx, []int{1}, geom.RadialGraphMomentOptions{MaxOperations: m.ExactOperations}); e != nil {
			t.Fatal("exact quota", e)
		}
		if bad, e := proof.MassMomentsBands(ctx, []int{1}, geom.RadialGraphMomentOptions{MaxOperations: m.ExactOperations - 1}); e == nil || bad.Height.CentroidZ != nil {
			t.Fatal("atomic quota", e)
		}
		m.Height.CentroidZ.SetInt64(999)
		again, e := proof.MassMomentsBands(ctx, []int{1}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		eq("owned result", again.Height.CentroidZ, zc)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if bad, e := proof.MassMomentsBands(cancelled, []int{1}, geom.RadialGraphMomentOptions{}); e != context.Canceled || bad.Height.CentroidZ != nil {
			t.Fatal("cancel", e)
		}
	}
}
