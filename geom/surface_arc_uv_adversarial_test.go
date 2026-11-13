package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSurfaceArcUVNonseparableWeightedTranspose(t *testing.T) {
	for _, transpose := range []bool{false, true} {
		t.Run(map[bool]string{false: "U", true: "V"}[transpose], func(t *testing.T) {
			// In normalized coordinates the exact homogeneous plane residual is
			// -1+2v-uv/2, hence v=2/(4-u). Binary weights are retained
			// exactly by normalization. Nonseparable weights prevent a polynomial/axis shortcut.
			poles := make([]Vec3, 4)
			weights := make([]float64, 4)
			for i := 0; i < 2; i++ {
				for j := 0; j < 2; j++ {
					k := 2*i + j
					if transpose {
						k = 2*j + i
					}
					sign := float64(2*j - 1)
					poles[k] = Vec3{X: float64(i), Y: float64(j), Z: (sign - float64(i) - 2*float64(j)) / 4}
					weights[k] = []float64{1, 1, 1, .5}[2*i+j]
				}
			}
			s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{-2, -2, 2, 2}, []float64{3, 3, 5, 5}, weights)
			if e != nil {
				t.Fatal(e)
			}
			p, e := NewSurfacePatch(s, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			plane, e := NewRawSupportPlane(Vec3{}, Vec3{X: 4, Z: -1}, Vec3{Y: 2, Z: -1}, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			cert, e := CertifySurfaceRawPlaneArcs(p, plane, SurfacePlaneArcOptions{})
			if e != nil || !cert.Complete || len(cert.Arcs) == 0 {
				t.Fatal("regular exact weighted branch", cert.Complete, e)
			}
			covered := make(map[int]bool)
			for _, a := range cert.Arcs {
				q, e := a.NewSurfaceArcUVEncloser(context.Background(), SurfaceArcUVOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for i := 1; i < 32; i++ {
					u := big.NewRat(int64(i), 32)
					v := new(big.Rat).Quo(big.NewRat(2, 1), new(big.Rat).Sub(big.NewRat(4, 1), u))
					if transpose {
						u, v = v, u
					}
					uv := [2]*big.Rat{new(big.Rat).Add(big.NewRat(-2, 1), new(big.Rat).Mul(big.NewRat(4, 1), u)), new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Mul(big.NewRat(2, 1), v))}
					in := func(x *big.Rat, lo, hi float64) bool {
						return x.Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && x.Cmp(new(big.Rat).SetFloat64(hi)) <= 0
					}
					d := a.source.Domain()
					if !in(uv[0], d.UMin, d.UMax) || !in(uv[1], d.VMin, d.VMax) {
						continue
					}
					f, _ := uv[1-q.dep].Float64()
					b, has, e := q.Enclose(context.Background(), f-.0001, f+.0002)
					if e != nil || !has || !in(uv[0], b.Min.X, b.Max.X) || !in(uv[1], b.Min.Y, b.Max.Y) {
						t.Logf("domain=%+v dep=%d net=%v", q.domain, q.dep, q.net)
						t.Fatalf("lost exact rational branch %v: %+v %v %v", uv, b, has, e)
					}
					covered[i] = true
				}
			}
			if len(covered) != 31 {
				t.Fatalf("only covered %d/31 independent points", len(covered))
			}
		})
	}
}
