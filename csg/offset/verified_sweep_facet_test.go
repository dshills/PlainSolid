package offset

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

func TestVerifiedSweepFacetAngleCurvedOriginalAndRotated(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		t.Run(fmt.Sprint(rotated), func(t *testing.T) {
			s := verifiedSweepBivariateRoof(t, rotated)
			o := VerifiedSweepMeshOptions{Deflection: .6}
			ordinary, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
			if err != nil || ordinary.SquaredFacetNormalTangent != nil {
				t.Fatal("distance-only sweep unexpectedly depended on the normal proof", err)
			}
			last := math.Inf(1)
			for _, request := range []float64{.2, .03} {
				o.NormalAngle = request
				m, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
				if err != nil {
					t.Fatal(err)
				}
				checkVerifiedSweepMesh(t, s, m, o.Deflection)
				if m.SquaredFacetNormalTangent == nil || m.SquaredFacetNormalTangent.Sign() <= 0 {
					t.Fatal("curved wall fixture has no exact angular certificate")
				}
				q, _ := m.SquaredFacetNormalTangent.Float64()
				bound := math.Atan(math.Sqrt(q))
				if bound > request || bound >= last {
					t.Fatal("tightening the request failed to tighten the facet certificate", bound, request, last)
				}
				last = bound
				seen := checkVerifiedSweepFacetAnalytic(t, m, rotated, bound)
				t.Logf("%d triangles, request %.3f rad, proved %.12f rad, sampled deviations by analytic side %v",
					len(m.Mesh.Triangles), request, bound, seen)
			}
			o.MaxTriangles = len(ordinary.Mesh.Triangles)
			bad, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
			if !(errors.Is(err, ErrLimit) || errors.Is(err, geom.ErrConvergence)) || !reflect.DeepEqual(bad, CertifiedVerifiedSweepMesh{}) {
				t.Fatal("angular limit published a partial sweep shell", err)
			}
		})
	}
}

func verifiedSweepBivariateRoof(t *testing.T, rotated bool) *geom.VerifiedNormalOffsetSweep {
	t.Helper()
	var net [][4]*big.Rat
	// h(u,v)=(u²+v²+uv)/16. Both original side-wall normals vary
	// along their free coordinates and can vary with displacement.
	for u := int64(0); u < 3; u++ {
		for v := int64(0); v < 3; v++ {
			x, y := big.NewRat(u, 2), big.NewRat(v, 2)
			if rotated {
				x, y = big.NewRat(-v, 2), big.NewRat(u, 2)
			}
			z := big.NewRat(2*u*(u-1)+2*v*(v-1)+u*v, 64)
			net = append(net, [4]*big.Rat{x, y, z, big.NewRat(1, 1)})
		}
	}
	p, err := geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 2, net, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	view := geom.NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	if rotated {
		view = geom.NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{false, true, false}}
	}
	return startVerifiedSweep(t, p, big.NewRat(-1, 10), big.NewRat(1, 10), view)
}

func checkVerifiedSweepFacetAnalytic(t *testing.T, m CertifiedVerifiedSweepMesh, rotated bool, bound float64) (observed [6]float64) {
	t.Helper()
	// Independently sample the full original cell for each delivered facet.
	for i, indices := range m.Mesh.Triangles {
		a, b, c := m.Mesh.Vertices[indices[0]], m.Mesh.Vertices[indices[1]], m.Mesh.Vertices[indices[2]]
		facet := b.Sub(a).Cross(c.Sub(a))
		pa, pb, pc := m.VertexStations[indices[0]], m.VertexStations[indices[1]], m.VertexStations[indices[2]]
		low, high := [3]int{}, [3]int{}
		for axis := range low {
			low[axis], high[axis] = min(pa[axis], pb[axis], pc[axis]), max(pa[axis], pb[axis], pc[axis])
		}
		side := m.OriginalFace[i]
		t0, _ := m.DisplacementStations[low[2]].Float64()
		t1, _ := m.DisplacementStations[high[2]].Float64()
		for _, u := range [2]float64{m.UStations[low[0]], m.UStations[high[0]]} {
			for _, v := range [2]float64{m.VStations[low[1]], m.VStations[high[1]]} {
				for _, d := range [3]float64{t0, (t0 + t1) / 2, t1} {
					n := analyticBivariateSweepNormal(side, u, v, d)
					if rotated {
						n.X, n.Y = -n.Y, n.X
					}
					dot := n.Dot(facet)
					dev := math.Atan2(n.Cross(facet).Length(), dot)
					if dot <= 0 || dev > bound+1e-14 {
						t.Fatal("independent continuous outward normal escaped certificate", i, side, u, v, d, dev, bound)
					}
					observed[side] = max(observed[side], dev)
				}
			}
		}
	}
	for side, angle := range observed {
		if angle <= 0 {
			t.Fatal("fixture failed to exercise varying analytic face normals", side, angle)
		}
	}
	return observed
}

func analyticBivariateSweepNormal(side geom.NormalOffsetSweepSide, u, v, displacement float64) geom.Vec3 {
	gu, gv := (2*u+v)/16, (2*v+u)/16
	n := geom.Vec3{X: -gu, Y: -gv, Z: 1}
	length := n.Length()
	unit := n.Scale(1 / length)
	switch side {
	case geom.NormalOffsetSweepLower:
		return unit.Scale(-1)
	case geom.NormalOffsetSweepUpper:
		return unit
	}
	free := geom.Vec3{X: 1, Z: gu}
	derivative := geom.Vec3{X: -1. / 8, Y: -1. / 16}
	if side <= geom.NormalOffsetSweepUMax {
		free = geom.Vec3{Y: 1, Z: gv}
		derivative = geom.Vec3{X: -1. / 16, Y: -1. / 8}
	}
	dUnit := derivative.Sub(unit.Scale(unit.Dot(derivative))).Scale(1 / length)
	normal := free.Add(dUnit.Scale(displacement)).Cross(unit)
	if side == geom.NormalOffsetSweepUMin || side == geom.NormalOffsetSweepVMax {
		normal = normal.Scale(-1)
	}
	return normal
}

func TestVerifiedSweepFacetAngleExactPlanarAndWorkLimit(t *testing.T) {
	ctx := context.Background()
	s := verifiedSweepSlab(t, new(big.Rat))
	o := VerifiedSweepMeshOptions{Deflection: 2, MaxTriangles: 12}
	ordinary, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
	if err != nil {
		t.Fatal(err)
	}
	o.NormalAngle = 1e-8
	m, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
	if err != nil || m.SquaredFacetNormalTangent == nil || m.SquaredFacetNormalTangent.Sign() != 0 ||
		!reflect.DeepEqual(m.Mesh, ordinary.Mesh) || m.GeometryWork <= ordinary.GeometryWork {
		t.Fatal("all six planar faces must prove zero angular deviation after rational rounding", err)
	}
	o.MaxWork = m.GeometryWork
	same, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
	if err != nil || !reflect.DeepEqual(same, m) {
		t.Fatal("the exact total angular budget changed the result", err)
	}
	o.MaxWork--
	short, err := TessellateVerifiedNormalOffsetSweep(ctx, s, o)
	if !errors.Is(err, ErrLimit) || !reflect.DeepEqual(short, CertifiedVerifiedSweepMesh{}) {
		t.Fatal("one-short face-normal work published a partial sweep shell", err)
	}
}
