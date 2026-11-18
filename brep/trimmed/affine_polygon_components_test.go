//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
	"testing"
)

func TestConvexAffinePolygonComponents(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, true)
	r, e := CombineConvexAffinePolygonShellComponents(ctx, b, a, parametric.Difference, ConvexAffinePolygonShellCollectionOptions{Crossing: ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}, Material: ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}})
	if e != nil {
		t.Fatal(e)
	}
	components, volumes := r.Components(), r.ComponentVolumes()
	if len(components) != 2 || len(r.Separations()) != 1 {
		t.Fatal("incomplete component proof")
	}
	// Independent raw-coordinate lower-Y lobe: (3/4)*(1/4+1/24)=7/32.
	lower := new(big.Rat).Mul(big.NewRat(3, 4), new(big.Rat).Add(big.NewRat(1, 4), big.NewRat(1, 24)))
	total := new(big.Rat).Sub(big.NewRat(6, 1), affineMaterialFixtureCommonVolume())
	if volumes[0].Cmp(lower) != 0 || volumes[1].Cmp(new(big.Rat).Sub(total, lower)) != 0 || r.ExactVolume().Cmp(total) != 0 {
		t.Fatal("component mass")
	}
	proof := r.Separations()[0]
	if proof.Components != [2]int{0, 1} || proof.PlaneOwner != 1 || proof.PlaneFace != 2 || proof.PointSide != 1 || proof.SectionVertex < 0 || proof.Point[0].Sign() != 0 || proof.Point[1].Cmp(big.NewRat(-1, 1)) != 0 || proof.Point[2].Sign() != 0 {
		t.Fatal("wrong point ownership")
	}
	source := r.OriginalCrossing()
	point := source.SectionVertices()[proof.SectionVertex].Point
	for k := range point {
		if point[k].Cmp(proof.Point[k]) != 0 {
			t.Fatal("unbound paired contact")
		}
	}
	faceCount := 0
	for ci, solid := range components {
		if len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
			t.Fatal("component Euler")
		}
		if e = affineMaterialVertexLinks(ctx, solid); e != nil {
			t.Fatal(e)
		}
		zero, shared := 0, 0
		budget := &nativePrismAffineBudget{ctx: ctx, max: 100000, bits: 16384}
		plane := affineMaterialPlane{n: proof.Normal, d: proof.Offset}
		for _, v := range solid.vertices {
			q := affineMaterialXYZ(v)
			sign := affineMaterialValue(budget, plane, q).Sign()
			if sign*proof.Sides[ci] < 0 {
				t.Fatal("wrong closed halfspace")
			}
			if sign == 0 {
				zero++
			}
			same := true
			for k := range q {
				same = same && q[k].Cmp(point[k]) == 0
			}
			if same {
				shared++
			}
		}
		if shared != 1 || (ci == proof.PointSide && zero != 1) || (ci != proof.PointSide && zero < 3) {
			t.Fatal("unproved isolated zero set")
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || len(restored.faces) != len(solid.faces) {
			t.Fatal("reload", e)
		}
		sources := r.FaceSources(ci)
		if len(sources) != len(solid.faces) {
			t.Fatal("missing original face")
		}
		faceCount += len(sources)
		for fi, f := range sources {
			original := source.source.sources[f.Owner].faces[f.Face]
			if solid.faces[fi].Surface != original.Surface || solid.faces[fi].Reversed != (original.Reversed != f.Reversed) || r.SourceFaceFragments(f.Owner, f.Face) == nil {
				t.Fatal("source support lost")
			}
		}
	}
	retained := 0
	for _, f := range r.Fragments() {
		if f.Retained {
			retained++
		}
	}
	if faceCount != retained {
		t.Fatal("lost selected fragments")
	}
	proof.Point[0].SetInt64(9)
	volumes[0].SetInt64(9)
	if r.Separations()[0].Point[0].Sign() != 0 || r.ComponentVolumes()[0].Cmp(lower) != 0 {
		t.Fatal("mutable collection proof")
	}
	if _, e = source.CombineComponents(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true, MaxOperations: r.ExactOperations()}}); e != nil {
		t.Fatal("exact quota", e)
	}
	if v, e := source.CombineComponents(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true, MaxOperations: r.ExactOperations() - 1}}); v != nil || e == nil {
		t.Fatal("partial quota publication")
	}
	if _, e = source.Combine(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}); e == nil {
		t.Fatal("strict contract widened")
	}
	if v, e := CombineConvexAffinePolygonShellComponents(ctx, a, b, parametric.Difference, ConvexAffinePolygonShellCollectionOptions{Crossing: ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}, Material: ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}}); v != nil || e == nil {
		t.Fatal("connected pinched component accepted")
	}
}
func TestConvexAffinePolygonComponentRejects(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, true)
	s, e := CertifyConvexAffinePolygonCrossing(ctx, b, a, ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []ConvexAffinePolygonMaterialCollectionOptions{{MaxComponents: 1, Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}, {MaxComponents: 33}, {Material: ConvexAffinePolygonMaterialOptions{MaxOperations: 1}}} {
		if r, e := s.CombineComponents(ctx, intersect.AffineRingDifference, o); r != nil || e == nil {
			t.Fatal("non-atomic bound")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.CombineComponents(canceled, intersect.AffineRingDifference, ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}); r != nil || e == nil {
		t.Fatal("cancellation")
	}
	budget := func() *nativePrismAffineBudget { return &nativePrismAffineBudget{ctx: ctx, max: 1000000, bits: 16384} }
	if _, ok := polygonComponentSeparator(budget(), s, a, a); ok {
		t.Fatal("area overlap accepted")
	}
	// Two exact separating vertex sets share a nonzero interval: no original
	// affine plane can prove a singleton zero set on either side.
	lo := Solid{vertices: []geom.Vec3{{X: -.5, Y: -1}, {X: .5, Y: -1}, {Y: -2, Z: -1}, {Y: -2, Z: 1}}}
	hi := Solid{vertices: []geom.Vec3{{X: -.5, Y: -1}, {X: .5, Y: -1}, {Z: -1}, {Z: 1}}}
	if _, ok := polygonComponentSeparator(budget(), s, lo, hi); ok {
		t.Fatal("interval contact accepted")
	}
	r, e := s.CombineComponents(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}})
	if e != nil {
		t.Fatal(e)
	}
	damaged := *s
	damaged.nodes = nil
	if out, e := damaged.CombineComponents(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}); out != nil || e == nil {
		t.Fatal("incomplete original owner publication")
	}
	parts := r.Components()
	if _, ok := polygonComponentSeparator(budget(), &damaged, parts[0], parts[1]); ok {
		t.Fatal("unowned contact accepted")
	}
}

