package trimmed

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

func rationalStationBand(t *testing.T, z0, z1, r0, r1, weight float64) *geom.WrappedSurfacePatch {
	t.Helper()
	polygon := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []geom.Vec3
	var weights []float64
	for i, q := range polygon {
		w := 1.
		if i%2 == 1 {
			w = weight
		}
		for j, r := range []float64{r0, r1} {
			z := z0
			if j == 1 {
				z = z1
			}
			poles = append(poles, geom.Vec3{X: .25 + r*q[0], Y: -.5 + r*q[1], Z: z})
			weights = append(weights, w)
		}
	}
	b, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{0, 0, 1, 1}, weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), patch, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

// Independent Green integral of the explicitly authored weighted quadratic,
// without evaluating a kernel curve or surface. Composite Simpson convergence
// is checked separately; this is a numerical test oracle, not an exact measure API.
func rationalQuarterProfileArea(weight float64, n int) float64 {
	sum := 0.
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		s := 1 - t
		b := 2 * weight * t * s
		db := 2 * weight * (1 - 2*t)
		x, y := s*s+b, b+t*t
		dx, dy := -2*s+db, db+2*t
		den := s*s + b + t*t
		f := 2 * (x*dy - y*dx) / (den * den)
		c := 2.
		if i == 0 || i == n {
			c = 1
		} else if i%2 == 1 {
			c = 4
		}
		sum += c * f
	}
	return sum / (3 * float64(n))
}

func TestRationalStationLoftNativeAndCertifiedDisplay(t *testing.T) {
	ctx := context.Background()
	a := rationalStationBand(t, 0, 1, 3, 1, .75)
	b := rationalStationBand(t, 1, 3, 1, 2, .75)
	for _, w := range []*geom.WrappedSurfacePatch{a, b} {
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, geom.SurfacePatchOptions{})
		if e != nil || !p.Complete || p.Circular {
			t.Fatal("genuine noncircular source", p, e)
		}
	}
	body, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	faces := body.Faces()
	slices.Reverse(faces)
	body, e = NewSolidContext(ctx, body.Vertices(), body.Edges(), faces, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var recipe bytes.Buffer
	if e = WriteJSON(&recipe, body); e != nil {
		t.Fatal(e)
	}
	body, e = ReadJSONContext(ctx, &recipe, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := body.ValidateWrappedRationalStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete || len(proof.Scales) != 3 {
		t.Fatal("native source proof", e)
	}
	if _, e = body.ValidateWrappedCircularStationLoft(ctx, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("circular query gate widened")
	}
	opts := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2, MaxTriangleChecks: 30000}, MaxTriangles: 20000}}
	validation := placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxTriangles: 20000, MaxCollisionPairs: 1000000}}
	p, e := body.PrepareWrappedRationalStationDisplay(ctx, opts, validation)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = body.PrepareWrappedStationDisplay(ctx, opts, validation); e == nil {
		t.Fatal("legacy circular preparation widened")
	}
	m := p.display.prepared.display.Mesh()
	owners := p.display.prepared.display.Faces()
	h := p.Stations()
	ancestry := p.Ancestry()
	edgeOwners := map[[2]int]map[int]bool{}
	volume := 0.
	for i, tr := range m.Triangles {
		x, y, z := m.Vertices[tr[0]], m.Vertices[tr[1]], m.Vertices[tr[2]]
		volume += (x.X*(y.Y*z.Z-y.Z*z.Y) + x.Y*(y.Z*z.X-y.X*z.Z) + x.Z*(y.X*z.Y-y.Y*z.X)) / 6
		for j := 0; j < 3; j++ {
			a, b := tr[j], tr[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if edgeOwners[key] == nil {
				edgeOwners[key] = map[int]bool{}
			}
			edgeOwners[key][owners[i]] = true
		}
	}
	if len(m.Vertices)-len(edgeOwners)+len(m.Triangles) != 2 {
		t.Fatal("native closed topology")
	}
	for _, edge := range h.StationRims[1] {
		row := ancestry.EdgeVertices[edge]
		for j := 1; j < len(row); j++ {
			x, y := row[j-1], row[j]
			if x > y {
				x, y = y, x
			}
			want := map[int]bool{h.WallFaces[0]: true, h.WallFaces[1]: true}
			if !reflect.DeepEqual(edgeOwners[[2]int{x, y}], want) {
				t.Fatal("shared exact noncircular rim")
			}
		}
	}
	area := rationalQuarterProfileArea(.75, 4096)
	if math.Abs(area-rationalQuarterProfileArea(.75, 2048)) > 1e-11 || math.Abs(area-math.Pi) < .01 {
		t.Fatal("independent noncircular oracle", area)
	}
	truth := 9 * area
	if math.Abs(volume-truth) > truth*.04 {
		t.Fatal("independent meridian integral", volume, truth)
	}
	// Same corners, height, scale and knots; the whole interior rim changes.
	wrong := rationalStationBand(t, 1, 3, 1, 2, .5)
	if denied, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{a, wrong}, 1e-8, geom.SurfacePatchOptions{}); e == nil || len(denied.faces) != 0 {
		t.Fatal("same-endpoint unequal profile admitted")
	}
	bad := body
	bad.faces = body.Faces()
	bad.faces[h.WallFaces[1]].Surface = wrong
	if denied, e := bad.PrepareWrappedRationalStationDisplay(ctx, opts, validation); e == nil || denied != nil {
		t.Fatal("stored changed rim interior accepted")
	}
	if denied, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || len(denied.faces) != 0 {
		t.Fatal("source proof quota")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if denied, _, e := NewWrappedRationalStationLoft(cancelled, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{}); e == nil || len(denied.faces) != 0 {
		t.Fatal("atomic cancellation")
	}
	gap := rationalStationBand(t, 1.25, 3, 1, 2, .75)
	if denied, _, e := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{a, gap}, 1e-8, geom.SurfacePatchOptions{}); e == nil || len(denied.faces) != 0 {
		t.Fatal("axial gap admitted")
	}
	if denied, _, e := NewWrappedCircularStationLoft(ctx, []*geom.WrappedSurfacePatch{a, b}, 1e-8, geom.SurfacePatchOptions{}); e == nil || len(denied.faces) != 0 {
		t.Fatal("old constructor widened")
	}
	opts.MaxStationSamples = 1
	if denied, e := body.PrepareWrappedRationalStationDisplay(ctx, opts, validation); e == nil || denied != nil {
		t.Fatal("cumulative budget")
	}
	t.Log("triangles", len(m.Triangles), "profile area", area, "volume", volume, "independent", truth)
}
