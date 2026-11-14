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

// ReindexedPlacedConicCircularPrismMaterial retains source IDs and ordinals separately
// from the canonical placed circular material proof. Both filled and hollow graphs are supported.
type ReindexedPlacedConicCircularPrismMaterial struct {
	Material             PlacedConicCircularPrismMaterial
	SourceForCanonical   map[int]int
	SourceVertexOrdinals map[int]int
	SourceUseOrdinals    map[int][]int
	NativeFaceSourceIDs  []int
	SourceRimCurveIDs    [2][2]int
	source               []byte
}

func ReadReindexedPlacedConicCircularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedPlacedConicCircularPrismMaterial, error) {
	bad := func() (ReindexedPlacedConicCircularPrismMaterial, error) {
		return ReindexedPlacedConicCircularPrismMaterial{}, fmt.Errorf("iges: unsupported reindexed circular conic material")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ReindexedPlacedConicCircularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 29)
	l.MaxValues = min(l.MaxValues, 2048)
	proof, originalCircle, source, e := reindexedPlacedCircularSource(ctx, r, l, tol)
	if e != nil {
		return bad()
	}
	material, e := ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(originalCircle), l, tol)
	if e != nil {
		return bad()
	}
	return ReindexedPlacedConicCircularPrismMaterial{material, proof.SourceForCanonical, proof.SourceVertexOrdinals, proof.SourceUseOrdinals, proof.NativeFaceSourceIDs, proof.SourceRimCurveIDs, source}, nil
}
func reindexedPlacedCircularProof(ctx context.Context, m ReindexedPlacedConicCircularPrismMaterial) ([]byte, error) {
	var b bytes.Buffer
	e := WritePlacedConicCircularPrismMaterialJSON(ctx, &b, m.Material)
	return b.Bytes(), e
}
func reindexedPlacedCircularEqual(ctx context.Context, a, b ReindexedPlacedConicCircularPrismMaterial) bool {
	if !reflect.DeepEqual(a.SourceForCanonical, b.SourceForCanonical) || !reflect.DeepEqual(a.SourceVertexOrdinals, b.SourceVertexOrdinals) || !reflect.DeepEqual(a.SourceUseOrdinals, b.SourceUseOrdinals) || !reflect.DeepEqual(a.NativeFaceSourceIDs, b.NativeFaceSourceIDs) || a.SourceRimCurveIDs != b.SourceRimCurveIDs {
		return false
	}
	x, e := reindexedPlacedCircularProof(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedPlacedCircularProof(ctx, b)
	return e == nil && bytes.Equal(x, y)
}
func WriteReindexedPlacedConicCircularPrismSource(ctx context.Context, w io.Writer, m ReindexedPlacedConicCircularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Material.Solid.Tolerance())
	if e != nil {
		return e
	}
	if !reindexedPlacedCircularEqual(ctx, m, p) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}

type reindexedPlacedCircularEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Proof     json.RawMessage
}

func WriteReindexedPlacedConicCircularPrismMaterialJSON(ctx context.Context, w io.Writer, m ReindexedPlacedConicCircularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var source bytes.Buffer
	if e := WriteReindexedPlacedConicCircularPrismSource(ctx, &source, m); e != nil {
		return e
	}
	proof, e := reindexedPlacedCircularProof(ctx, m)
	if e != nil {
		return e
	}
	b, e := json.Marshal(reindexedPlacedCircularEnvelope{"plainsolid-iges-reindexed-placed-circular-conic-prism", 1, source.String(), m.Material.Solid.Tolerance(), proof})
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
func ReadReindexedPlacedConicCircularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedPlacedConicCircularPrismMaterial, error) {
	bad := func() (ReindexedPlacedConicCircularPrismMaterial, error) {
		return ReindexedPlacedConicCircularPrismMaterial{}, geom.ErrInvalid
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
	var q reindexedPlacedCircularEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-placed-circular-conic-prism" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadReindexedPlacedConicCircularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	p, e := reindexedPlacedCircularProof(ctx, m)
	if e != nil {
		return bad()
	}
	var x, y bytes.Buffer
	if json.Compact(&x, q.Proof) != nil || json.Compact(&y, p) != nil || !bytes.Equal(x.Bytes(), y.Bytes()) {
		return bad()
	}
	return m, nil
}

// Proves exact circle coefficients and local coordinate invariance first; the
// existing placed/reindexed ellipse path then proves ownership and placement.
func reindexedPlacedCircularSource(ctx context.Context, r io.Reader, l Limits, tol float64) (ReindexedEllipticalPrismMaterial, []byte, []byte, error) {
	bad := func() (ReindexedEllipticalPrismMaterial, []byte, []byte, error) {
		return ReindexedEllipticalPrismMaterial{}, nil, nil, geom.ErrInvalid
	}
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, true, false, false, true, true, true, true, true)
	if e != nil || (len(doc.entities) != 21 && len(doc.entities) != 29) || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	var tr topologyRecord
	for _, rec := range doc.entities {
		if rec.kind == 124 {
			if tr.id != 0 {
				return bad()
			}
			tr = rec
		}
	}
	if tr.id == 0 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
		return bad()
	}
	pose, e := decodeMatrix(tr.values)
	if e != nil || !roofRigid(pose) {
		return bad()
	}
	local := doc
	local.entities = nil
	for _, rec := range doc.entities {
		if rec.kind == 124 {
			continue
		}
		want := 0
		if ellipsePlacedLeaf(rec.kind) {
			want = tr.id
		}
		if rec.transform != want {
			return bad()
		}
		rec.transform = 0
		rec.values = append([]parameter(nil), rec.values...)
		if rec.kind == 502 || rec.kind == 122 {
			if compositeSourceNumbers(110, rec.values) != nil {
				return bad()
			}
			n := 1
			if rec.kind == 502 {
				if len(rec.values) < 2 {
					return bad()
				}
				n, e = intValue(rec.values[1])
				if e != nil || (n != 2 && n != 4) {
					return bad()
				}
			}
			if len(rec.values) != 2+3*n {
				return bad()
			}
			for j := 0; j < n; j++ {
				p := [3]float64{}
				for k := 0; k < 3; k++ {
					p[k], e = realValue(rec.values[2+3*j+k])
					if e != nil {
						return bad()
					}
				}
				q, e := roofMapPoint(pose, geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, true)
				if e != nil {
					return bad()
				}
				for k, x := range []float64{q.X, q.Y, q.Z} {
					rec.values[2+3*j+k].text = number(x)
				}
			}
		}
		local.entities = append(local.entities, rec)
	}
	if _, e = normalizeCircularPrismSource(&local); e != nil {
		return bad()
	}
	// Only the conic coefficient changes in the original world graph. Every
	// other source coordinate is retained, with its exact invariance proved above.
	byID := map[int]topologyRecord{}
	for _, rec := range local.entities {
		byID[rec.id] = rec
	}
	for i, rec := range doc.entities {
		if rec.kind == 104 {
			doc.entities[i].values = byID[rec.id].values
		}
	}
	normalized, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	proof, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(normalized), l, tol)
	if e != nil || proof.Placed == nil || (proof.Kind != "placed-filled" && proof.Kind != "placed-annular") {
		return bad()
	}
	canonical, e := readPlacedTopologyRecords(bytes.NewReader(proof.Placed.source), l, true, false, false, false, false, false, true, false, false, true, true, true, true, true)
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
	original, e := mixedRoofFile(canonical)
	if e != nil {
		return bad()
	}
	return proof, original, source, nil
}
