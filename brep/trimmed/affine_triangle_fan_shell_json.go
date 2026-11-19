//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"reflect"
)

// AffineTriangleFanJSONOptions has the pair/chain hard input bounds and a
// cumulative allowance for all cyclic and opposite proofs, native uses and
// original tensor imports. Defaults and hard maxima match the pair codec.
type AffineTriangleFanJSONOptions AffineTriangleShellJSONOptions

type affineFanVertexRecipe struct {
	Contact  affineTriangleContactRecipe `json:"contact"`
	Vertices [2]int                      `json:"vertices"`
	Point    [3]string                   `json:"point"`
}
type affineTriangleFanRecipe struct {
	Version    int                             `json:"version"`
	Kind       string                          `json:"kind"`
	Patches    []geom.ExactTensorSurfaceRecipe `json:"patches"`
	Mates      []affineTriangleContactRecipe   `json:"mates"`
	Opposite   []affineFanVertexRecipe         `json:"opposite"`
	Center     int                             `json:"center"`
	Projection [2][2]int                       `json:"projection"`
	Witness    wrappedShellWitness             `json:"witness"`
}

// WriteAffineTriangleFanJSON serializes only the strict three-face domain.
func WriteAffineTriangleFanJSON(ctx context.Context, dst io.Writer, s *AffineTriangleFanShell, o AffineTriangleFanJSONOptions) error {
	if s == nil || len(s.pairs) != 3 {
		return geom.ErrInvalid
	}
	return writeTensorTriangleFan(ctx, dst, s, nil, o)
}

// WriteAffineTriangleFourFanJSON includes both original opposite proofs.
func WriteAffineTriangleFourFanJSON(ctx context.Context, dst io.Writer, s *AffineTriangleFourFanShell, o AffineTriangleFanJSONOptions) error {
	if s == nil || s.fan == nil || len(s.fan.pairs) != 4 {
		return geom.ErrInvalid
	}
	return writeTensorTriangleFan(ctx, dst, s.fan, s.opposite[:], o)
}

// ReadAffineTriangleFanJSON rebuilds only a three-face archive. It rejects a
// four-face recipe rather than discarding required opposite contact evidence.
func ReadAffineTriangleFanJSON(ctx context.Context, src io.Reader, o AffineTriangleFanJSONOptions) (*AffineTriangleFanShell, error) {
	fan, _, e := readTensorTriangleFan(ctx, src, 3, o)
	return fan, e
}

