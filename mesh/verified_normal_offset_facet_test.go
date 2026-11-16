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

func TestVerifiedNormalOffsetFacetAngle(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		t.Run(fmt.Sprint(rotated), func(t *testing.T) {
			p := gridTestParabolicOffset(t, rotated)
			o := NormalOffsetGridOptions{Deflection: .7}
			ordinary, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
			if err != nil || ordinary.SquaredFacetNormalTangent != nil {
				t.Fatal("distance-only grid unexpectedly depended on the angular proof", err)
			}
			lastCount := len(ordinary.Mesh.Triangles)
			lastBound := math.Inf(1)
			for _, request := range []float64{.2, .1} {
				o.NormalAngle = request
				got, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
				if err != nil {
					t.Fatal(err)
				}
				if got.SquaredFacetNormalTangent == nil || got.SquaredFacetNormalTangent.Sign() <= 0 ||
					got.SquaredFacetNormalTangent.Cmp(normalOffsetFacetTangentLimit(request)) > 0 {
					t.Fatal("missing rigorous exact facet certificate", got.SquaredFacetNormalTangent)
				}
				distance, _ := got.DistanceUpperSquared.Float64()
				if !(distance > 0 && distance <= o.Deflection*o.Deflection) {
					t.Fatal("facet option changed the physical distance contract", distance)
				}
				checkCertifiedOffsetGrid(t, p.Projection(), got)
				q, _ := got.SquaredFacetNormalTangent.Float64()
				bound := math.Atan(math.Sqrt(q))
				observed := checkOffsetFacetAnalyticSamples(t, got, rotated, bound)
				if len(got.Mesh.Triangles) <= lastCount || bound >= lastBound || bound > request {
					t.Fatal("tighter angular request failed to refine the grid and its bound", bound, len(got.Mesh.Triangles))
				}
				lastBound, lastCount = bound, len(got.Mesh.Triangles)
				t.Logf("%d triangles, requested %.3f rad, proved %.12f rad, analytic samples %.12f rad, geometry work %d",
					lastCount, request, bound, observed, got.GeometryWork)
			}
			o.MaxTriangles = len(ordinary.Mesh.Triangles)
			short, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
			if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(short, CertifiedNormalOffsetGrid{}) {
				t.Fatal("unproved angular request published a partial grid", err)
			}
		})
	}
}

func checkOffsetFacetAnalyticSamples(t *testing.T, got CertifiedNormalOffsetGrid, rotated bool, bound float64) float64 {
	t.Helper()
	cols := len(got.VStations)
	// Sample the original analytic normals throughout each full cell
	// for both facets, including UV positions outside the individual
	// triangle. The actual certificate covers the continuous cell.
	observed := 0.
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.Mesh.Vertices[tri[0]], got.Mesh.Vertices[tri[1]], got.Mesh.Vertices[tri[2]]
		facet := b.Sub(a).Cross(c.Sub(a))
		umin := min(tri[0]/cols, tri[1]/cols, tri[2]/cols)
		umax := max(tri[0]/cols, tri[1]/cols, tri[2]/cols)
		for j := range 5 {
			u := got.UStations[umin] + float64(j)/4*(got.UStations[umax]-got.UStations[umin])
			x := 2*u - 1
			normal := geom.Vec3{X: -2 * x, Z: 1}
			if rotated {
				normal.X, normal.Y = 0, normal.X
			}
			dot := normal.Dot(facet)
			deviation := math.Atan2(normal.Cross(facet).Length(), dot)
			if dot <= 0 || deviation > bound+1e-14 {
				t.Fatal("independent analytic normal escaped the represented facet cone", tri, u, deviation, bound)
			}
			observed = max(observed, deviation)
		}
	}
	return observed
}

func TestVerifiedNormalOffsetFacetConeRejectsBackFacingBox(t *testing.T) {
	facet := normalOffsetFacetVector{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	limit := big.NewRat(1, 16)
	front := newNormalOffsetFacetBox(geom.Box{Min: geom.Vec3{X: -.125, Z: 1}, Max: geom.Vec3{X: .125, Z: 1}})
	if got := front.certify(facet, limit); got == nil || got.Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal("did not bound the exact box corners", got)
	}
	back := newNormalOffsetFacetBox(geom.Box{Min: geom.Vec3{Z: -1}, Max: geom.Vec3{Z: -1}})
	if got := back.certify(facet, limit); got != nil {
		t.Fatal("a squared cosine test alone admitted the opposite normal", got)
	}
	boundary := newNormalOffsetFacetBox(geom.Box{Min: geom.Vec3{Z: 0}, Max: geom.Vec3{Z: 1}})
	if got := boundary.certify(facet, limit); got != nil {
		t.Fatal("a possible zero dot must not pass as an acute cone", got)
	}
}

