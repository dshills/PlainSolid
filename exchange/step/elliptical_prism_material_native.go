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

type ellipticalPrismMaterialRecipe struct {
	Format                           string `json:"format"`
	Version                          int    `json:"version"`
	Source                           string `json:"source_step"`
	RepairTolerance, NativeTolerance float64
	Native                           json.RawMessage
	Faces                            [6]int64
	RimEdges                         [2][4]int
	SourceRims                       [2]int64
}

func prismNativeBytes(s trimmed.Solid) ([]byte, error) {
	var b bytes.Buffer
	e := trimmed.WriteJSON(&b, s)
	return b.Bytes(), e
}

// WriteEllipticalPrismMaterialJSON retains the original source, enforced repair,
// exact reconstructed native geometry and original rim/face ancestry. It
// re-proves and regenerates the entire material before writing any bytes.
func WriteEllipticalPrismMaterialJSON(ctx context.Context, w io.Writer, m EllipticalPrismMaterial) error {
	if ctx == nil || w == nil || m.Shell.Side.Bounds.Tolerance == nil || !ellipseOwnedSourceBound(m.Shell.Source, 121) {
		return fmt.Errorf("step: invalid ellipse prism material recipe")
	}
	tol, ok := m.Shell.Side.Bounds.Tolerance.Float64()
	if !ok {
		return fmt.Errorf("step: invalid repair tolerance")
	}
	canonical, e := PromoteRepairedEllipticalPrism(ctx, m.Shell.Source, Limits{}, EllipticalWireRepairOptions{MaxDistance: tol}, m.Solid.Tolerance())
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
	if !bytes.Equal(actual, expected) || m.NativeFaceSourceIDs != canonical.NativeFaceSourceIDs || m.NativeRimEdges != canonical.NativeRimEdges || m.SourceRimEdgeIDs != canonical.SourceRimEdgeIDs || !reflect.DeepEqual(m.Shell, canonical.Shell) {
		return fmt.Errorf("step: stale ellipse prism material binding")
	}
	source, e := mappedPlanarSourceBytes(canonical.Shell.Source)
	if e != nil {
		return e
	}
	b, e := json.Marshal(ellipticalPrismMaterialRecipe{Format: "plainsolid-repaired-elliptical-prism-material", Version: 1, Source: string(source), RepairTolerance: tol, NativeTolerance: m.Solid.Tolerance(), Native: actual, Faces: m.NativeFaceSourceIDs, RimEdges: m.NativeRimEdges, SourceRims: m.SourceRimEdgeIDs})
	if e != nil {
		return e
	}
	if len(b) > 512<<10 {
		return fmt.Errorf("step: ellipse prism recipe limit")
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadEllipticalPrismMaterialJSON reconstructs and binds source plus native data
// atomically. The returned geometry is generated from the proved source rather
// than trusting a supplied native payload. Limits apply to original STEP data.
func ReadEllipticalPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (EllipticalPrismMaterial, error) {
	fail := func() (EllipticalPrismMaterial, error) {
		return EllipticalPrismMaterial{}, fmt.Errorf("step: invalid ellipse prism material recipe")
	}
	if ctx == nil || r == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return EllipticalPrismMaterial{}, e
	}
	b, e := io.ReadAll(io.LimitReader(r, (512<<10)+1))
	if e != nil {
		return EllipticalPrismMaterial{}, e
	}
	if len(b) > 512<<10 {
		return fail()
	}
	var q ellipticalPrismMaterialRecipe
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return fail()
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF || q.Format != "plainsolid-repaired-elliptical-prism-material" || q.Version != 1 {
		return fail()
	}
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewBufferString(q.Source), l, EllipticalWireRepairOptions{MaxDistance: q.RepairTolerance})
	if e != nil {
		return EllipticalPrismMaterial{}, e
	}
	m, e := PromoteRepairedEllipticalPrism(ctx, shell.Source, l, EllipticalWireRepairOptions{MaxDistance: q.RepairTolerance}, q.NativeTolerance)
	if e != nil {
		return EllipticalPrismMaterial{}, e
	}
	expected, e := prismNativeBytes(m.Solid)
	if e != nil {
		return fail()
	}
	var a, c any
	if json.Unmarshal(q.Native, &a) != nil || json.Unmarshal(expected, &c) != nil || !reflect.DeepEqual(a, c) || q.Faces != m.NativeFaceSourceIDs || q.RimEdges != m.NativeRimEdges || q.SourceRims != m.SourceRimEdgeIDs {
		return fail()
	}
	return m, nil
}
