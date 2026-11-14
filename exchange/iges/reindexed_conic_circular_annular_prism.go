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

// ReindexedConicCircularAnnularPrismMaterial retains source IDs and ordinals separately
// from the canonical exact circular annular material proof. Placement and non-annular
// material remain unsupported here.
type ReindexedConicCircularAnnularPrismMaterial struct {
	Material             ConicCircularAnnularPrismMaterial
	SourceForCanonical   map[int]int
	SourceVertexOrdinals map[int]int
	SourceUseOrdinals    map[int][]int
	NativeFaceSourceIDs  []int
	SourceRimCurveIDs    [2][2]int
	source               []byte
}

func ReadReindexedConicCircularAnnularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedConicCircularAnnularPrismMaterial, error) {
	bad := func() (ReindexedConicCircularAnnularPrismMaterial, error) {
		return ReindexedConicCircularAnnularPrismMaterial{}, fmt.Errorf("iges: unsupported reindexed circular conic material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ReindexedConicCircularAnnularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 28)
	l.MaxValues = min(l.MaxValues, 2048)
	proof, originalCircle, source, e := reindexedCircularSource(ctx, r, l, tol, true)
	if e != nil {
		return bad()
	}
	material, e := ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(originalCircle), l, tol)
	if e != nil {
		return bad()
	}
	return ReindexedConicCircularAnnularPrismMaterial{material, proof.SourceForCanonical, proof.SourceVertexOrdinals, proof.SourceUseOrdinals, proof.NativeFaceSourceIDs, proof.SourceRimCurveIDs, source}, nil
}
func reindexedCircularAnnularProof(ctx context.Context, m ReindexedConicCircularAnnularPrismMaterial) ([]byte, error) {
	var b bytes.Buffer
	e := WriteConicCircularAnnularPrismMaterialJSON(ctx, &b, m.Material)
	return b.Bytes(), e
}
func reindexedCircularAnnularEqual(ctx context.Context, a, b ReindexedConicCircularAnnularPrismMaterial) bool {
	if !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || !reflect.DeepEqual(a.NativeFaceSourceIDs, b.NativeFaceSourceIDs) || a.SourceRimCurveIDs != b.SourceRimCurveIDs {
		return false
	}
	x, e := reindexedCircularAnnularProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedCircularAnnularProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedConicCircularAnnularPrismSource(ctx context.Context, w io.Writer, m ReindexedConicCircularAnnularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Material.Solid.Tolerance())
	if e != nil {
		return e
	}
	if !reindexedCircularAnnularEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedCircularAnnularEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedConicCircularAnnularPrismMaterialJSON(ctx context.Context, w io.Writer, m ReindexedConicCircularAnnularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteReindexedConicCircularAnnularPrismSource(ctx, &source, m); e != nil {
		return e
	}
	proof, e := reindexedCircularAnnularProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedCircularAnnularEnvelope{"plainsolid-iges-reindexed-conic-circular-annular-prism", 1, source.String(), m.Material.Solid.Tolerance(), proof})
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
func ReadReindexedConicCircularAnnularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedConicCircularAnnularPrismMaterial, error) {
	bad := func() (ReindexedConicCircularAnnularPrismMaterial, error) {
		return ReindexedConicCircularAnnularPrismMaterial{}, geom.ErrInvalid
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
	var q reindexedCircularAnnularEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-conic-circular-annular-prism" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadReindexedConicCircularAnnularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	p, e := reindexedCircularAnnularProof(ctx, m)
	if e != nil {
		return bad()
	}
	var x, y bytes.Buffer
	if json.Compact(&x, q.Proof) != nil || json.Compact(&y, p) != nil || !bytes.Equal(x.Bytes(), y.Bytes()) {
		return bad()
	}
	return m, nil
}
