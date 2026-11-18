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

// AffineTriangleChainJSONOptions uses the same bounded stages as the pair
// codec but a distinct schema. MaxWork is cumulative over all three support
// imports, both mates, outer point proof, native pairs and indexed chain uses.
type AffineTriangleChainJSONOptions AffineTriangleShellJSONOptions

type affineTriangleContactRecipe struct {
	Triangles [2][3][2]string `json:"triangles"`
	Normal    [3]string       `json:"normal"`
	UV        [2][2]string    `json:"uv"`
	Offset    [2]string       `json:"offset"`
	Level     string          `json:"level"`
}
type affineTriangleChainRecipe struct {
	Version    int                             `json:"version"`
	Kind       string                          `json:"kind"`
	Patches    []geom.ExactTensorSurfaceRecipe `json:"patches"`
	Mates      []affineTriangleContactRecipe   `json:"mates"`
	Outer      affineTriangleContactRecipe     `json:"outer"`
	Vertices   [2]int                          `json:"vertices"`
	Point      [3]string                       `json:"point"`
	Projection [2][2]int                       `json:"projection"`
	Witness    wrappedShellWitness             `json:"witness"`
}

func encodeTriangleContact(t [2][3][2]*big.Rat, q geom.SurfaceAffineTriangleSeparator) affineTriangleContactRecipe {
	n := affineTriangleContactRecipe{Level: q.Level.RatString()}
	for i := range q.Normal {
		n.Normal[i] = q.Normal[i].RatString()
	}
	for owner := range t {
		for i := range t[owner] {
			for j := range t[owner][i] {
				n.Triangles[owner][i][j] = t[owner][i][j].RatString()
			}
		}
		n.Offset[owner] = q.Offset[owner].RatString()
		for j := range q.UV[owner] {
			n.UV[owner][j] = q.UV[owner][j].RatString()
		}
	}
	return n
}
func parseTriangleContact(n affineTriangleContactRecipe, bits int, remaining *int) ([2][3][2]*big.Rat, geom.SurfaceAffineTriangleSeparator, error) {
	var t [2][3][2]*big.Rat
	var q geom.SurfaceAffineTriangleSeparator
	parse := func(s string) (*big.Rat, error) {
		*remaining--
		if *remaining < 1 || len(s) > bits+3 {
			return nil, geom.ErrConvergence
		}
		for _, c := range s {
			if c != '-' && c != '/' && (c < '0' || c > '9') {
				return nil, geom.ErrInvalid
			}
		}
		v, ok := new(big.Rat).SetString(s)
		if !ok || v.RatString() != s {
			return nil, geom.ErrInvalid
		}
		if v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
			return nil, geom.ErrConvergence
		}
		return v, nil
	}
	var e error
	q.Level, e = parse(n.Level)
	if e != nil {
		return t, q, e
	}
	for i := range q.Normal {
		q.Normal[i], e = parse(n.Normal[i])
		if e != nil {
			return t, q, e
		}
	}
	for owner := range t {
		q.Offset[owner], e = parse(n.Offset[owner])
		if e != nil {
			return t, q, e
		}
		for j := range q.UV[owner] {
			q.UV[owner][j], e = parse(n.UV[owner][j])
			if e != nil {
				return t, q, e
			}
		}
		for i := range t[owner] {
			for j := range t[owner][i] {
				t[owner][i][j], e = parse(n.Triangles[owner][i][j])
				if e != nil {
					return t, q, e
				}
			}
		}
	}
	return t, q, nil
}

