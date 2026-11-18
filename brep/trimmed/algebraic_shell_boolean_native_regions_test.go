//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestNativeTubeRegionComponentsSingleLens(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	options := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	options.Trace.Coverage.Isolation.SeparatingProjections = true
	options.Trace.Coverage.Isolation.ParameterTolerance = .25
	options.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, options)
	if e != nil {
		t.Fatal(e)
	}
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	native, e := result.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{})
	if e != nil || len(native) != 1 {
		t.Fatal(len(native), e)
	}
	legacy, e := result.NativeTubeLens(ctx, 1e-7, edgecheck.Options{})
	if e != nil || !reflect.DeepEqual(legacy.Topology(), native[0].Topology()) {
		t.Fatal("legacy lens changed", e)
	}
	if native[0].Topology().EulerCharacteristic != 2 || native[0].Topology().InnerWires != 0 || len(native[0].Faces()) != 2 {
		t.Fatal(native[0].Topology())
	}
	for _, face := range native[0].Faces() {
		if face.Surface != a.faces[0].Surface && face.Surface != b.faces[1].Surface {
			t.Fatal("lost original support")
		}
	}
	// Public diagnostic tables cannot change the immutable material binding.
	result.Faces = nil
	result.Edges = nil
	result.Vertices = nil
	bad := result
	bad.faces = append([]AlgebraicShellBooleanFace(nil), result.faces...)
	bad.faces[0].Holes = [][]AlgebraicShellBooleanUse{bad.faces[0].Outer}
	if partial, e := bad.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{}); e == nil || partial != nil {
		t.Fatal("reused outer as hole", e)
	}
	bad = result
	bad.faces = append([]AlgebraicShellBooleanFace(nil), result.faces...)
	bad.faces[1].Operand = bad.faces[0].Operand
	if partial, e := bad.NativeTubeRegionComponents(ctx, 1e-7, edgecheck.Options{}); e == nil || partial != nil {
		t.Fatal("missing opposite owner", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if partial, e := result.NativeTubeRegionComponents(cancelled, 1e-7, edgecheck.Options{}); e != context.Canceled || partial != nil {
		t.Fatal("cancel", e)
	}
}
