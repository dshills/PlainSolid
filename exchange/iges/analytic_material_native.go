package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

type analyticMaterialEnvelope struct {
	Format              string
	Version             int
	Family              AnalyticMaterialFamily
	Source              string
	Tolerance           float64
	LengthUnit          float64
	SourceRoles         map[int]int
	NativeFaceSourceIDs []int
	Native              json.RawMessage
}

func analyticMaterialSnapshot(m AnalyticMaterial, source []byte) (analyticMaterialEnvelope, error) {
	family, e := m.Family()
	if e != nil {
		return analyticMaterialEnvelope{}, e
	}
	solid, e := m.Solid()
	if e != nil {
		return analyticMaterialEnvelope{}, e
	}
	unit, e := m.LengthUnit()
	if e != nil {
		return analyticMaterialEnvelope{}, e
	}
	native, e := igesPrismBytes(solid)
	if e != nil {
		return analyticMaterialEnvelope{}, e
	}
	var compact bytes.Buffer
	if json.Compact(&compact, native) != nil {
		return analyticMaterialEnvelope{}, geom.ErrInvalid
	}
	return analyticMaterialEnvelope{"plainsolid-iges-analytic-material", 1, family, string(source), solid.Tolerance(), unit, m.SourceRoles(), m.NativeFaceSourceIDs(), compact.Bytes()}, nil
}

// WriteAnalyticMaterialJSON stages a source-bound envelope only after the selected
// existing codec reauthenticates the material. No partial payload is written on
// proof failure or cancellation. The envelope is bounded to one MiB.
func WriteAnalyticMaterialJSON(ctx context.Context, w io.Writer, m AnalyticMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	var source bytes.Buffer
	if e := WriteAnalyticMaterialSource(ctx, &source, m); e != nil {
		return e
	}
	q, e := analyticMaterialSnapshot(m, source.Bytes())
	if e != nil {
		return e
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadAnalyticMaterialJSON reconstructs solely through ReadAnalyticMaterial and
// verifies the stored tag, units, ancestry, tolerance and native bytes. It does
// not deserialize a trusted shape or try another family after a proof failure.
// MaxBytes applies to this envelope (capped at one MiB); source parsing retains
// the dispatcher and caller's remaining limits.
func ReadAnalyticMaterialJSON(ctx context.Context, r io.Reader, l Limits) (AnalyticMaterial, error) {
	bad := func() (AnalyticMaterial, error) { return AnalyticMaterial{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return AnalyticMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(1<<20))
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return AnalyticMaterial{}, e
	}
	if e = ctx.Err(); e != nil {
		return AnalyticMaterial{}, e
	}
	if int64(len(b)) > limit {
		return bad()
	}
	var q analyticMaterialEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != "plainsolid-iges-analytic-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadAnalyticMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return AnalyticMaterial{}, e
	}
	expected, e := analyticMaterialSnapshot(p, []byte(q.Source))
	if e != nil {
		return bad()
	}
	var compact bytes.Buffer
	if json.Compact(&compact, q.Native) != nil {
		return bad()
	}
	q.Native = compact.Bytes()
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return AnalyticMaterial{}, e
	}
	return p, nil
}
