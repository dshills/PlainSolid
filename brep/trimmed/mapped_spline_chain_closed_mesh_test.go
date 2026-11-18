package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/collision"
	"plainsolid/geom"
)

func mappedChainClosedFixture(t *testing.T) (MappedSplineChainAgreement, [2][]PCurve, []MappedChainPlanarFace, [][2]MappedChainClosedUse) {
	t.Helper()
	return mappedChainClosedFixtureWithKnot(t, false)
}

func mappedChainClosedFixtureWithKnot(t *testing.T, crease bool) (MappedSplineChainAgreement, [2][]PCurve, []MappedChainPlanarFace, [][2]MappedChainClosedUse) {
	t.Helper()
	ctx := context.Background()
	a, ca := mappedChainFixture(t, false)
	b, cb := mappedChainFixture(t, true)
	var knot, slope float64
	if crease {
		knot, slope = .5, .5
		a, b = mappedChainCreasedSupport(t, false, knot, slope), mappedChainCreasedSupport(t, true, knot, slope)
	}
	proof, err := CertifyMappedSplineChainAgreement(ctx, a, ca, b, cb, edgecheck.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err = proof.WriteJSON(ctx, &data, MappedChainNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadMappedSplineChainBoundaryJSON(ctx, &data, MappedChainNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for owner, chain := range proof.sources {
		if !sameAuthoredBezierSpline(loaded.sources[owner].parts[0].Source().Curve, chain.parts[0].Source().Curve) {
			t.Fatal("original source lost during reload", owner)
		}
	}
	closing, planar, seams := mappedChainFixtureClosures(t, knot, slope)
	return loaded, closing, planar, seams
}

func mappedChainFixtureClosures(t *testing.T, knot, slope float64, extra ...[2]float64) ([2][]PCurve, []MappedChainPlanarFace, [][2]MappedChainClosedUse) {
	t.Helper()
	line := func(a, b geom.Vec2) PCurve {
		return mappedChainFixtureLine(t, a, b)
	}
	closing := [2][]PCurve{
		mappedChainClosing(t),
		{
			line(geom.Vec2{X: .25, Y: 1}, geom.Vec2{Y: 1}),
			line(geom.Vec2{Y: 1}, geom.Vec2{}),
			line(geom.Vec2{}, geom.Vec2{X: .25}),
		},
	}
	face := func(origin, u, v geom.Vec3, reversed bool, ring ...geom.Vec2) MappedChainPlanarFace {
		return mappedChainFixturePlanarFace(t, origin, u, v, reversed, ring...)
	}
	xAxis, yAxis, height := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 2}
	planar := []MappedChainPlanarFace{
		// East and west keep their roof seam junction as an original coedge endpoint.
		face(geom.Vec3{X: 1, Z: -1}, yAxis, height, false,
			geom.Vec2{}, geom.Vec2{X: .75}, geom.Vec2{X: .75, Y: .875}, geom.Vec2{X: .25, Y: .625}, geom.Vec2{Y: .5}),
		face(geom.Vec3{Z: -1}, yAxis, height, true,
			geom.Vec2{}, geom.Vec2{X: .75}, geom.Vec2{X: .75, Y: .5}, geom.Vec2{X: .25, Y: .5}, geom.Vec2{Y: .5}),
		face(geom.Vec3{Y: .75, Z: -1}, xAxis, height, true,
			geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: .875}, geom.Vec2{Y: .5}),
		face(geom.Vec3{Z: -1}, xAxis, height, false,
			geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: .5}, geom.Vec2{Y: .5}),
		face(geom.Vec3{Z: -1}, xAxis, yAxis, true,
			geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: .75}, geom.Vec2{Y: .75}),
	}
	seams := [][2]MappedChainClosedUse{
		{{0, 0}, {2, 2}}, {{0, 1}, {4, 2}}, {{0, 2}, {3, 2}},
		{{1, 0}, {2, 3}}, {{1, 1}, {5, 2}}, {{1, 2}, {3, 3}},
		{{2, 1}, {4, 1}}, {{2, 4}, {5, 1}}, {{3, 1}, {4, 3}}, {{3, 4}, {5, 3}},
		{{2, 0}, {6, 1}}, {{3, 0}, {6, 3}}, {{4, 0}, {6, 2}}, {{5, 0}, {6, 0}},
	}
	if knot > 0 {
		wallUV := func(x, y float64) geom.Vec2 {
			return geom.Vec2{X: x, Y: (1 + mappedChainCreasedHeight(x, y, knot, slope, extra)) / 2}
		}
		xs := []float64{1}
		for i := len(extra) - 1; i >= 0; i-- {
			xs = append(xs, extra[i][0])
		}
		xs = append(xs, knot, 0)
		ends := [2]PCurve{closing[0][2], closing[1][2]}
		closing[0], closing[1] = closing[0][:1:1], closing[1][:1:1]
		north, south := []geom.Vec2{{}, {X: 1}}, []geom.Vec2{{}, {X: 1}}
		for i, x := range xs {
			north, south = append(north, wallUV(x, .75)), append(south, wallUV(x, 0))
			if i+1 < len(xs) {
				next := xs[i+1]
				closing[0] = append(closing[0], line(geom.Vec2{X: x, Y: .75}, geom.Vec2{X: next, Y: .75}))
				closing[1] = append(closing[1], line(geom.Vec2{Y: x}, geom.Vec2{Y: next}))
			}
		}
		for i := range closing {
			closing[i] = append(closing[i], ends[i])
		}
		planar[0] = face(geom.Vec3{X: 1, Z: -1}, yAxis, height, false,
			geom.Vec2{}, geom.Vec2{X: .75}, geom.Vec2{X: .75, Y: wallUV(1, .75).Y}, geom.Vec2{X: .25, Y: wallUV(1, .25).Y}, geom.Vec2{Y: wallUV(1, 0).Y})
		planar[2] = face(geom.Vec3{Y: .75, Z: -1}, xAxis, height, true, north...)
		planar[3] = face(geom.Vec3{Z: -1}, xAxis, height, false, south...)
		seams = nil
		for owner := range 2 {
			seams = append(seams, [2]MappedChainClosedUse{{owner, 0}, {2, 2 + owner}})
			for i := 1; i < len(xs); i++ {
				seams = append(seams, [2]MappedChainClosedUse{{owner, i}, {4 + owner, i + 1}})
			}
			seams = append(seams, [2]MappedChainClosedUse{{owner, len(xs)}, {3, 2 + owner}})
		}
		last := len(xs) + 1
		seams = append(seams, [][2]MappedChainClosedUse{
			{{2, 1}, {4, 1}}, {{2, 4}, {5, 1}}, {{3, 1}, {4, last}}, {{3, 4}, {5, last}},
			{{2, 0}, {6, 1}}, {{3, 0}, {6, 3}}, {{4, 0}, {6, 2}}, {{5, 0}, {6, 0}},
		}...)
	}

	return closing, planar, seams
}

