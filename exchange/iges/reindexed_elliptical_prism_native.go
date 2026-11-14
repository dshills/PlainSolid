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

type reindexedEllipseIdentity struct {
	Kind                    string
	Roles                   map[int]int
	Solid, Shell, Transform int
	Faces                   []int
	Rims                    [2][2]int
}

func reindexedEllipseIdentityOf(m ReindexedEllipticalPrismMaterial) reindexedEllipseIdentity {
	return reindexedEllipseIdentity{m.Kind, m.SourceForCanonical, m.SourceSolidID, m.SourceShellID, m.SourceTransformID, m.NativeFaceSourceIDs, m.SourceRimCurveIDs}
}
func reindexedEllipseProofBytes(ctx context.Context, m ReindexedEllipticalPrismMaterial) ([]byte, error) {
	var b bytes.Buffer
	var e error
	switch m.Kind {
	case "filled":
		if m.Filled == nil || m.Annular != nil || m.Placed != nil {
			return nil, geom.ErrInvalid
		}
		e = WriteEllipticalPrismMaterialJSON(ctx, &b, *m.Filled)
	case "annular":
		if m.Annular == nil || m.Filled != nil || m.Placed != nil {
			return nil, geom.ErrInvalid
		}
		e = WriteEllipticalAnnularPrismMaterialJSON(ctx, &b, *m.Annular)
	case "placed-filled", "placed-annular":
		if m.Placed == nil || m.Filled != nil || m.Annular != nil {
			return nil, geom.ErrInvalid
		}
		if (m.Kind == "placed-annular") != (m.Placed.Annular != nil) {
			return nil, geom.ErrInvalid
		}
		e = WritePlacedEllipticalPrismMaterialJSON(ctx, &b, *m.Placed)
	default:
		return nil, geom.ErrInvalid
	}
	return b.Bytes(), e
}
func reindexedEllipseEqual(ctx context.Context, a, b ReindexedEllipticalPrismMaterial) bool {
	if !reflect.DeepEqual(reindexedEllipseIdentityOf(a), reindexedEllipseIdentityOf(b)) {
		return false
	}
	x, e := reindexedEllipseProofBytes(ctx, a)
	if e != nil {
		return false
	}
	y, e := reindexedEllipseProofBytes(ctx, b)
	return e == nil && bytes.Equal(x, y)
}

type reindexedEllipseEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Identity  reindexedEllipseIdentity
	Proof     json.RawMessage
}

func WriteReindexedEllipticalPrismMaterialJSON(ctx context.Context, w io.Writer, m ReindexedEllipticalPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid().Tolerance())
	if e != nil {
		return e
	}
	if !reindexedEllipseEqual(ctx, m, p) {
		return fmt.Errorf("iges: stale reindexed ellipse recipe")
	}
	proof, e := reindexedEllipseProofBytes(ctx, p)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(reindexedEllipseEnvelope{"plainsolid-iges-reindexed-ellipse-material", 1, string(p.source), p.Solid().Tolerance(), reindexedEllipseIdentityOf(p), proof})
	if e != nil {
		return e
	}
	if len(raw) > 1<<20 {
		return geom.ErrConvergence
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadReindexedEllipticalPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (ReindexedEllipticalPrismMaterial, error) {
	bad := func() (ReindexedEllipticalPrismMaterial, error) {
		return ReindexedEllipticalPrismMaterial{}, fmt.Errorf("iges: invalid reindexed ellipse envelope")
	}
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(1<<20))
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return bad()
	}
	var q reindexedEllipseEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-reindexed-ellipse-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadReindexedEllipticalPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	proof, e := reindexedEllipseProofBytes(ctx, m)
	if e != nil {
		return bad()
	}
	var a, b bytes.Buffer
	if json.Compact(&a, q.Proof) != nil || json.Compact(&b, proof) != nil || !bytes.Equal(a.Bytes(), b.Bytes()) || !reflect.DeepEqual(q.Identity, reindexedEllipseIdentityOf(m)) {
		return bad()
	}
	return m, nil
}
