package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

const rationalSphericalHierarchyFormat = "plainsolid-step-rational-spherical-hierarchy"

type rationalHierarchyRelationClaim struct {
	Source ExactNAUOOccurrence
	Matrix [3][4]string
}
type rationalHierarchyInstanceClaim struct {
	ProductID int64
	Path      []int64
	Matrix    [3][4]string
}
type rationalSphericalHierarchyEnvelope struct {
	Format        string
	Version       int
	Source        string
	LengthUnit    float64
	Definitions   []AssemblyProduct
	Roots         []int64
	LocalProof    independentAnalyticDAGEnvelope
	Relationships []rationalHierarchyRelationClaim
	Instances     []rationalHierarchyInstanceClaim
}

func rationalHierarchyMatrixClaim(m [3][4]*big.Rat) (out [3][4]string, e error) {
	for i := range m {
		for j, q := range m[i] {
			if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
				return out, geom.ErrInvalid
			}
			out[i][j], e = extrusionDirectionDecimal(q)
			if e != nil {
				return out, e
			}
		}
	}
	return
}
func rationalSphericalHierarchyEnvelopeFor(ctx context.Context, m RationalSphericalHierarchy) (rationalSphericalHierarchyEnvelope, []IndependentExactMaterial, error) {
	q := rationalSphericalHierarchyEnvelope{Format: rationalSphericalHierarchyFormat, Version: 1, LengthUnit: m.LengthUnit, Definitions: m.Definitions, Roots: m.Roots}
	if !independentNativeSourceBound(m.Source, 4096) || len(m.Relationships) > 64 || len(m.Instances) > 64 || len(m.Definitions) > 16 {
		return q, nil, geom.ErrInvalid
	}
	b, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		return q, nil, e
	}
	q.Source = string(b)
	p := m.proof
	p.Products = m.Products
	var c []IndependentExactMaterial
	q.LocalProof, c, e = sphericalHierarchyEnvelope(ctx, p)
	if e != nil {
		return q, nil, e
	}
	for _, r := range m.Relationships {
		matrix, e := rationalHierarchyMatrixClaim(r.Matrix())
		if e != nil {
			return q, nil, e
		}
		r.pose = mappedRationalBasis{}
		q.Relationships = append(q.Relationships, rationalHierarchyRelationClaim{r, matrix})
	}
	for _, u := range m.Instances {
		if len(u.Path) < 1 || len(u.Path) > 8 {
			return q, nil, geom.ErrInvalid
		}
		matrix, e := rationalHierarchyMatrixClaim(u.Matrix())
		if e != nil {
			return q, nil, e
		}
		q.Instances = append(q.Instances, rationalHierarchyInstanceClaim{u.ProductID, u.Path, matrix})
	}
	return q, c, nil
}

// WriteRationalSphericalHierarchyJSON records exact original relationship and
// path matrices, separately from the private neutral local-material proof.
// One source reproof checks both; no rational pose becomes a float transform.
func WriteRationalSphericalHierarchyJSON(ctx context.Context, w io.Writer, m RationalSphericalHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	q, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadRationalSphericalHierarchy(ctx, bytes.NewBufferString(q.Source), Limits{}, q.LocalProof.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := rationalSphericalHierarchyEnvelopeFor(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, expected) || !reflect.DeepEqual(c, d) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 16<<20 {
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

// ReadRationalSphericalHierarchyJSON reconstructs only from original STEP,
// validating every stored rational matrix, path and native local body claim.
func ReadRationalSphericalHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalSphericalHierarchy, error) {
	bad := func() (RationalSphericalHierarchy, error) { return RationalSphericalHierarchy{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return RationalSphericalHierarchy{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	var q rationalSphericalHierarchyEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != rationalSphericalHierarchyFormat || q.Version != 1 || len(q.Relationships) > 64 || len(q.Instances) > 64 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewBufferString(q.Source), l.Source, q.LocalProof.Proof.Tolerance)
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	expected, _, e := rationalSphericalHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	for i := range q.LocalProof.Proof.Members {
		p := &q.LocalProof.Proof.Members[i]
		p.Claims, e = independentCompact(p.Claims)
		if e != nil {
			return bad()
		}
		p.Native, e = independentCompact(p.Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	return m, nil
}
