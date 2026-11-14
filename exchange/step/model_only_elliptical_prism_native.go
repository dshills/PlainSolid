package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"reflect"
)

type modelOnlyPrismRecipe struct {
	Format          string `json:"format"`
	Version         int    `json:"version"`
	Source          string `json:"source_step"`
	NativeTolerance float64
	Native          json.RawMessage
	FaceIDs         [6]int64
	RimEdges        [2][4]int
}

// WriteModelOnlyEllipticalPrismJSON persists exact source, native material and
// ancestry under a separate version1 format with no repair tolerance or phase.
func WriteModelOnlyEllipticalPrismJSON(ctx context.Context, w io.Writer, m ModelOnlyEllipticalPrism) error {
	if ctx == nil || w == nil || !ellipseOwnedSourceBound(m.Source, 76) {
		return fmt.Errorf("step: invalid exact prism recipe")
	}
	raw, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		return e
	}
	canonical, e := ReadModelOnlyEllipticalPrism(ctx, bytes.NewReader(raw), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	actual, e := prismNativeBytes(m.Solid)
	if e != nil {
		return e
	}
	expected, e := prismNativeBytes(canonical.Solid)
	if e != nil {
		return e
	}
	tol := m.Solid.Tolerance()
	m.Solid = trimmed.Solid{}
	canonical.Solid = trimmed.Solid{}
	if !bytes.Equal(actual, expected) || !reflect.DeepEqual(m, canonical) {
		return fmt.Errorf("step: stale exact prism geometry or ancestry")
	}
	b, e := json.Marshal(modelOnlyPrismRecipe{Format: "plainsolid-exact-model-elliptical-prism", Version: 1, Source: string(raw), NativeTolerance: tol, Native: actual, FaceIDs: m.NativeFaceSourceIDs, RimEdges: m.NativeRimEdges})
	if e != nil {
		return e
	}
	if len(b) > 512<<10 {
		return fmt.Errorf("step: exact prism recipe budget")
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadModelOnlyEllipticalPrismJSON regenerates the exact native material after
// proving its complete original model-only source. Tampering fails atomically.
func ReadModelOnlyEllipticalPrismJSON(ctx context.Context, r io.Reader, l Limits) (ModelOnlyEllipticalPrism, error) {
	fail := func() (ModelOnlyEllipticalPrism, error) {
		return ModelOnlyEllipticalPrism{}, fmt.Errorf("step: invalid exact prism recipe")
	}
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	b, e := io.ReadAll(io.LimitReader(r, (512<<10)+1))
	if e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	if len(b) > 512<<10 {
		return fail()
	}
	var q modelOnlyPrismRecipe
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return fail()
	}
	var tail any
	if d.Decode(&tail) != io.EOF || q.Format != "plainsolid-exact-model-elliptical-prism" || q.Version != 1 {
		return fail()
	}
	m, e := ReadModelOnlyEllipticalPrism(ctx, bytes.NewBufferString(q.Source), l, q.NativeTolerance)
	if e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	expected, e := prismNativeBytes(m.Solid)
	if e != nil {
		return fail()
	}
	var a, c any
	if json.Unmarshal(q.Native, &a) != nil || json.Unmarshal(expected, &c) != nil || !reflect.DeepEqual(a, c) || q.FaceIDs != m.NativeFaceSourceIDs || q.RimEdges != m.NativeRimEdges {
		return fail()
	}
	return m, nil
}
