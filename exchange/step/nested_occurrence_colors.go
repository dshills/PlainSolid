package step

import (
	"bytes"
	"context"
	"io"
	"math/big"

	"plainsolid/geom"
)

// NestedColoredPath is one expanded leaf use. Path lists the original outer and
// inner NAUO IDs. Definition fields identify PRODUCT_DEFINITION records.
// An internal override applies to every incoming group path.
type NestedColoredPath struct {
	Path                                                      [2]int64
	RootDefinitionID, GroupDefinitionID, DefinitionID, BodyID int64
	Transform                                                 geom.Affine
	Color                                                     RGB
	Overridden                                                bool
}

// NestedOccurrenceColoredAssembly authenticates one repeated intermediate group
// and planar whole-body leaf colors. It does not interpret SHUO/path-specific
// overrides, group colors, face overrides, or deeper assemblies.
type NestedOccurrenceColoredAssembly struct {
	model OccurrenceColoredAssembly
	paths []NestedColoredPath
}

// Paths returns independent value copies, in original outer/inner source order.
func (m NestedOccurrenceColoredAssembly) Paths() []NestedColoredPath {
	return append([]NestedColoredPath(nil), m.paths...)
}

// Overrides preserves every original contextual claim, including equal duplicates.
func (m NestedOccurrenceColoredAssembly) Overrides() []OccurrenceSurfaceColor {
	return append([]OccurrenceSurfaceColor(nil), m.model.Overrides...)
}

// NativeAssembly returns fresh source-authenticated local planar assets and
// unexpanded occurrences. Call Paths for effective colors and composed poses.
// The returned mutable source/claim containers cannot alter this result.
func (m NestedOccurrenceColoredAssembly) NativeAssembly(ctx context.Context) (ColoredAssembly, error) {
	if ctx == nil || !independentNativeSourceRecordBound(m.model.Assembly.Assembly.Source, 4096, 8) {
		return ColoredAssembly{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return ColoredAssembly{}, e
	}
	raw, e := mappedPlanarSourceBytes(m.model.Assembly.Assembly.Source)
	if e != nil {
		return ColoredAssembly{}, e
	}
	proof, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		return ColoredAssembly{}, e
	}
	return proof.model.Assembly, nil
}

// ReadNestedOccurrenceColoredAssembly proves the original contextual chains and
// the complete two-level grammar. Cancellation surrounds the bounded legacy
// source proof; at most 64 leaf paths and 16 products are admitted.
func ReadNestedOccurrenceColoredAssembly(ctx context.Context, r io.Reader, l Limits) (NestedOccurrenceColoredAssembly, error) {
	var zero NestedOccurrenceColoredAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	m, e := readOccurrenceColoredAssembly(r, l, false)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	a := m.Assembly.Assembly
	if len(a.Roots) != 1 || len(a.Products) > 16 {
		return zero, geom.ErrInvalid
	}
	// Source mapping frames are authenticated independently of the float parser.
	// Only neutral child origins and exact cardinal parent frames are admitted.
	for id, en := range a.Source.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "ITEM_DEFINED_TRANSFORMATION" {
			tr, e := simple(a.Source, id, "ITEM_DEFINED_TRANSFORMATION", 4)
			if e != nil {
				return zero, e
			}
			from, e := mappedExactFrame(a.Source, tr.Args[2].Ref)
			if e != nil || frameTransform(from) != geom.Identity() {
				return zero, geom.ErrInvalid
			}
			to, e := mappedExactFrame(a.Source, tr.Args[3].Ref)
			if e != nil || !planarSplineRigid(frameTransform(to)) {
				return zero, geom.ErrInvalid
			}
		}
	}
	root := a.Roots[0]
	if len(m.Assembly.ProductBodies[root]) != 0 {
		return zero, geom.ErrInvalid
	}
	var outer, inner []AssemblyOccurrence
	var group int64
	for _, o := range a.Occurrences {
		if o.Parent == root {
			if group == 0 {
				group = o.Child
			}
			if o.Child != group {
				return zero, geom.ErrInvalid
			}
			outer = append(outer, o)
		}
	}
	if len(outer) < 2 || group == root || len(m.Assembly.ProductBodies[group]) != 0 {
		return zero, geom.ErrInvalid
	}
	seen := map[int64]bool{root: true, group: true}
	innerIDs := map[int64]bool{}
	for _, o := range a.Occurrences {
		if !planarSplineRigid(o.Transform) {
			return zero, geom.ErrInvalid
		}
		if o.Parent == root {
			continue
		}
		if o.Parent != group || o.Child == root || o.Child == group {
			return zero, geom.ErrInvalid
		}
		b := m.Assembly.ProductBodies[o.Child]
		if len(b) != 1 || b[0].SurfaceColor == nil || len(b[0].FaceColors) != 0 {
			return zero, geom.ErrInvalid
		}
		inner = append(inner, o)
		innerIDs[o.ID] = true
		seen[o.Child] = true
	}
	if len(inner) == 0 || len(outer) > 64/len(inner) || len(seen) != len(a.Products) {
		return zero, geom.ErrInvalid
	}
	colors := map[int64]RGB{}
	for _, c := range m.Overrides {
		if !innerIDs[c.OccurrenceID] {
			return zero, geom.ErrInvalid
		}
		colors[c.OccurrenceID] = c.Color
	}
	out := NestedOccurrenceColoredAssembly{model: m}
	for _, o := range outer {
		for _, i := range inner {
			if e = ctx.Err(); e != nil {
				return zero, e
			}
			pose, e := nestedColorExactCompose(o.Transform, i.Transform)
			if e != nil {
				return zero, e
			}
			ids, _, e := a.productItems(i.Child)
			if e != nil || len(ids) != 1 {
				return zero, geom.ErrInvalid
			}
			color := *m.Assembly.ProductBodies[i.Child][0].SurfaceColor
			c, override := colors[i.ID]
			if override {
				color = c
			}
			out.paths = append(out.paths, NestedColoredPath{[2]int64{o.ID, i.ID}, root, group, i.Child, ids[0], pose, color, override})
		}
	}
	return out, nil
}

// Composition must be represented without binary64 rounding, even for otherwise
// valid proper signed frames with large or very small translated origins.
func nestedColorExactCompose(a, b geom.Affine) (geom.Affine, error) {
	var out geom.Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			q := new(big.Rat)
			for k := 0; k < 3; k++ {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(a[i][k]), new(big.Rat).SetFloat64(b[k][j])))
			}
			if j == 3 {
				q.Add(q, new(big.Rat).SetFloat64(a[i][3]))
			}
			v, exact := q.Float64()
			if !exact {
				return geom.Affine{}, geom.ErrUnrepresentable
			}
			out[i][j] = v
		}
	}
	return out, nil
}

// WriteNestedOccurrenceColoredAssemblySource re-proves the original graph and
// emits it unchanged semantically; derived native geometry is never substituted.
func WriteNestedOccurrenceColoredAssemblySource(ctx context.Context, w io.Writer, m NestedOccurrenceColoredAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if !independentNativeSourceRecordBound(m.model.Assembly.Assembly.Source, 4096, 8) {
		return geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(m.model.Assembly.Assembly.Source)
	if e != nil {
		return e
	}
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
