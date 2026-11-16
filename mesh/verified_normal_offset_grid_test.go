package mesh

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestCertifiedVerifiedNormalOffsetGrid(t *testing.T) {
	for _, rotated := range []bool{false, true} {
		t.Run(fmt.Sprint(rotated), func(t *testing.T) {
			ctx := context.Background()
			p := gridTestParabolicOffset(t, rotated)
			o := NormalOffsetGridOptions{Deflection: .22}
			m, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
			if err != nil {
				t.Fatal(err)
			}
			checkCertifiedOffsetGrid(t, p.Projection(), m)
			n := len(m.UStations) - 1
			if len(m.VStations) != n+1 || len(m.Mesh.Vertices) != (n+1)*(n+1) || len(m.Mesh.Triangles) != 2*n*n ||
				m.UStations[0] != 0 || m.VStations[0] != 0 || m.UStations[n] != 1 || m.VStations[n] != 1 ||
				m.GeometryWork <= len(m.Mesh.Vertices)*4096 {
				t.Fatal("invalid grid or work receipt", n, m.GeometryWork)
			}
			distance, _ := m.DistanceUpperSquared.Float64()
			if distance <= 0 || distance > o.Deflection*o.Deflection {
				t.Fatal("certificate exceeded requested physical distance", distance)
			}
			for _, tri := range m.Mesh.Triangles {
				var u, v float64
				var interpolated geom.Vec3
				for _, k := range tri {
					u += m.UStations[k/(n+1)]
					v += m.VStations[k%(n+1)]
					interpolated = interpolated.Add(m.Mesh.Vertices[k])
				}
				u, v = u/3, v/3
				x := 2*u - 1
				scale := 1 / math.Sqrt(1+4*x*x)
				truePoint := geom.Vec3{X: x - .5*x*scale, Y: v, Z: x*x + .25*scale}
				if rotated {
					truePoint.X, truePoint.Y = -truePoint.Y, truePoint.X
				}
				if delta := truePoint.Sub(interpolated.Scale(1. / 3)).Length(); delta*delta > distance {
					t.Fatal("independent analytic offset escaped the certificate", u, v, delta)
				}
			}
			t.Logf("grid %d x %d, %d triangles, distance %.12g, monotonicity %s, geometry work %d",
				n, n, len(m.Mesh.Triangles), math.Sqrt(distance), m.ProjectedMonotonicityLower.FloatString(12), m.GeometryWork)
		})
	}
}

func TestVerifiedNormalOffsetGridBivariateRationalRoof(t *testing.T) {
	ctx := context.Background()
	// Original homogeneous degree-(2,2) controls for (u, v, f/w),
	// f=(u²+v²)/4 and w=1+(u+v)/8. The non-constant weights and both
	// independent height derivatives are used by the exact normal proof.
	var controls [][4]*big.Rat
	eighth := big.NewRat(1, 8)
	for i := int64(0); i < 3; i++ {
		for j := int64(0); j < 3; j++ {
			u, v := big.NewRat(i, 2), big.NewRat(j, 2)
			uu, vv := big.NewRat(i*(i-1), 2), big.NewRat(j*(j-1), 2)
			uv := new(big.Rat).Mul(u, v)
			x := new(big.Rat).Add(uu, uv)
			y := new(big.Rat).Add(vv, uv)
			x.Mul(x, eighth).Add(x, u)
			y.Mul(y, eighth).Add(y, v)
			z := new(big.Rat).Add(uu, vv)
			z.Quo(z, big.NewRat(4, 1))
			w := new(big.Rat).Add(u, v)
			w.Mul(w, eighth).Add(w, big.NewRat(1, 1))
			controls = append(controls, [4]*big.Rat{x, y, z, w})
		}
	}
	source, err := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, controls, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := gridTestVerifiedOffset(t, source, big.NewRat(-1, 10), geom.NormalOffsetProjection{Axes: [2]int{0, 1}})
	m, err := TessellateVerifiedNormalOffsetGrid(ctx, p, NormalOffsetGridOptions{})
	if err != nil {
		t.Fatal(err)
	}
	checkCertifiedOffsetGrid(t, p.Projection(), m)
	distance, _ := m.DistanceUpperSquared.Float64()
	if !(distance > 0 && distance <= .05*.05) {
		t.Fatal("wrong default physical tolerance", distance)
	}
	for k, point := range m.Mesh.Vertices {
		cols := len(m.VStations)
		u, v := m.UStations[k/cols], m.VStations[k%cols]
		f, w := (u*u+v*v)/4, 1+(u+v)/8
		du, dv := (u*w/2-f/8)/(w*w), (v*w/2-f/8)/(w*w)
		normal := geom.Vec3{X: -du, Y: -dv, Z: 1}
		exact := geom.Vec3{X: u, Y: v, Z: f / w}.Add(normal.Scale(-.1 / normal.Length()))
		if point.Sub(exact).Length() > 4e-16 {
			t.Fatal("sample does not match the independent signed rational offset", u, v, point, exact)
		}
	}
	t.Logf("default rational roof %d x %d, %d triangles, distance %.12g, lower projected monotonicity %s",
		len(m.UStations)-1, len(m.VStations)-1, len(m.Mesh.Triangles), math.Sqrt(distance), m.ProjectedMonotonicityLower.FloatString(12))
}

