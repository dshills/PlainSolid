package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

type nativeAlgebraicGraphStock struct {
	Version int             `json:"version"`
	Kind    string          `json:"kind"`
	Stock   json.RawMessage `json:"stock"`
	Planes  [][][]*float64  `json:"planes"`
	Compose bool            `json:"compose"`
}

// WriteAlgebraicGraphStockJSON stores one original source stock and two
// oriented cut planes. The indexed topology, exact roots and ancestry are
// recomputed on reload rather than encoded as authoritative input.
func WriteAlgebraicGraphStockJSON(w io.Writer, pair AlgebraicGraphCapPair) error {
	if w == nil || pair.caps[0].recipe == nil || pair.caps[1].recipe == nil || len(pair.seams) == 0 {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := WriteJSON(&b, pair.caps[0].recipe.stock); e != nil {
		return e
	}
	doc := nativeAlgebraicGraphStock{Version: 1, Kind: "trimmed-algebraic-graph-stock", Stock: b.Bytes(),
		Compose: pair.caps[0].recipe.compose}
	for _, c := range pair.caps {
		doc.Planes = append(doc.Planes, graphCapEncodePlane(c.recipe.section))
	}
	raw, e := json.Marshal(doc)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadAlgebraicGraphStockJSON re-proves both original cap arrangements,
// their sewn exact source topology, and all individual stock coedges using
// the supplied resource limits. No partial pair or topology is returned.
func ReadAlgebraicGraphStockJSON(ctx context.Context, r io.Reader, limits JSONLimits, section RationalGraphShellSectionOptions,
	cap RationalGraphShellRegionOptions, stock geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapPair, AlgebraicGraphShellBoundary, error) {
	var noPair AlgebraicGraphCapPair
	var noShell AlgebraicGraphShellBoundary
	if ctx == nil || r == nil || limits.MaxBytes < 0 || limits.MaxBytes > 1<<30 || limits.MaxEntities < 0 || limits.MaxControlPoints < 0 {
		return noPair, noShell, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return noPair, noShell, e
	}
	if limits.MaxBytes == 0 {
		limits.MaxBytes = 32 << 20
	}
	b, e := io.ReadAll(io.LimitReader(r, limits.MaxBytes+1))
	if e != nil {
		return noPair, noShell, e
	}
	if int64(len(b)) > limits.MaxBytes {
		return noPair, noShell, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var doc nativeAlgebraicGraphStock
	if e := d.Decode(&doc); e != nil {
		return noPair, noShell, e
	}
	if e := d.Decode(new(any)); e != io.EOF || doc.Version != 1 || doc.Kind != "trimmed-algebraic-graph-stock" || len(doc.Planes) != 2 || len(doc.Stock) == 0 {
		return noPair, noShell, geom.ErrInvalid
	}
	var planes [2][3]geom.Vec3
	for i, p := range doc.Planes {
		planes[i], e = graphCapDecodePlane(p)
		if e != nil {
			return noPair, noShell, e
		}
	}
	source, e := ReadJSONContext(ctx, bytes.NewReader(doc.Stock), limits)
	if e != nil {
		return noPair, noShell, e
	}
	section.ComposeSectionCurves = doc.Compose
	clipped, e := source.ClipAlgebraicGraphCapPair(ctx, planes, section, cap)
	if e != nil {
		return noPair, noShell, e
	}
	indexed, e := clipped.ExactIndexedBoundary(ctx, section, stock)
	if e != nil {
		return noPair, noShell, e
	}
	return clipped, indexed, nil
}