// WriteAffineTriangleChainShellJSON re-proves all source contacts and actual
// native-use bindings before any bytes are written. The middle source occurs
// only once in the tensor recipe; both mate triangles refer to that same patch.
func WriteAffineTriangleChainShellJSON(ctx context.Context, dst io.Writer, s *AffineTriangleChainShell, options AffineTriangleChainJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.graph == nil || s.outer == nil || s.pairs[0] == nil || s.pairs[1] == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := triangleShellJSONOptions(AffineTriangleShellJSONOptions(options))
	if e != nil {
		return e
	}
	n := affineTriangleChainRecipe{Version: 1, Kind: "exact-affine-three-triangle-chain", Vertices: s.outer.Vertices(), Projection: s.pairs[0].projection}
	a, _, _ := s.pairs[0].contact.Source(0)
	b, _, _ := s.pairs[0].contact.Source(1)
	c, _, _ := s.pairs[1].contact.Source(1)
	reserved := 0
	for i, p := range []*geom.SurfacePatch{a, b, c} {
		support, ok := s.graph.faces[i].Surface.(*geom.ExactTensorSurface)
		if !ok || support.ExactPatch() != p {
			return geom.ErrDomain
		}
		if reserved >= o.MaxWork {
			return geom.ErrConvergence
		}
		r, work, e := p.ExportExactTensorRecipeBounded(ctx, o.MaxBytes, geom.SurfacePatchOptions{MaxOperations: o.MaxWork - reserved, MaxCoefficientBits: o.MaxBits, MaxPoles: o.MaxPoles})
		if e != nil {
			return e
		}
		reserved += work
		n.Patches = append(n.Patches, r)
	}
	for _, pair := range s.pairs {
		var t [2][3][2]*big.Rat
		for owner := range t {
			_, t[owner], _ = pair.contact.Source(owner)
		}
		n.Mates = append(n.Mates, encodeTriangleContact(t, pair.contact.Separator()))
	}
	var t [2][3][2]*big.Rat
	for owner := range t {
		_, t[owner], _ = s.outer.Source(owner)
	}
	n.Outer = encodeTriangleContact(t, s.outer.Separator())
	for i, v := range s.outer.Point() {
		n.Point[i] = v.RatString()
	}
	n.Witness, e = wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return e
	}
	raw, e := json.Marshal(n)
	if e != nil {
		return e
	}
	if int64(len(raw)) > o.MaxBytes {
		return geom.ErrConvergence
	}
	if reserved >= o.MaxWork {
		return geom.ErrConvergence
	}
	o.MaxWork -= reserved
	restored, e := decodeAffineTriangleChain(ctx, n, o)
	if e != nil {
		return e
	}
	for fi, f := range s.graph.faces {
		if f.Reversed || len(f.Holes) != 0 || !reflect.DeepEqual(f.Outer, restored.graph.faces[fi].Outer) {
			return geom.ErrDomain
		}
		for _, u := range f.Outer {
			if restored.operations >= o.MaxWork {
				return geom.ErrConvergence
			}
			cap := min(1000000, o.MaxWork-restored.operations)
			same, e := compareTriangleNativeUse(ctx, s.graph.edges[u.Edge], f.Surface, u, edgecheck.Options{MaxWork: cap, MaxBits: o.MaxBits})
			if e != nil {
				return e
			}
			if !same {
				return geom.ErrDomain
			}
			restored.operations += cap
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

// ReadAffineTriangleChainShellJSON reconstructs all exact source and native
// proofs, and rejects stale fan, exterior, shared-point or owner witnesses.
func ReadAffineTriangleChainShellJSON(ctx context.Context, src io.Reader, options AffineTriangleChainJSONOptions) (*AffineTriangleChainShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := triangleShellJSONOptions(AffineTriangleShellJSONOptions(options))
	if e != nil {
		return nil, e
	}
	raw, e := io.ReadAll(io.LimitReader(src, o.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > o.MaxBytes {
		return nil, geom.ErrConvergence
	}
	if e = checkWrappedRecipeJSON(ctx, raw); e != nil {
		return nil, e
	}
	var n affineTriangleChainRecipe
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeAffineTriangleChain(ctx, n, o)
}
func decodeAffineTriangleChain(ctx context.Context, n affineTriangleChainRecipe, o AffineTriangleShellJSONOptions) (*AffineTriangleChainShell, error) {
	if n.Version != 1 || n.Kind != "exact-affine-three-triangle-chain" || len(n.Patches) != 3 || len(n.Mates) != 2 || len(n.Witness.Vertices) != 5 || len(n.Witness.Edges) != 7 || len(n.Witness.FaceUses) != 3 || len(n.Witness.Boundary) != 5 {
		return nil, geom.ErrInvalid
	}
	for _, i := range n.Vertices {
		if i < 0 || i > 2 {
			return nil, geom.ErrInvalid
		}
	}
	for _, s := range n.Point {
		if len(s) > o.MaxBits+3 {
			return nil, geom.ErrConvergence
		}
	}
	remaining := o.MaxWork
	var triangles [3][2][3][2]*big.Rat
	var sep [3]geom.SurfaceAffineTriangleSeparator
	for i, r := range []affineTriangleContactRecipe{n.Mates[0], n.Mates[1], n.Outer} {
		var e error
		triangles[i], sep[i], e = parseTriangleContact(r, o.MaxBits, &remaining)
		if e != nil {
			return nil, e
		}
	}
	var patches [3]*geom.SurfacePatch
	for i, r := range n.Patches {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		p, e := geom.ImportExactTensorSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{MaxOperations: remaining, MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return nil, e
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
	var mates [2]*geom.SurfaceAffineTriangleMate
	for i := range mates {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		q := sep[i]
		p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[i], patches[i+1], triangles[i], q.Normal, q.UV, q.Offset, q.Level, contactOptions())
		if e != nil {
			return nil, e
		}
		remaining -= p.ExactOperations()
		mates[i] = p
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	outer, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, patches[0], patches[2], triangles[2], n.Vertices, sep[2], contactOptions())
	if e != nil {
		return nil, e
	}
	remaining -= outer.ExactOperations()
	for i, v := range outer.Point() {
		if n.Point[i] != v.RatString() {
			return nil, geom.ErrDomain
		}
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	so := o.Shell
	so.MaxWork = triangleStageCap(so.MaxWork, remaining)
	so.Proof.MaxBits = triangleStageCap(so.Proof.MaxBits, o.MaxBits)
	for i := range so.Projection {
		so.Projection[i].ProjectionAxes = n.Projection[i]
	}
	s, e := NewAffineTriangleChainShell(ctx, mates[0], mates[1], outer, so)
	if e != nil {
		return nil, e
	}
	remaining -= s.operations
	witness, e := wrappedIndexedShellWitness(s.graph.vertices, s.graph.edges, s.graph.faces, s.graph.vertexSources, s.graph.edgeSources, s.graph.boundary)
	if e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(witness, n.Witness) {
		return nil, geom.ErrDomain
	}
	s.operations = o.MaxWork - remaining
	return s, nil
}
