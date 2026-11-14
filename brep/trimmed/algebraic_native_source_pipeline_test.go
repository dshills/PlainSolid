package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

// This whole-pipeline fixture uses explicit exact quadratic source p-curves
// representing straight UV edges on nonlinear warped support faces. Curved UV
// image restriction is qualified independently in TestNativeAlgebraicCurvedSourceUse.
func TestNativeAlgebraicExactSourceCurvePipeline(t *testing.T) {
	ctx := context.Background()
	var e error
	a := makeNativeCurvedTubeBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1})
	b := makeNativeCurvedTubeBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5})
	convert := func(s Solid) Solid {
		faces := s.Faces()
		for i := range faces {
			for j := range faces[i].Outer {
				u := &faces[i].Outer[j]
				var h [3][3]*big.Rat
				for k := range h {
					q := big.NewRat(int64(k), 2)
					h[k] = [3]*big.Rat{nativeAlgebraicLerp(u.UVStart.X, u.UVEnd.X, q), nativeAlgebraicLerp(u.UVStart.Y, u.UVEnd.Y, q), big.NewRat(1, 1)}
				}
				curve, e := NewExactQuadraticUV(h)
				if e != nil {
					t.Fatal(e)
				}
				u.Curve = curve
			}
		}
		rebuilt, e := NewSolidContext(ctx, s.Vertices(), s.Edges(), faces, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		proof, e := rebuilt.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(proof, e)
		}
		return rebuilt
	}
	a, b = convert(a), convert(b)
	// Same endpoints do not prove continuous identity or rectangular coverage.
	bad := a.faces[0].Outer[0]
	q := bad.Curve.(ExactQuadraticUV)
	h := q.HomogeneousControls()
	h[1][0].Add(h[1][0], big.NewRat(1, 1024))
	bad.Curve, e = NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	if exactAffineNativeSourceUse(bad) {
		t.Fatal("false affine trim")
	}
	same, e := compareNativeSourceQuadraticUse(ctx, a.edges[bad.Edge], a.faces[0].Surface, bad, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if same {
		t.Fatal("endpoint-only agreement")
	}

	o := AlgebraicShellSectionOptions{}
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Seeds = curvedNativeBoxSeeds(t, a, b)
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete {
		t.Fatal("section", section.Pairs, e)
	}
	material, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal("material", e)
	}
	native, e := material.NativeRepresentable(ctx, 1e-8, NativeAlgebraicBooleanOptions{})
	if e != nil {
		t.Fatal("native", e)
	}
	curved := 0
	for _, edge := range native.edges {
		m, ok := edge.Curve.(surfaceTrimCurve)
		if !ok {
			continue
		}
		tube, ok := m.curve.(intersect.TubeTrim)
		if !ok {
			continue
		}
		path, e := tube.CertifyAffinePath(ctx, geom.TubeAffineLineOptions{})
		if e != nil || !path.Proven {
			t.Fatal("native exact curved path", e)
		}
		line, e := tube.CertifyAffineChord(ctx, geom.TubeAffineLineOptions{})
		if e != nil || !line.Proven {
			curved++
		}
	}
	if curved == 0 {
		t.Fatal("fixture lacks curved model tubes")
	}
	count := 0
	for faceID, f := range native.faces {
		source, e := material.OriginalFace(faceID)
		if e != nil || source.Surface != f.Surface {
			t.Fatal("original support", e)
		}
		for _, u := range f.Outer {
			if _, ok := u.Curve.(ExactQuadraticUV); ok {
				count++
			}
		}
	}
	if count == 0 {
		t.Fatal("source p-curve reconstruction path unused")
	}
	if native.Topology().EulerCharacteristic != 2 {
		t.Fatal(native.Topology())
	}
	mass, e := native.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-5})
	if e != nil || mass.Volume < .1249999 || mass.Volume > .1250001 {
		t.Fatal(mass.Volume, e)
	}
	var out bytes.Buffer
	if e = WriteJSON(&out, native); e != nil {
		t.Fatal("write", e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	back, e := ReadJSONContext(ctx, bytes.NewReader(raw), JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	agreement, e := back.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete || back.Topology().EulerCharacteristic != 2 {
		t.Fatal("reloaded proof", agreement, e)
	}
	retained := 0
	for _, f := range back.faces {
		for _, u := range f.Outer {
			if _, ok := u.Curve.(ExactQuadraticUV); ok {
				retained++
			}
		}
	}
	if retained != count {
		t.Fatal("lost persisted source curves", retained, count)
	}
	// A one-ULP change can pass numerical endpoint tolerance but cannot pass
	// the exact mixed-boundary source/vertex corroboration on read or write.
	var encoded nativeSolid
	if e = json.Unmarshal(raw, &encoded); e != nil {
		t.Fatal(e)
	}
	tubeEdge := -1
	for i, x := range material.index {
		if x.SectionPiece >= 0 {
			tubeEdge = i
			break
		}
	}
	if tubeEdge < 0 {
		t.Fatal("no tube")
	}
	id := native.edges[tubeEdge].Start
	encoded.Vertices[id].X = math.Nextafter(encoded.Vertices[id].X, math.Inf(1))
	forged, _ := json.Marshal(encoded)
	if _, e = ReadJSONContext(ctx, bytes.NewReader(forged), JSONLimits{}); e == nil {
		t.Fatal("rounded tube vertex accepted")
	}
	badNative := native
	badNative.vertices = native.Vertices()
	badNative.vertices[id] = encoded.Vertices[id]
	var rejected bytes.Buffer
	if e = WriteJSON(&rejected, badNative); e == nil || rejected.Len() != 0 {
		t.Fatal("non-atomic forged write", e)
	}
	t.Logf("retained exact source uses=%d faces=%d edges=%d curvedTubes=%d volume=%.12g", count, len(native.faces), len(native.edges), curved, mass.Volume)
}
