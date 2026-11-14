package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/geom"
	"reflect"
)

// ReindexedConicCircularPrismMaterial retains source IDs and ordinals separately
// from the canonical exact circular native material proof. Placement and hollow
// material remain unsupported here.
type ReindexedConicCircularPrismMaterial struct {
	Material             ConicCircularPrismMaterial
	SourceForCanonical   map[int]int
	SourceVertexOrdinals map[int]int
	SourceUseOrdinals    map[int][]int
	NativeFaceSourceIDs  []int
	SourceRimCurveIDs    [2]int
	source               []byte
}

func ReadReindexedConicCircularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedConicCircularPrismMaterial, error) {
	bad := func() (ReindexedConicCircularPrismMaterial, error) {
		return ReindexedConicCircularPrismMaterial{}, fmt.Errorf("iges: unsupported reindexed circular conic material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ReindexedConicCircularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 20)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil || len(doc.entities) != 20 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	if _, e = normalizeCircularPrismSource(&doc); e != nil {
		return bad()
	}
	normalized, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	proof, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(normalized), l, tol)
	if e != nil || proof.Kind != "filled" || proof.Filled == nil {
		return bad()
	}
	// Invert only the exact temporary conic normalization, on the now-proved
	// canonical graph. No coordinate or reference is inferred or rounded.
	canonical, e := readPlacedTopologyRecords(bytes.NewReader(proof.Filled.Topology.source), l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil {
		return bad()
	}
	for i, rec := range canonical.entities {
		if rec.kind == 104 {
			v := append([]parameter(nil), rec.values...)
			v[3] = v[1]
			canonical.entities[i].values = v
		}
	}
	originalCircle, e := mixedRoofFile(canonical)
	if e != nil {
		return bad()
	}
	material, e := ReadConicCircularPrismMaterial(ctx, bytes.NewReader(originalCircle), l, tol)
	if e != nil {
		return bad()
	}
	return ReindexedConicCircularPrismMaterial{material, proof.SourceForCanonical, proof.SourceVertexOrdinals, proof.SourceUseOrdinals, proof.NativeFaceSourceIDs, proof.SourceRimCurveIDs[0], source}, nil
}
func reindexedCircleProof(ctx context.Context, m ReindexedConicCircularPrismMaterial) ([]byte, error) {
	var b bytes.Buffer
	e := WriteConicCircularPrismMaterialJSON(ctx, &b, m.Material)
	return b.Bytes(), e
}
func reindexedCircleEqual(ctx context.Context, a, b ReindexedConicCircularPrismMaterial) bool {
	if !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || !reflect.DeepEqual(a.NativeFaceSourceIDs, b.NativeFaceSourceIDs) || a.SourceRimCurveIDs != b.SourceRimCurveIDs {
		return false
	}
	x, e := reindexedCircleProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedCircleProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedConicCircularPrismSource(ctx context.Context, w io.Writer, m ReindexedConicCircularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedConicCircularPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Material.Solid.Tolerance())
	if e != nil {
		return e
	}
	if !reindexedCircleEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedCircleEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedConicCircularPrismMaterialJSON(ctx context.Context, w io.Writer, m ReindexedConicCircularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteReindexedConicCircularPrismSource(ctx, &source, m); e != nil {
		return e
	}
	proof, e := reindexedCircleProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedCircleEnvelope{"plainsolid-iges-reindexed-conic-circular-prism", 1, source.String(), m.Material.Solid.Tolerance(), proof})
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadReindexedConicCircularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedConicCircularPrismMaterial, error) {
	bad := func() (ReindexedConicCircularPrismMaterial, error) {
		return ReindexedConicCircularPrismMaterial{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(1<<20))
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(b)) > limit {
		return bad()
	}
	var q reindexedCircleEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-conic-circular-prism" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadReindexedConicCircularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	p, e := reindexedCircleProof(ctx, m)
	if e != nil {
		return bad()
	}
	var x, y bytes.Buffer
	if json.Compact(&x, q.Proof) != nil || json.Compact(&y, p) != nil || !bytes.Equal(x.Bytes(), y.Bytes()) {
		return bad()
	}
	return m, nil
}
