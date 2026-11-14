package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
	"sort"
)

const independentTorusAssemblyNativeFormat = "plainsolid-step-independent-torus-assembly"

type independentTorusAssemblyMemberEnvelope struct {
	DefinitionID   int64
	TorusFrame     *geom.Affine `json:",omitempty"`
	Claims, Native json.RawMessage
}
type independentTorusAssemblyEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Assembly  Assembly
	Members   []independentTorusAssemblyMemberEnvelope
}

// Only native bodies are stored once per local product, never per occurrence.
// Source is stored separately from the occurrence claims to avoid a second
// authoritative graph. Caller-owned data is bounded before serialization.
func makeIndependentTorusAssemblyEnvelope(ctx context.Context, m IndependentTorusAssembly, source []byte, cylinderMode ...bool) (independentTorusAssemblyEnvelope, []IndependentExactMaterial, error) {
	q := independentTorusAssemblyEnvelope{Format: independentTorusAssemblyNativeFormat, Version: 1, Source: string(source), Assembly: m.Assembly}
	q.Assembly.Source = nil
	cylinder := len(cylinderMode) >= 1 && cylinderMode[0]
	dag := cylinder && len(cylinderMode) >= 2 && cylinderMode[1]
	hierarchy := dag && len(cylinderMode) >= 3 && cylinderMode[2]
	sphere := hierarchy && len(cylinderMode) == 4 && cylinderMode[3]
	products := 3
	if cylinder {
		products = 4
		q.Format = independentAnalyticAssemblyNativeFormat
	}
	locals := products - 1
	if dag {
		products = 5
		locals = 3
		q.Format = independentAnalyticDAGClaimsFormat
	}
	if hierarchy {
		products = len(m.Assembly.Products)
		locals = 3
		q.Format = independentAnalyticHierarchyClaimsFormat
		if products < 4 || products > 16 {
			return q, nil, geom.ErrInvalid
		}
	}
	if sphere {
		locals = len(m.Products)
		if locals < 3 || locals > 4 {
			return q, nil, geom.ErrInvalid
		}
		q.Format = independentSphericalHierarchyClaimsFormat
	}
	var trustedClaims []IndependentExactMaterial
	if len(m.Products) != locals || len(m.Assembly.Products) != products || len(m.Assembly.Roots) != 1 || len(m.Assembly.Occurrences) < products || (!hierarchy && len(m.Assembly.Occurrences) > 16) || len(m.Assembly.Occurrences) > 64 {
		return q, nil, geom.ErrInvalid
	}
	for _, p := range m.Assembly.Products {
		if len(p.Identifier) > 1024 || len(p.Name) > 1024 || len(p.Description) > 1024 || len(p.Representations) != 1 {
			return q, nil, geom.ErrInvalid
		}
	}
	for _, u := range m.Assembly.Occurrences {
		if len(u.Identifier) > 1024 || len(u.Name) > 1024 || !planarSplineRigid(u.Transform) {
			return q, nil, geom.ErrInvalid
		}
	}
	ids := make([]int64, 0, 2)
	for id := range m.Products {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if e := ctx.Err(); e != nil {
			return q, nil, e
		}
		p := m.Products[id]
		if p.SourceDefinitionID != id || len(p.Solid.Faces()) > 4096 || len(p.Solid.Vertices()) > 8192 {
			return q, nil, geom.ErrInvalid
		}
		tol := p.Solid.Tolerance()
		if !finiteNumber(tol) || tol <= 0 {
			return q, nil, geom.ErrInvalid
		}
		if p.Planar != nil {
			q.Tolerance = tol
		}
		claims, native, e := independentMaterialClaims(p, cylinder, sphere, true)
		if e != nil {
			return q, nil, e
		}
		if p.Torus == nil && p.Planar == nil && !(cylinder && p.Cylinder != nil) && !(sphere && p.Sphere != nil) {
			return q, nil, geom.ErrInvalid
		}
		b, e := json.Marshal(claims)
		if e != nil {
			return q, nil, e
		}
		member := independentTorusAssemblyMemberEnvelope{DefinitionID: id, Claims: b, Native: native}
		if p.Torus != nil {
			frame := frameTransform(p.Torus.Frame)
			member.TorusFrame = &frame
		}
		if p.Cylinder != nil {
			member.Claims, e = json.Marshal(struct {
				Material      IndependentExactMaterial
				CylinderFrame geom.Affine
			}{Material: claims, CylinderFrame: frameTransform(p.Cylinder.Frame)})
			if e != nil {
				return q, nil, e
			}
		}
		if p.Sphere != nil {
			member.Claims, e = json.Marshal(struct {
				Material    IndependentExactMaterial
				SphereFrame geom.Affine
			}{claims, frameTransform(p.Sphere.Frame)})
			if e != nil {
				return q, nil, e
			}
		}
		q.Members = append(q.Members, member)
		trustedClaims = append(trustedClaims, claims)
	}
	return q, trustedClaims, nil
}

