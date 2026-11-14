package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoaxialNoncircularStationLoftBooleanCrossing(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	parts := [2]*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 2, 3, 1, .75), rationalStationBand(t, 0, 2, 1, 3, .75)}
	var inputs [2]Solid
	for i, band := range parts {
		var err error
		inputs[i], _, err = NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{band}, 1e-8, o)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], inputs[1], op, o)
		if err != nil || out.Topology().EulerCharacteristic != 2 || len(h.Bands) != 2 {
			t.Fatal("the common profile must preserve an exact crossing rim", out.Topology(), h, err)
		}
		wantCaps := [2][2]bool{{i == 1, i == 0}, {i == 0, i == 1}}
		if wantCaps != h.CapSources {
			t.Fatal("incorrect original cap sources", h.CapSources, wantCaps)
		}
		for j, band := range h.Bands {
			owner := (i + j + 1) % 2
			if !band.BasisPreserved || band.SourceWalls[owner] != 0 || band.SourceWalls[1-owner] != -1 ||
				band.LowerZ.Cmp(big.NewRat(int64(j), 1)) != 0 || band.UpperZ.Cmp(big.NewRat(int64(j+1), 1)) != 0 {
				t.Fatal("result wall must retain its original source and exact signed band", i, j, band)
			}
		}
		proof, err := out.ValidateWrappedRationalStationLoft(ctx, o)
		if err != nil || proof.Scales[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("incorrect shared original-profile rim", proof, err)
		}
		mass, err := out.BoundWrappedRationalStationMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 100)})
		// Integrate the two linear radius ramps, times the independently
		// integrated area of the normalized .75-weight profile.
		want := 3.1748736333785317 * [2]float64{14. / 3, 38. / 3}[i]
		if err != nil {
			t.Fatal(err)
		}
		low, _ := mass.Volume.Lower.Float64()
		high, _ := mass.Volume.Upper.Float64()
		if low > want || high < want || math.Abs(high-low) > .01 {
			t.Fatal("independent weighted profile volume", low, high, want)
		}
	}
	other, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 2, 1, 3, .5)}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], other, parametric.Union, o); !errors.Is(err, geom.ErrUnrepresentable) || len(out.faces) != 0 || len(h.Bands) != 0 {
		t.Fatal("equal control-polygon radii alone must not admit different rational profiles", h, err)
	}
	if out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], inputs[1], parametric.Difference, o); !errors.Is(err, geom.ErrUnrepresentable) || len(out.faces) != 0 || len(h.Bands) != 0 {
		t.Fatal("crossing noncircular difference leaked partial output", h, err)
	}
}