func TestVerifiedNormalOffsetGridRoundingAndNativeDomain(t *testing.T) {
	ctx := context.Background()
	distance := big.NewRat(1, 3)
	source, err := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{X: 2, Y: -1}, {X: 2, Y: 3}, {X: 4, Y: -1}, {X: 4, Y: 3},
	}, []float64{2, 2, 4, 4}, []float64{-1, -1, 3, 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := gridTestVerifiedOffset(t, patch, distance, geom.NormalOffsetProjection{Axes: [2]int{0, 1}})
	o := NormalOffsetGridOptions{Deflection: 5, MaxTriangles: 2}
	m, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil {
		t.Fatal(err)
	}
	checkCertifiedOffsetGrid(t, p.Projection(), m)
	if !reflect.DeepEqual(m.UStations, []float64{2, 4}) || !reflect.DeepEqual(m.VStations, []float64{-1, 3}) {
		t.Fatal("changed original parameter domain")
	}
	z, _ := distance.Float64()
	gap := new(big.Rat).Sub(distance, new(big.Rat).SetFloat64(z))
	want := new(big.Rat).Add(big.NewRat(20, 1), gap.Mul(gap, gap))
	if gap.Sign() == 0 || m.DistanceUpperSquared.Cmp(want) != 0 ||
		m.ProjectedMonotonicityLower.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("bound lost represented rounding or original parameter units", m.DistanceUpperSquared, m.ProjectedMonotonicityLower)
	}
	for _, v := range m.Mesh.Vertices {
		if v.Z != z {
			t.Fatal("wrong rounded offset plane", v)
		}
	}
	o.MaxWork = m.GeometryWork
	replay, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil || !reflect.DeepEqual(replay, m) {
		t.Fatal("same-bound budget changed display", err)
	}
	o.MaxWork--
	refusal, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(refusal, CertifiedNormalOffsetGrid{}) {
		t.Fatal("one-short geometry budget published a partial display", err)
	}
}

