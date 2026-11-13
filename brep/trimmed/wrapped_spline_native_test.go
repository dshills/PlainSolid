package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestWrappedSplineNativeRoundTrip(t *testing.T) {
	ctx := context.Background()
	w := independentWrappedToroid(t)
	f, e := WrappedSplineFace(ctx, w, false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	encoded := append([]byte(nil), data.Bytes()...)
	r, e := ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil || r.Topology().EulerCharacteristic != 0 || r.Topology().Edges != 2 {
		t.Fatal(r.Topology(), e)
	}
	proof, e := r.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	base, _ := wrappedSurfaceModel(r.faces[0].Surface)
	if !geom.SameWrappedSurfaceRecipe(w, base) || base.SeamCertificate().FirstPartialsMatched != ([2]bool{true, true}) {
		t.Fatal("native wrapped geometry or seam order changed")
	}
	mass, e := r.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-3, MaxSubdivisions: 16})
	// Independently integrated from the rational quadratic profile in the
	// toroidal fixture, rather than the volume formula for a circular torus.
	if e != nil || math.Abs(mass.Volume-10.079822587922244) > .001 || mass.Centroid.Length() > 5e-5 {
		t.Fatal("restored rational toroid mass", mass, e)
	}
	var native nativeSolid
	if e := json.Unmarshal(encoded, &native); e != nil {
		t.Fatal(e)
	}
	var support struct {
		Kind   string
		Spline geom.BSplineSurfaceData
		Wrap   struct {
			U, V            bool
			ContinuityOrder int
		}
	}
	if e := json.Unmarshal(native.Faces[0].Surface, &support); e != nil {
		t.Fatal(e)
	}
	support.Spline.Poles[0].Z += .125
	native.Faces[0].Surface, e = json.Marshal(support)
	if e != nil {
		t.Fatal(e)
	}
	bad, e := json.Marshal(native)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSONContext(ctx, bytes.NewReader(bad), JSONLimits{}); e == nil {
		t.Fatal("native decode accepted a false wrapped seam")
	}
}

func TestSplitWrappedSplineSeamExact(t *testing.T) {
	ctx := context.Background()
	f, e := WrappedSplineFace(ctx, independentWrappedToroid(t), false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	x, split, e := s.SplitSplineEdgeExact(ctx, 0, .5, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if top := x.Topology(); top.Vertices != 2 || top.Edges != 3 || top.EulerCharacteristic != 0 || len(top.Shells) != 1 {
		t.Fatal(top)
	}
	counts := map[int]int{}
	for _, u := range x.faces[0].Outer {
		counts[u.Edge]++
	}
	if counts[split.First] != 2 || counts[split.Second] != 2 {
		t.Fatal("both uses of each split edge must be retained", counts, split)
	}
	proof, e := x.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
}