// ReadAffineTriangleFourFanJSON reconstructs all four adjacent and both
// opposite certificates before proving the native four-owner circular link.
func ReadAffineTriangleFourFanJSON(ctx context.Context, src io.Reader, o AffineTriangleFanJSONOptions) (*AffineTriangleFourFanShell, error) {
	fan, opposite, e := readTensorTriangleFan(ctx, src, 4, o)
	if e != nil {
		return nil, e
	}
	return &AffineTriangleFourFanShell{fan: fan, opposite: [2]*geom.SurfaceAffineTriangleVertexContact{opposite[0], opposite[1]}}, nil
}
func writeTensorTriangleFan(ctx context.Context, dst io.Writer, s *AffineTriangleFanShell, opposite []*geom.SurfaceAffineTriangleVertexContact, options AffineTriangleFanJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.graph == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := triangleShellJSONOptions(AffineTriangleShellJSONOptions(options))
	if e != nil {
		return e
	}
	n := len(s.pairs)
	if n != 3 && n != 4 || n == 3 && len(opposite) != 0 || n == 4 && len(opposite) != 2 {
		return geom.ErrInvalid
	}
	r := affineTriangleFanRecipe{Version: 1, Kind: "exact-affine-triangle-cyclic-fan", Center: s.center, Projection: s.pairs[0].projection}
	reserved := 0
	for i, pair := range s.pairs {
		if pair == nil || pair.contact == nil {
			return geom.ErrInvalid
		}
		p, _, _ := pair.contact.Source(0)
		surface, ok := s.graph.faces[i].Surface.(*geom.ExactTensorSurface)
		if !ok || surface.ExactPatch() != p {
			return geom.ErrDomain
		}
		if reserved >= o.MaxWork {
			return geom.ErrConvergence
		}
		recipe, work, e := p.ExportExactTensorRecipeBounded(ctx, o.MaxBytes, geom.SurfacePatchOptions{MaxOperations: o.MaxWork - reserved, MaxCoefficientBits: o.MaxBits, MaxPoles: o.MaxPoles})
		if e != nil {
			return e
		}
		reserved += work
		r.Patches = append(r.Patches, recipe)
		var t [2][3][2]*big.Rat
		for j := range t {
			_, t[j], _ = pair.contact.Source(j)
		}
		r.Mates = append(r.Mates, encodeTriangleContact(t, pair.contact.Separator()))
	}
	for _, p := range opposite {
		if p == nil {
			return geom.ErrInvalid
		}
		var t [2][3][2]*big.Rat
		for j := range t {
			_, t[j], _ = p.Source(j)
		}
		v := affineFanVertexRecipe{Contact: encodeTriangleContact(t, p.Separator()), Vertices: p.Vertices()}
		for i, x := range p.Point() {
			v.Point[i] = x.RatString()
		}
		r.Opposite = append(r.Opposite, v)
	}
	r.Witness, e = wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes || reserved >= o.MaxWork {
		return geom.ErrConvergence
	}
	o.MaxWork -= reserved
	rebuilt, _, e := decodeTensorTriangleFan(ctx, r, n, o)
	if e != nil {
		return e
	}
	for fi, f := range s.graph.faces {
		if f.Reversed || len(f.Holes) != 0 || !reflect.DeepEqual(f.Outer, rebuilt.graph.faces[fi].Outer) {
			return geom.ErrDomain
		}
		for _, u := range f.Outer {
			if rebuilt.operations >= o.MaxWork {
				return geom.ErrConvergence
			}
			cap := min(1000000, o.MaxWork-rebuilt.operations)
			same, e := compareTriangleNativeUse(ctx, s.graph.edges[u.Edge], f.Surface, u, edgecheck.Options{MaxWork: cap, MaxBits: o.MaxBits})
			if e != nil {
				return e
			}
			if !same {
				return geom.ErrDomain
			}
			rebuilt.operations += cap
		}
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	count, e := dst.Write(raw)
	if e == nil && count != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
func readTensorTriangleFan(ctx context.Context, src io.Reader, count int, options AffineTriangleFanJSONOptions) (*AffineTriangleFanShell, []*geom.SurfaceAffineTriangleVertexContact, error) {
	if ctx == nil || src == nil {
		return nil, nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	o, e := triangleShellJSONOptions(AffineTriangleShellJSONOptions(options))
	if e != nil {
		return nil, nil, e
	}
	raw, e := io.ReadAll(io.LimitReader(src, o.MaxBytes+1))
	if e != nil {
		return nil, nil, e
	}
	if int64(len(raw)) > o.MaxBytes {
		return nil, nil, geom.ErrConvergence
	}
	if e = checkWrappedRecipeJSON(ctx, raw); e != nil {
		return nil, nil, e
	}
	var r affineTriangleFanRecipe
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&r); e != nil {
		return nil, nil, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, nil, geom.ErrInvalid
	}
	return decodeTensorTriangleFan(ctx, r, count, o)
}
func decodeTensorTriangleFan(ctx context.Context, r affineTriangleFanRecipe, n int, o AffineTriangleShellJSONOptions) (*AffineTriangleFanShell, []*geom.SurfaceAffineTriangleVertexContact, error) {
	if r.Version != 1 || r.Kind != "exact-affine-triangle-cyclic-fan" || len(r.Patches) != n || len(r.Mates) != n || n == 3 && len(r.Opposite) != 0 || n == 4 && len(r.Opposite) != 2 || r.Center < 0 || r.Center > n || len(r.Witness.Vertices) != n+1 || len(r.Witness.Edges) != 2*n || len(r.Witness.FaceUses) != n || len(r.Witness.Boundary) != n {
		return nil, nil, geom.ErrInvalid
	}
	for _, v := range r.Opposite {
		for _, i := range v.Vertices {
			if i < 0 || i > 2 {
				return nil, nil, geom.ErrInvalid
			}
		}
		for _, s := range v.Point {
			if len(s) > o.MaxBits+3 {
				return nil, nil, geom.ErrConvergence
			}
		}
	}
	remaining := o.MaxWork
	triangles := make([][2][3][2]*big.Rat, n+len(r.Opposite))
	sep := make([]geom.SurfaceAffineTriangleSeparator, len(triangles))
	recipes := append([]affineTriangleContactRecipe(nil), r.Mates...)
	for _, v := range r.Opposite {
		recipes = append(recipes, v.Contact)
	}
	for i, q := range recipes {
		var e error
		triangles[i], sep[i], e = parseTriangleContact(q, o.MaxBits, &remaining)
		if e != nil {
			return nil, nil, e
		}
	}
	patches := make([]*geom.SurfacePatch, n)
	for i, q := range r.Patches {
		if remaining < 1 {
			return nil, nil, geom.ErrConvergence
		}
		p, e := geom.ImportExactTensorSurfaceRecipe(ctx, q, geom.SurfacePatchOptions{MaxOperations: remaining, MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return nil, nil, e
		}
		remaining -= p.ExactOperations()
		patches[i] = p
	}
	contactOptions := func() geom.SurfaceAffineCoordinateOptions {
		co := o.Contact
		co.MaxOperations = triangleStageCap(co.MaxOperations, remaining)
		co.MaxCoefficientBits = triangleStageCap(co.MaxCoefficientBits, o.MaxBits)
		co.MaxPoles = triangleStageCap(co.MaxPoles, o.MaxPoles)
		return co
	}
	mates := make([]*geom.SurfaceAffineTriangleMate, n)
	for i := range mates {
		if remaining < 1 {
			return nil, nil, geom.ErrConvergence
		}
		q := sep[i]
		p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[i], patches[(i+1)%n], triangles[i], q.Normal, q.UV, q.Offset, q.Level, contactOptions())
		if e != nil {
			return nil, nil, e
		}
		remaining -= p.ExactOperations()
		mates[i] = p
	}
	opposite := make([]*geom.SurfaceAffineTriangleVertexContact, len(r.Opposite))
	for i, v := range r.Opposite {
		if remaining < 1 {
			return nil, nil, geom.ErrConvergence
		}
		p, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, patches[i], patches[i+2], triangles[n+i], v.Vertices, sep[n+i], contactOptions())
		if e != nil {
			return nil, nil, e
		}
		remaining -= p.ExactOperations()
		for j, x := range p.Point() {
			if v.Point[j] != x.RatString() {
				return nil, nil, geom.ErrDomain
			}
		}
		opposite[i] = p
	}
	if remaining < 1 {
		return nil, nil, geom.ErrConvergence
	}
	so := o.Shell
	so.MaxWork = triangleStageCap(so.MaxWork, remaining)
	so.Proof.MaxBits = triangleStageCap(so.Proof.MaxBits, o.MaxBits)
	for i := range so.Projection {
		so.Projection[i].ProjectionAxes = r.Projection[i]
	}
	s, e := buildAffineTriangleFan(ctx, mates, opposite, so)
	if e != nil {
		return nil, nil, e
	}
	remaining -= s.operations
	if s.center != r.Center {
		return nil, nil, geom.ErrDomain
	}
	witness, e := wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return nil, nil, e
	}
	if !reflect.DeepEqual(witness, r.Witness) {
		return nil, nil, geom.ErrDomain
	}
	s.operations = o.MaxWork - remaining
	return s, opposite, nil
}