func TestVerifiedNormalOffsetGridAtomicRefusals(t *testing.T) {
	ctx := context.Background()
	p := gridTestParabolicOffset(t, false)
	for _, options := range []NormalOffsetGridOptions{
		{Deflection: .01, MaxTriangles: 2}, {MaxWork: 1}, {Deflection: 10, MaxSourceOperations: 1},
	} {
		m, err := TessellateVerifiedNormalOffsetGrid(ctx, p, options)
		if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(m, CertifiedNormalOffsetGrid{}) {
			t.Fatal("unproved grid published with a short budget", options, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if m, err := TessellateVerifiedNormalOffsetGrid(canceled, p, NormalOffsetGridOptions{}); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(m, CertifiedNormalOffsetGrid{}) {
		t.Fatal("canceled proof published a partial display", err)
	}
	// The true XY projection has identity Jacobian. At this translation
	// its represented X coordinate is identical for every grid vertex.
	var controls [][4]*big.Rat
	for x := range 2 {
		for y := range 2 {
			controls = append(controls, [4]*big.Rat{big.NewRat(1<<60+int64(x), 1), big.NewRat(int64(y), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	patch, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	flat := gridTestVerifiedOffset(t, patch, new(big.Rat), geom.NormalOffsetProjection{Axes: [2]int{0, 1}})
	if m, err := TessellateVerifiedNormalOffsetGrid(ctx, flat, NormalOffsetGridOptions{Deflection: 4, MaxDepth: 2}); !errors.Is(err, geom.ErrConvergence) ||
		!reflect.DeepEqual(m, CertifiedNormalOffsetGrid{}) {
		t.Fatal("rounded projection collapse published an embedded display", err)
	}
}

func TestNormalOffsetGridCellUsesPositiveSymmetricJacobian(t *testing.T) {
	makeCorners := func(xy [4][2]int64) (out [4]normalOffsetGridPoint) {
		for i, p := range xy {
			out[i].exact = [3]*big.Rat{big.NewRat(p[0], 1), big.NewRat(p[1], 1), new(big.Rat)}
		}
		return out
	}
	projection := geom.NormalOffsetProjection{Axes: [2]int{0, 1}}
	one := big.NewRat(1, 1)
	alternate, margin := normalOffsetGridCellJacobian(makeCorners([4][2]int64{{0, 0}, {1, 0}, {1, 3}, {0, 1}}), projection, one, one)
	if !alternate || margin == nil || margin.Sign() <= 0 {
		t.Fatal("failed to use the diagonal with two strictly positive proofs", alternate, margin)
	}
	// This shear has positive ordinary determinant and an indefinite
	// symmetric part. Orientation alone cannot prove monotone embedding.
	_, margin = normalOffsetGridCellJacobian(makeCorners([4][2]int64{{0, 0}, {1, 0}, {4, 1}, {3, 1}}), projection, one, one)
	if margin != nil {
		t.Fatal("accepted an indefinite symmetric Jacobian", margin)
	}
}

func gridTestParabolicOffset(t *testing.T, rotated bool) *geom.VerifiedNormalOffsetPatch {
	t.Helper()
	var controls [][4]*big.Rat
	for i, x := range []int64{-1, 0, 1} {
		for _, y := range []int64{0, 1} {
			a, b := x, y
			if rotated {
				a, b = -y, x
			}
			controls = append(controls, [4]*big.Rat{big.NewRat(a, 1), big.NewRat(b, 1), big.NewRat([]int64{1, -1, 1}[i], 1), big.NewRat(1, 1)})
		}
	}
	patch, err := geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 1, controls, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	projection := geom.NormalOffsetProjection{Axes: [2]int{0, 1}}
	if rotated {
		projection = geom.NormalOffsetProjection{Axes: [2]int{1, 0}, Reversed: [2]bool{false, true}}
	}
	return gridTestVerifiedOffset(t, patch, big.NewRat(1, 4), projection)
}

func gridTestVerifiedOffset(t *testing.T, source *geom.SurfacePatch, d *big.Rat, projection geom.NormalOffsetProjection) *geom.VerifiedNormalOffsetPatch {
	t.Helper()
	p, proof, err := geom.NewVerifiedNormalOffsetPatch(context.Background(), source, d, projection, geom.NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("source offset did not prove embedding", err)
	}
	return p
}

func checkCertifiedOffsetGrid(t *testing.T, proj geom.NormalOffsetProjection, m CertifiedNormalOffsetGrid) {
	t.Helper()
	if err := m.Mesh.Validate(); err != nil {
		t.Fatal(err)
	}
	cols := len(m.VStations)
	edges := make(map[[2]int]int)
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	diff := func(a, b float64) *big.Rat { return new(big.Rat).Sub(rat(a), rat(b)) }
	cross := func(a, b, c, d *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Mul(a, d), new(big.Rat).Mul(b, c))
	}
	for _, tri := range m.Mesh.Triangles {
		for j, start := range tri {
			stop := tri[(j+1)%3]
			edges[[2]int{min(start, stop), max(start, stop)}]++
		}
		a, b, c := tri[0], tri[1], tri[2]
		ux, uy := diff(m.UStations[b/cols], m.UStations[a/cols]), diff(m.UStations[c/cols], m.UStations[a/cols])
		vx, vy := diff(m.VStations[b%cols], m.VStations[a%cols]), diff(m.VStations[c%cols], m.VStations[a%cols])
		det := cross(ux, uy, vx, vy)
		if det.Sign() <= 0 {
			t.Fatal("triangle is reversed in the original UV frame", tri)
		}
		coords := func(id int) [3]float64 {
			p := m.Mesh.Vertices[id]
			return [3]float64{p.X, p.Y, p.Z}
		}
		var j [2][2]*big.Rat
		for k, axis := range proj.Axes {
			q, r := diff(coords(b)[axis], coords(a)[axis]), diff(coords(c)[axis], coords(a)[axis])
			if proj.Reversed[k] {
				q.Neg(q)
				r.Neg(r)
			}
			j[k][0] = new(big.Rat).Quo(cross(q, r, vx, vy), det)
			j[k][1] = new(big.Rat).Quo(cross(ux, uy, q, r), det)
		}
		// The independently solved derivative satisfies S − certificate*I >= 0.
		x := new(big.Rat).Sub(j[0][0], m.ProjectedMonotonicityLower)
		y := new(big.Rat).Sub(j[1][1], m.ProjectedMonotonicityLower)
		off := new(big.Rat).Add(j[0][1], j[1][0])
		d := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(x, y))
		if m.ProjectedMonotonicityLower.Sign() <= 0 || x.Sign() < 0 || y.Sign() < 0 || d.Cmp(off.Mul(off, off)) < 0 {
			t.Fatal("mesh contradicts its claimed exact symmetric Jacobian bound", tri)
		}
	}
	boundary := 0
	for e, count := range edges {
		if count == 1 {
			boundary++
		} else if count != 2 {
			t.Fatal("interior edge failed to conform", e, count)
		}
	}
	if boundary != 2*(len(m.UStations)+len(m.VStations)-2) || len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 1 {
		t.Fatal("invalid open grid topology", boundary)
	}
}