func TestConvexAffinePolygonForwardPinch(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, true)
	r, e := CombineConvexAffinePolygonShellComponents(ctx, a, b, parametric.Difference, ConvexAffinePolygonShellCollectionOptions{Crossing: ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}, Material: ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}})
	var pinch *ConvexAffinePolygonPinchError
	if r != nil || !errors.As(e, &pinch) || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("missing exact pinch evidence", e)
	}
	if pinch.Component != 0 || pinch.ClosedFans != 2 || pinch.SectionVertex < 0 || pinch.Point[0].Sign() != 0 || pinch.Point[1].Cmp(big.NewRat(-1, 1)) != 0 || pinch.Point[2].Sign() != 0 {
		t.Fatal("wrong pinch", pinch)
	}
}

func TestConvexAffinePolygonComponentsOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_AFFINE_POLYGON_PAIRED")
	if exe == "" {
		t.Skip("optional independent OCCT reference")
	}
	data, e := exec.Command(exe, "full").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Volume                   float64
		ComponentVolumes         []float64
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 6 {
		t.Fatal("incomplete oracle")
	}
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, true)
	r, e := CombineConvexAffinePolygonShellComponents(ctx, b, a, parametric.Difference, ConvexAffinePolygonShellCollectionOptions{Crossing: ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}, Material: ConvexAffinePolygonMaterialCollectionOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}}})
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, row := range rows {
		if row.Operation != 2 {
			continue
		}
		found++
		if !row.Valid {
			t.Fatal("invalid reference")
		}
		if row.Order == 0 {
			if row.Solids != 1 || math.Abs(row.Volume-1747.0/64) > 1e-10 {
				t.Fatal("forward OCCT", row)
			}
			// OCCT's one solid is deliberately not evidence for a circular native link.
			continue
		}
		if row.Solids != 2 || len(row.ComponentVolumes) != 2 || math.Abs(row.Volume-91.0/64) > 1e-10 {
			t.Fatal("reverse OCCT", row)
		}
		sort.Float64s(row.ComponentVolumes)
		for i, v := range r.ComponentVolumes() {
			want, _ := v.Float64()
			if math.Abs(row.ComponentVolumes[i]-want) > 1e-10 {
				t.Fatal("component mass", row.ComponentVolumes, r.ComponentVolumes())
			}
		}
	}
	if found != 2 {
		t.Fatal("missing Cut rows")
	}
}
