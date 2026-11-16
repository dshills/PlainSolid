package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteNestedMixedConicalAP242STEP transfers document occurrence names and
// translations to a re-proved original AP242 source. Shared relationships
// must have consistent edits across all their expanded document nodes.
// Product assets, parentage, rotations and the source units must match.
func (s State) WriteNestedMixedConicalAP242STEP(ctx context.Context, w io.Writer, original io.Reader, limits STEPAssemblyOptions, geometry geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(s.Features) != 0 || limits.MaxNodes < 0 || limits.MaxDepth < 0 || limits.MaxDepth > 256 {
		return geom.ErrInvalid
	}
	if limits.MaxDepth != 0 && limits.MaxDepth < 3 {
		return geom.ErrConvergence
	}
	var source bytes.Buffer
	m, err := step.ReadNestedMixedConicalAP242Assembly(ctx, io.TeeReader(original, &source), limits.Geometry, geometry)
	if err != nil {
		return err
	}
	doc, err := mixedConicalDocument(ctx, m, limits.MaxNodes, 0)
	if err != nil {
		return err
	}
	base, _ := doc.Snapshot()
	if base.LengthUnit != s.LengthUnit || len(base.Nodes) != len(s.Nodes) || len(s.Trimmed) != len(base.Trimmed) ||
		len(s.Assets)+len(s.Planar)+len(s.Parametric) != 0 {
		return geom.ErrUnrepresentable
	}
	for id, before := range base.Trimmed {
		if err := ctx.Err(); err != nil {
			return err
		}
		after, ok := s.Trimmed[id]
		if !ok {
			return geom.ErrUnrepresentable
		}
		var a, b bytes.Buffer
		if err := trimmed.WriteJSON(&a, before); err != nil {
			return err
		}
		if err := trimmed.WriteJSON(&b, after); err != nil {
			return err
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			return geom.ErrUnrepresentable
		}
	}
	// Use the immutable source labels from the re-proved baseline. Current
	// document labels are editable user data, not authority to retarget a use.
	uses := make(map[int64]Node, len(m.Occurrences))
	oldUses := make(map[int64]Node, len(m.Occurrences))
	for key, before := range base.Nodes {
		if err := ctx.Err(); err != nil {
			return err
		}
		after, ok := s.Nodes[key]
		if !ok || after.Parent != before.Parent || after.Asset != before.Asset {
			return geom.ErrUnrepresentable
		}
		if before.Parent == "" {
			if after != before {
				return geom.ErrUnrepresentable
			}
			continue
		}
		for i, row := range after.Transform {
			if [3]float64(row[:3]) != [3]float64(before.Transform[i][:3]) || math.IsNaN(row[3]) || math.IsInf(row[3], 0) {
				return geom.ErrUnrepresentable
			}
		}
		id := base.Labels["source-node:"+key].Attributes["step.source.nauo"].Integer
		if id <= 0 {
			return geom.ErrUnrepresentable
		}
		if prev, exists := uses[id]; exists && (prev.Name != after.Name || prev.Transform != after.Transform) {
			return fmt.Errorf("document: conflicting edits to shared STEP occurrence %d: %w", id, geom.ErrUnrepresentable)
		}
		uses[id], oldUses[id] = after, before
	}
	edits := make([]step.NestedMixedConicalAP242OccurrenceEdit, 0, len(uses))
	for _, o := range m.Occurrences {
		after, ok := uses[o.NAUOID]
		if !ok {
			return geom.ErrUnrepresentable
		}
		before := oldUses[o.NAUOID]
		ed := step.NestedMixedConicalAP242OccurrenceEdit{NAUOID: o.NAUOID}
		if after.Name != before.Name {
			if after.Name == "" {
				return geom.ErrUnrepresentable // An empty source name displays the product name.
			}
			ed.Name = &after.Name
		}
		if after.Transform != before.Transform {
			var origin [3]*big.Rat
			local := o.Matrix()
			for i := range origin {
				origin[i] = local[i][3]
				if after.Transform[i][3] != before.Transform[i][3] {
					origin[i] = new(big.Rat).SetFloat64(after.Transform[i][3])
				}
			}
			ed.Translation = &origin
		}
		if ed.Name != nil || ed.Translation != nil {
			edits = append(edits, ed)
		}
	}
	return step.WriteNestedMixedConicalAP242Edits(ctx, w, &source, edits, limits.Geometry, geometry)
}