// WriteIndependentTorusAssemblyJSON re-proves the original STEP once, compares
// all occurrence/local-product claims and native encodings, then writes a
// distinct version-one source-bound envelope atomically. The effective native
// tolerance of the planar reconstruction is retained explicitly. The analytic
// torus retains its constructor-defined tolerance in its own native encoding.
func WriteIndependentTorusAssemblyJSON(ctx context.Context, w io.Writer, m IndependentTorusAssembly) error {
	return writeIndependentTorusAssemblyJSON(ctx, w, m, false)
}
func writeIndependentTorusAssemblyJSON(ctx context.Context, w io.Writer, m IndependentTorusAssembly, cylinder bool) error {
	if ctx == nil || w == nil || !independentNativeSourceBound(m.Assembly.Source, 4096) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	source, e := mappedPlanarSourceBytes(m.Assembly.Source)
	if e != nil {
		return e
	}
	actual, actualClaims, e := makeIndependentTorusAssemblyEnvelope(ctx, m, source, cylinder)
	if e != nil {
		return e
	}
	proof, e := readIndependentTorusAssembly(ctx, bytes.NewReader(source), Limits{}, actual.Tolerance, cylinder)
	if e != nil {
		return e
	}
	expected, expectedClaims, e := makeIndependentTorusAssemblyEnvelope(ctx, proof, source, cylinder)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(actualClaims, expectedClaims) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(expected)
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

// ReadIndependentTorusAssemblyJSON reconstructs only through the strict original
// assembly reader and verifies every stored claim against that single proof.
// Native JSON is compared with canonical regenerated bytes, never deserialized
// as input geometry. Source parse limits and envelope byte limits are separate.
func ReadIndependentTorusAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentTorusAssembly, error) {
	return readIndependentTorusAssemblyJSON(ctx, r, l, false)
}
func readIndependentTorusAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, cylinder bool) (IndependentTorusAssembly, error) {
	format := independentTorusAssemblyNativeFormat
	members := 2
	if cylinder {
		format = independentAnalyticAssemblyNativeFormat
		members = 3
	}
	bad := func() (IndependentTorusAssembly, error) { return IndependentTorusAssembly{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentTorusAssembly{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return IndependentTorusAssembly{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentTorusAssembly{}, e
	}
	var q independentTorusAssemblyEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != format || q.Version != 1 || len(q.Members) != members || q.Assembly.Source != nil {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	proof, e := readIndependentTorusAssembly(ctx, bytes.NewReader([]byte(q.Source)), l.Source, q.Tolerance, cylinder)
	if e != nil {
		return IndependentTorusAssembly{}, e
	}
	expected, _, e := makeIndependentTorusAssemblyEnvelope(ctx, proof, []byte(q.Source), cylinder)
	if e != nil {
		return IndependentTorusAssembly{}, e
	}
	for i := range q.Members {
		q.Members[i].Claims, e = independentCompact(q.Members[i].Claims)
		if e != nil {
			return bad()
		}
		q.Members[i].Native, e = independentCompact(q.Members[i].Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentTorusAssembly{}, e
	}
	return proof, nil
}