func TestVerifiedNormalOffsetFacetExactPlanarAndWorkLimit(t *testing.T) {
	ctx := context.Background()
	var controls [][4]*big.Rat
	for x := range 2 {
		for y := range 2 {
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(x), 1), big.NewRat(int64(y), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	patch, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := gridTestVerifiedOffset(t, patch, big.NewRat(1, 3), geom.NormalOffsetProjection{Axes: [2]int{0, 1}})
	o := NormalOffsetGridOptions{Deflection: 2, MaxTriangles: 2}
	previous, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil {
		t.Fatal(err)
	}
	o.NormalAngle = 1e-8
	got, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil || got.SquaredFacetNormalTangent == nil || got.SquaredFacetNormalTangent.Sign() != 0 ||
		got.GeometryWork <= previous.GeometryWork || !reflect.DeepEqual(got.Mesh, previous.Mesh) {
		t.Fatal("exact parallel offset should prove zero facet deviation", err)
	}
	o.MaxWork = got.GeometryWork
	same, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil || !reflect.DeepEqual(same, got) {
		t.Fatal("the exact angular work budget changed the proof", err)
	}
	o.MaxWork--
	bad, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if !errors.Is(err, geom.ErrConvergence) || !reflect.DeepEqual(bad, CertifiedNormalOffsetGrid{}) {
		t.Fatal("one-short normal-box budget published a partial grid", err)
	}
}

func TestVerifiedNormalOffsetFacetBivariateRational(t *testing.T) {
	ctx := context.Background()
	p := gridTestBivariateRationalOffset(t)
	o := NormalOffsetGridOptions{Deflection: .3, NormalAngle: .1}
	m, err := TessellateVerifiedNormalOffsetGrid(ctx, p, o)
	if err != nil {
		t.Fatal(err)
	}
	if m.SquaredFacetNormalTangent == nil || m.SquaredFacetNormalTangent.Sign() <= 0 ||
		m.SquaredFacetNormalTangent.Cmp(normalOffsetFacetTangentLimit(o.NormalAngle)) > 0 {
		t.Fatal("missing qualified nonconstant bivariate facet bound", m.SquaredFacetNormalTangent)
	}
	checkCertifiedOffsetGrid(t, p.Projection(), m)
	q, _ := m.SquaredFacetNormalTangent.Float64()
	bound := math.Atan(math.Sqrt(q))
	cols := len(m.VStations)
	observed := 0.
	for _, tri := range m.Mesh.Triangles {
		a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
		facet := b.Sub(a).Cross(c.Sub(a))
		ulo, uhi := min(tri[0]/cols, tri[1]/cols, tri[2]/cols), max(tri[0]/cols, tri[1]/cols, tri[2]/cols)
		vlo, vhi := min(tri[0]%cols, tri[1]%cols, tri[2]%cols), max(tri[0]%cols, tri[1]%cols, tri[2]%cols)
		for _, u := range [2]float64{m.UStations[ulo], m.UStations[uhi]} {
			for _, v := range [2]float64{m.VStations[vlo], m.VStations[vhi]} {
				f, w := (u*u+v*v)/4, 1+(u+v)/8
				du, dv := (u*w/2-f/8)/(w*w), (v*w/2-f/8)/(w*w)
				normal := geom.Vec3{X: -du, Y: -dv, Z: 1}
				dot := normal.Dot(facet)
				deviation := math.Atan2(normal.Cross(facet).Length(), dot)
				if dot <= 0 || deviation > bound+1e-14 {
					t.Fatal("independent bivariate normal escaped the certified cone", tri, u, v, deviation, bound)
				}
				observed = max(observed, deviation)
			}
		}
	}
	t.Logf("negative offset on the weight-varying rational roof, %d triangles, proved %.12f rad, analytic samples %.12f rad",
		len(m.Mesh.Triangles), bound, observed)
}

func TestVerifiedNormalOffsetFacetChoosesAlternateDiagonal(t *testing.T) {
	var corners [4]normalOffsetGridPoint
	for i, xyz := range [4][3]int64{{0, 0, 0}, {1, 0, 1}, {1, 1, 0}, {0, 1, 2}} {
		corners[i].exact = [3]*big.Rat{big.NewRat(xyz[0], 1), big.NewRat(xyz[1], 1), big.NewRat(xyz[2], 1)}
	}
	one := big.NewRat(1, 1)
	projection := geom.NormalOffsetProjection{Axes: [2]int{0, 1}}
	plain, margin, _ := certifyNormalOffsetGridCell(corners, projection, one, one, nil, nil)
	if plain != 0 || margin == nil {
		t.Fatal("both original projected diagonals should qualify")
	}
	// One diagonal has maximum squared tangent 8. The other has 5.
	normals := newNormalOffsetFacetBox(geom.Box{Min: geom.Vec3{Z: 1}, Max: geom.Vec3{Z: 1}})
	diagonal, margin, bound := certifyNormalOffsetGridCell(corners, projection, one, one, normals, big.NewRat(6, 1))
	if diagonal != 1 || margin == nil || bound == nil || bound.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("failed to select the only angularly qualified diagonal", diagonal, margin, bound)
	}
}