func TestCoaxialNoncircularStationLoftThroughBore(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	stock, stockHistory, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 1, 3, 2, .75), rationalStationBand(t, 1, 3, 2, 3, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	tool, toolHistory, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{
		rationalStationBand(t, 0, 3, 1, 1, .75),
	}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, stock, tool, parametric.Difference, o)
	top := out.Topology()
	if err != nil || top.EulerCharacteristic != 0 || len(top.Shells) != 1 || top.InnerWires != 2 || len(h.Bands) != 2 || len(h.InnerBands) != 1 {
		t.Fatal("preserve original outer and inward tool wall with two annular caps", top, h, err)
	}
	if h.SourceCaps != [2][2]int{stockHistory.CapFaces, toolHistory.CapFaces} {
		t.Fatal("incorrect original cap IDs", h.SourceCaps)
	}
	for _, sources := range h.CapSources {
		if sources != [2]bool{true, true} {
			t.Fatal("annular cap must retain both original rims", h)
		}
	}
	inner := h.InnerBands[0].ResultWall
	if !out.faces[inner].Reversed || out.faces[inner].Surface != tool.faces[toolHistory.WallFaces[0]].Surface {
		t.Fatal("inner wall must retain and reverse the exact tool support", h)
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, out); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := restored.ValidateWrappedRationalStationBore(ctx, o)
	if err != nil || !proof.Complete || proof.InnerWalls[0] != inner || proof.Caps != h.ResultCaps {
		t.Fatal("reload must revalidate original noncircular bore topology", proof, err)
	}
	if circular, err := restored.ValidateWrappedCircularStationBore(ctx, o); err == nil || circular.Complete {
		t.Fatal("circular validator accepted noncircular bore", circular)
	}
	for i, z := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
		section, err := restored.SectionWrappedRationalStationBoreRational(ctx, z, o)
		cap := -1
		if i == 0 {
			cap = h.ResultCaps[0]
		}
		if err != nil || section.Cap != cap || section.Inner == nil || !section.Inner.Hole ||
			section.Outer.Section.Height.Cmp(z) != 0 || section.Inner.Section.Radius.Cmp(big.NewRat(1, 1)) != 0 ||
			len(section.Outer.Faces) != 1+i/2 || section.Inner.Faces[0] != inner {
			t.Fatal("annular section must retain both original weighted loops and cap or crease ownership", z, section, err)
		}
	}
	r := big.NewRat
	for _, tc := range []struct {
		point [3]*big.Rat
		want  parametric.Location
		faces []int
	}{
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}, parametric.Outside, nil},
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(1, 3)}, parametric.Outside, nil},
		{[3]*big.Rat{r(27, 28), r(3, 14), r(0, 1)}, parametric.Boundary, []int{h.ResultCaps[0], inner}},
		{[3]*big.Rat{r(27, 28), r(3, 14), r(1, 3)}, parametric.Boundary, []int{inner}},
		{[3]*big.Rat{r(7, 4), r(-1, 2), r(0, 1)}, parametric.Boundary, []int{h.ResultCaps[0]}},
		{[3]*big.Rat{r(7, 4), r(-1, 2), r(1, 1)}, parametric.Inside, nil},
		{[3]*big.Rat{r(9, 4), r(-1, 2), r(1, 1)}, parametric.Boundary, proof.OuterWalls},
	} {
		loc, err := restored.LocateWrappedRationalStationBoreRational(ctx, tc.point, WrappedRationalStationPointOptions{})
		slices.Sort(tc.faces)
		if err != nil || loc.Location != tc.want || !slices.Equal(loc.Faces, tc.faces) {
			t.Fatal("exact annular material location and original owners", tc, loc, err)
		}
	}
	lineOptions := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	horizontal, err := restored.ClipWrappedRationalStationBoreLineRational(ctx,
		[3]*big.Rat{r(1, 4), r(-1, 2), r(1, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-3, 1), r(3, 1), lineOptions)
	if err != nil || len(horizontal.Hits) != 4 || len(horizontal.Interior) != 2 {
		t.Fatal("horizontal bore line must cross two material spans", horizontal, err)
	}
	for i, hit := range horizontal.Hits {
		want := []int64{-2, -1, 1, 2}[i]
		faces := []int{inner}
		if i == 0 || i == 3 {
			faces = proof.OuterWalls
		}
		if hit.Parameter.Min.Cmp(r(want, 1)) != 0 || hit.Parameter.Max.Cmp(r(want, 1)) != 0 || !slices.Equal(hit.Faces, faces) {
			t.Fatal("exact contact parameter and original bore owners", hit, want, faces)
		}
	}
	for _, x := range []int64{1, 5, 7} {
		vertical, err := restored.ClipWrappedRationalStationBoreLineRational(ctx,
			[3]*big.Rat{r(x, 4), r(-1, 2), r(-1, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(0, 1), r(5, 1), lineOptions)
		if x == 1 && (err != nil || len(vertical.Hits)+len(vertical.Interior) != 0) {
			t.Fatal("axial line through bore opening must miss material and caps", vertical, err)
		}
		if x == 5 && (!errors.Is(err, geom.ErrConvergence) || len(vertical.Hits)+len(vertical.Interior) != 0) {
			t.Fatal("inner wall generator requires a coincidence query", vertical, err)
		}
		if x == 7 && (err != nil || len(vertical.Hits) != 2 || len(vertical.Interior) != 1 ||
			!slices.Equal(vertical.Hits[0].Faces, []int{h.ResultCaps[0]}) || !slices.Equal(vertical.Hits[1].Faces, []int{h.ResultCaps[1]}) ||
			vertical.Hits[0].Parameter.Min.Cmp(r(1, 1)) != 0 || vertical.Hits[1].Parameter.Min.Cmp(r(4, 1)) != 0) {
			t.Fatal("vertical annular material line must enter and exit through exposed caps", vertical, err)
		}
	}
	capOrigin := [3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}
	capDirection := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	annulus, err := restored.ClipWrappedRationalStationBoreCapLineRational(ctx, capOrigin, capDirection, r(-4, 1), r(4, 1), lineOptions)
	if err != nil || len(annulus.Boundary) != 2 || len(annulus.Hits) != 4 {
		t.Fatal("two boundary intervals separated by the cap aperture", annulus, err)
	}
	for i, span := range annulus.Boundary {
		cap := h.ResultCaps[0]
		start, end := [2]int64{-3, 1}[i], [2]int64{-1, 3}[i]
		walls := [2]int{proof.OuterWalls[0], inner}
		first, last := []int{cap, walls[i]}, []int{cap, walls[1-i]}
		slices.Sort(first)
		slices.Sort(last)
		if span.Start.Min.Cmp(r(start, 1)) != 0 || span.End.Max.Cmp(r(end, 1)) != 0 ||
			!slices.Equal(span.Faces, []int{cap}) || !slices.Equal(span.StartFaces, first) || !slices.Equal(span.EndFaces, last) {
			t.Fatal("annular cap spans must retain both distinct exact rim owners", span)
		}
	}
	aperture, err := restored.ClipWrappedRationalStationBoreCapLineRational(ctx, capOrigin, capDirection, r(-1, 2), r(1, 2), lineOptions)
	if err != nil || len(aperture.Boundary)+len(aperture.Hits) != 0 {
		t.Fatal("a clip inside the opening has no cap", aperture, err)
	}
	mass, err := restored.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
	want := 3.1748736333785317 * 16
	if err != nil || math.Abs(mass.Volume-want) > 1e-5 {
		t.Fatal("independent weighted outer minus inner integral", mass, want, err)
	}
	bounds, err := restored.BoundWrappedRationalStationBoreMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: r(1, 100)})
	if err != nil || !bounds.Source.Complete || bounds.Source.Caps != h.ResultCaps ||
		bounds.AxialVolumeFactor.Cmp(r(16, 1)) != 0 || bounds.AxialCubicFactor.Cmp(r(183, 4)) != 0 ||
		bounds.AxialFirstZFactor.Cmp(r(101, 4)) != 0 || bounds.Centroid[2].Lower.Cmp(r(101, 64)) != 0 ||
		bounds.Centroid[2].Upper.Cmp(r(101, 64)) != 0 {
		t.Fatal("correlated weighted bore volume and exact moments", bounds, err)
	}
	lower, _ := bounds.Volume.Lower.Float64()
	upper, _ := bounds.Volume.Upper.Float64()
	if lower > want || upper < want || upper-lower > .01 || bounds.Centroid[0].Lower.Cmp(r(1, 4)) != 0 || bounds.Centroid[1].Upper.Cmp(r(-1, 2)) != 0 {
		t.Fatal("weighted area enclosure or original center", bounds)
	}
	if partial, err := restored.BoundWrappedRationalStationBoreMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: r(1, 100), MaxOperations: 1}); err == nil || partial.Volume.Lower != nil {
		t.Fatal("mass proof must publish no partial bound on exhausted quota", partial, err)
	}
	if out, _, err := CombineCoaxialWrappedRationalStationLofts(ctx, tool, stock, parametric.Difference, o); err != nil || len(out.faces) != 0 {
		t.Fatal("contained tool-minus-stock should be empty", err)
	}
}