func mappedChainFixtureLine(t *testing.T, a, b geom.Vec2) PCurve {
	t.Helper()
	q, err := NewExactQuadraticUV([3][3]*big.Rat{
		{ratInset(a.X), ratInset(a.Y), big.NewRat(1, 1)},
		{ratInset((a.X + b.X) / 2), ratInset((a.Y + b.Y) / 2), big.NewRat(1, 1)},
		{ratInset(b.X), ratInset(b.Y), big.NewRat(1, 1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func mappedChainFixturePlanarFace(t *testing.T, origin, u, v geom.Vec3, reversed bool, ring ...geom.Vec2) MappedChainPlanarFace {
	t.Helper()
	knots := []float64{0, 0, 1, 1}
	patch, err := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{origin, origin.Add(v), origin.Add(u), origin.Add(u).Add(v)}, knots, knots, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := MappedChainPlanarFace{Surface: patch, Reversed: reversed}
	for i, p := range ring {
		out.Outer = append(out.Outer, mappedChainFixtureLine(t, p, ring[(i+1)%len(ring)]))
	}
	return out
}

func TestMappedSplineChainClosedMesh(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	proof, closing, planar, seams := mappedChainClosedFixture(t)
	reversed := [2]bool{false, true}
	const tolerance = .04
	face := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: tolerance},
	}}
	opts := MappedChainClosedMeshOptions{
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{face, face},
	}
	out, err := proof.TessellateClosedCertified(ctx, closing, reversed, planar, seams, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("triangles=%d vertices=%d chain samples=%d initial ledger=%d volume=%s bound=%s",
		len(out.Mesh.Triangles), len(out.Mesh.Vertices), len(out.ChainSamples), len(out.Boundary.Vertices),
		out.DisplayVolume.FloatString(9), out.SquaredDistance.FloatString(9))
	if !out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 {
		t.Fatal("global contact proof is incomplete", out.DisplayCollision)
	}
	if out.SquaredDistance.Cmp(new(big.Rat).Mul(ratInset(tolerance), ratInset(tolerance))) > 0 {
		t.Fatal("source distance certificate exceeds tolerance")
	}
	// The entire ideal roof has height x*y over [0,1] x [0,3/4].
	// The planar bottom is -1, giving analytic volume 57/64.
	volumeError := new(big.Rat).Abs(new(big.Rat).Sub(out.DisplayVolume, big.NewRat(57, 64)))
	if volumeError.Cmp(new(big.Rat).Mul(big.NewRat(3, 4), ratInset(tolerance))) > 0 {
		t.Fatal("display volume drift", out.DisplayVolume.FloatString(9))
	}
	if len(out.OriginalFace) != len(out.Mesh.Triangles) || len(out.FaceVertices) != 7 || len(out.ClosingVertices) != len(seams) {
		t.Fatal("incomplete face or closing-seam ancestry")
	}
	type incidence struct {
		face     int
		reversed bool
	}
	edges := make(map[[2]int][]incidence)
	covered := make(map[int]bool)
	for i, triangle := range out.Mesh.Triangles {
		f := out.OriginalFace[i]
		covered[f] = true
		for j, v := range triangle {
			w := triangle[(j+1)%3]
			key := [2]int{min(v, w), max(v, w)}
			edges[key] = append(edges[key], incidence{f, v > w})
		}
	}
	if len(covered) != 7 || len(out.Mesh.Vertices)-len(edges)+len(out.Mesh.Triangles) != 2 {
		t.Fatal("original face inventory or shell Euler characteristic")
	}
	checkSeam := func(a, b, face0, face1 int) {
		t.Helper()
		germs := edges[[2]int{min(a, b), max(a, b)}]
		if len(germs) != 2 || germs[0].reversed == germs[1].reversed ||
			min(germs[0].face, germs[1].face) != min(face0, face1) ||
			max(germs[0].face, germs[1].face) != max(face0, face1) {
			t.Fatal("seam incidence or orientation", a, b, germs)
		}
	}
	ledger := make(map[string]geom.Vec3)
	for _, v := range out.Boundary.Vertices {
		ledger[v.ModelParameter.RatString()] = v.Point
	}
	for i, s := range out.ChainSamples {
		point := out.Mesh.Vertices[s.Vertex]
		if i > 0 {
			prev := out.ChainSamples[i-1]
			if prev.ModelParameter.Cmp(s.ModelParameter) >= 0 {
				t.Fatal("chain ledger is not strictly ordered")
			}
			checkSeam(prev.Vertex, s.Vertex, 0, 1)
		}
		key := s.ModelParameter.RatString()
		if want, ok := ledger[key]; ok {
			if point != want {
				t.Fatal("moved certified ledger vertex")
			}
			delete(ledger, key)
		}
		for owner, c := range proof.sources {
			uv := authoredMixedSplinePoint(t, c.parts[0].source, s.AuthoredParameters[owner])
			x, y := uv[0], uv[1]
			if owner == 1 {
				x, y = y, x
			}
			fx, _ := x.Float64()
			fy, _ := y.Float64()
			fz, _ := new(big.Rat).Mul(x, y).Float64()
			if point != (geom.Vec3{X: fx, Y: fy, Z: fz}) {
				t.Fatal("both original authored parameters must reproduce the common model point")
			}
		}
	}
	if len(ledger) != 0 || len(out.Boundary.KnotVertices) != 4 || len(out.ChainSamples) <= len(out.Boundary.Vertices) {
		t.Fatal("lost original boundary or extra fill refinement")
	}
	for i, ids := range out.ClosingVertices {
		for j := 1; j < len(ids); j++ {
			checkSeam(ids[j-1], ids[j], seams[i][0].Face, seams[i][1].Face)
		}
	}
	for _, p := range out.Planar {
		if p.FillSquaredDistance.Sign() != 0 {
			t.Fatal("original planar closures must be exact")
		}
	}

	t.Run("different owner partitions", func(t *testing.T) {
		uneven := opts
		uneven.Curved[1].Face.BoundaryModelBudget = .002
		refined, err := proof.TessellateClosedCertified(ctx, closing, reversed, planar, seams, uneven)
		if err != nil {
			t.Fatal(err)
		}
		if refined.RefinementRounds != 1 || len(refined.ChainSamples) <= len(out.ChainSamples) ||
			!refined.DisplayCollision.Complete || len(refined.DisplayCollision.Contacts) != 0 {
			t.Fatal("owner fills did not refine to one certified partition")
		}
		shared := make(map[int]int)
		for owner, f := range refined.Curved {
			for _, use := range f.UVRegion.Samples[:len(proof.cuts)-1] {
				for _, local := range use.Vertices {
					shared[refined.FaceVertices[owner][local]] |= 1 << owner
				}
			}
		}
		if len(shared) != len(refined.ChainSamples) {
			t.Fatal("the ledger did not cover both recertified roofs")
		}
		for _, sample := range refined.ChainSamples {
			if shared[sample.Vertex] != 3 {
				t.Fatal("both owners must use each refined mesh vertex")
			}
		}
		t.Logf("rounds=%d triangles=%d shared=%d", refined.RefinementRounds, len(refined.Mesh.Triangles), len(shared))
	})

	requireFailure := func(ctx context.Context, planar []MappedChainPlanarFace, seams [][2]MappedChainClosedUse, o MappedChainClosedMeshOptions, want error) {
		t.Helper()
		bad, err := proof.TessellateClosedCertified(ctx, closing, reversed, planar, seams, o)
		if !errors.Is(err, want) || len(bad.Mesh.Vertices) != 0 || len(bad.Mesh.Triangles) != 0 || len(bad.ChainSamples) != 0 {
			t.Fatal("expected atomic rejection", want, err)
		}
	}
	requireFailure(ctx, planar, seams[:len(seams)-1], opts, geom.ErrInvalid)
	wrong := append([]MappedChainPlanarFace(nil), planar...)
	wrong[0].Outer = append([]PCurve(nil), wrong[0].Outer...)
	coefficients := wrong[0].Outer[2].(ExactQuadraticUV).HomogeneousControls()
	coefficients[1][1].Add(coefficients[1][1], big.NewRat(1, 16))
	wrong[0].Outer[2], err = NewExactQuadraticUV(coefficients)
	if err != nil {
		t.Fatal(err)
	}
	requireFailure(ctx, wrong, seams, opts, geom.ErrConvergence)
	limited := opts
	limited.ClosingProof.MaxWork = 1
	requireFailure(ctx, planar, seams, limited, geom.ErrConvergence)
	limited = opts
	limited.MaxTriangles = len(out.Mesh.Triangles) - 1
	requireFailure(ctx, planar, seams, limited, geom.ErrConvergence)
	limited = opts
	limited.Collision = collision.Options{MaxPairs: 1}
	bad, err := proof.TessellateClosedCertified(ctx, closing, reversed, planar, seams, limited)
	if !errors.Is(err, collision.ErrLimit) || len(bad.Mesh.Vertices) != 0 || len(bad.ChainSamples) != 0 {
		t.Fatal("global collision quota failure must discard the shell", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	requireFailure(canceled, planar, seams, opts, context.Canceled)
}
