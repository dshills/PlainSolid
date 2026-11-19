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

// AffineTriangleShellJSONOptions applies cumulative work, bounded input bytes,
// tensor coefficients and per-stage contact/shell limits. Defaults: 16MiB,
// 64M work, 4096 poles and 16384 coefficient bits. Hard maxima are 32MiB,
// 1G work, 65536 poles and 65536 bits. Old wrapped schemas are unchanged.
type AffineTriangleShellJSONOptions struct {
	MaxBytes                   int64
	MaxWork, MaxPoles, MaxBits int
	Contact                    geom.SurfaceAffineCoordinateOptions
	Shell                      AffineTriangleShellOptions
}
type affineTriangleShellRecipe struct {
	Version    int                             `json:"version"`
	Kind       string                          `json:"kind"`
	Patches    []geom.ExactTensorSurfaceRecipe `json:"patches"`
	Triangles  [2][3][2]string                 `json:"triangles"`
	Normal     [3]string                       `json:"normal"`
	UV         [2][2]string                    `json:"uv"`
	Offset     [2]string                       `json:"offset"`
	Level      string                          `json:"level"`
	Projection [2][2]int                       `json:"projection"`
	Witness    wrappedShellWitness             `json:"witness"`
}

func triangleShellJSONOptions(o AffineTriangleShellJSONOptions) (AffineTriangleShellJSONOptions, error) {
	if o.MaxBytes == 0 {
		o.MaxBytes = 16 << 20
	}
	if o.MaxWork == 0 {
		o.MaxWork = 64000000
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxBytes < 1 || o.MaxBytes > 32<<20 || o.MaxWork < 1 || o.MaxWork > 1000000000 || o.MaxPoles < 1 || o.MaxPoles > 65536 || o.MaxBits < 64 || o.MaxBits > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// WriteAffineTriangleMateShellJSON re-proves the serialized original geometry
// and every indexed owner before the first byte is published.
func WriteAffineTriangleMateShellJSON(ctx context.Context, dst io.Writer, s *AffineTriangleMateShell, o AffineTriangleShellJSONOptions) error {
	if ctx == nil || dst == nil || s == nil || s.contact == nil || s.graph == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	o, e := triangleShellJSONOptions(o)
	if e != nil {
		return e
	}
	n := affineTriangleShellRecipe{Version: 1, Kind: "exact-affine-triangle-mate-shell"}
	q := s.contact.Separator()
	n.Level = q.Level.RatString()
	for i := range q.Normal {
		n.Normal[i] = q.Normal[i].RatString()
	}
	reserved := 0
	for owner := 0; owner < 2; owner++ {
		p, t, e := s.contact.Source(owner)
		if e != nil {
			return e
		}
		u, v := p.ControlShape()
		if u > o.MaxPoles/v {
			return geom.ErrConvergence
		}
		support, ok := s.graph.faces[owner].Surface.(*geom.ExactTensorSurface)
		if !ok || support.ExactPatch() != p {
			return geom.ErrDomain
		}
		if reserved >= o.MaxWork {
			return geom.ErrConvergence
		}
		recipe, work, e := p.ExportExactTensorRecipeBounded(ctx, o.MaxBytes, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxBits, MaxOperations: o.MaxWork - reserved})
		reserved += work
		if e != nil {
			return e
		}
		n.Patches = append(n.Patches, recipe)
		for i := range t {
			for j := range t[i] {
				n.Triangles[owner][i][j] = t[i][j].RatString()
			}
		}
		n.Offset[owner] = q.Offset[owner].RatString()
		for j := range q.UV[owner] {
			n.UV[owner][j] = q.UV[owner][j].RatString()
		}
		n.Projection[owner] = s.projection[owner]
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
	restored, e := decodeAffineTriangleShell(ctx, n, o)
	if e != nil {
		return e
	}
	// Reprove the actual stored curve/use geometry as well as the recipe witness.
	for fi, f := range s.graph.faces {
		if !reflect.DeepEqual(f.Outer, restored.graph.faces[fi].Outer) || len(f.Holes) != 0 {
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
			if f.Reversed != restored.graph.faces[fi].Reversed {
				return geom.ErrDomain
			}
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

// ReadAffineTriangleMateShellJSON validates tensor knots/weights, reconstructs
// exact contact, both global projected injectivity proofs and the native graph.
func ReadAffineTriangleMateShellJSON(ctx context.Context, src io.Reader, o AffineTriangleShellJSONOptions) (*AffineTriangleMateShell, error) {
	if ctx == nil || src == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := triangleShellJSONOptions(o)
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
	var n affineTriangleShellRecipe
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&n); e != nil {
		return nil, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, geom.ErrInvalid
	}
	return decodeAffineTriangleShell(ctx, n, o)
}
func decodeAffineTriangleShell(ctx context.Context, n affineTriangleShellRecipe, o AffineTriangleShellJSONOptions) (*AffineTriangleMateShell, error) {
	if n.Version != 1 || n.Kind != "exact-affine-triangle-mate-shell" || len(n.Patches) != 2 || len(n.Witness.Vertices) != 4 || len(n.Witness.Edges) != 5 || len(n.Witness.FaceUses) != 2 || len(n.Witness.Boundary) != 4 {
		return nil, geom.ErrInvalid
	}
	remaining := o.MaxWork
	parse := func(s string) (*big.Rat, error) {
		remaining--
		if remaining < 1 || len(s) > o.MaxBits+3 {
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
		if v.Num().BitLen() > o.MaxBits || v.Denom().BitLen() > o.MaxBits {
			return nil, geom.ErrConvergence
		}
		return v, nil
	}
	var tri [2][3][2]*big.Rat
	var q geom.SurfaceAffineTriangleSeparator
	var e error
	q.Level, e = parse(n.Level)
	if e != nil {
		return nil, e
	}
	for i := range q.Normal {
		q.Normal[i], e = parse(n.Normal[i])
		if e != nil {
			return nil, e
		}
	}
	for owner := 0; owner < 2; owner++ {
		q.Offset[owner], e = parse(n.Offset[owner])
		if e != nil {
			return nil, e
		}
		for j := 0; j < 2; j++ {
			q.UV[owner][j], e = parse(n.UV[owner][j])
			if e != nil {
				return nil, e
			}
		}
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				tri[owner][i][j], e = parse(n.Triangles[owner][i][j])
				if e != nil {
					return nil, e
				}
			}
		}
	}
	var patches [2]*geom.SurfacePatch
	for i, r := range n.Patches {
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		patches[i], e = geom.ImportExactTensorSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{MaxOperations: remaining, MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return nil, e
		}
		remaining -= patches[i].ExactOperations()
	}
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	co := o.Contact
	co.MaxOperations = triangleStageCap(co.MaxOperations, remaining)
	co.MaxCoefficientBits = triangleStageCap(co.MaxCoefficientBits, o.MaxBits)
	co.MaxPoles = triangleStageCap(co.MaxPoles, o.MaxPoles)
	contact, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[0], patches[1], tri, q.Normal, q.UV, q.Offset, q.Level, co)
	if e != nil {
		return nil, e
	}
	remaining -= contact.ExactOperations()
	if remaining < 1 {
		return nil, geom.ErrConvergence
	}
	so := o.Shell
	so.MaxWork = triangleStageCap(so.MaxWork, remaining)
	so.Proof.MaxBits = triangleStageCap(so.Proof.MaxBits, o.MaxBits)
	for i := range so.Projection {
		so.Projection[i].ProjectionAxes = n.Projection[i]
	}
	s, e := NewAffineTriangleMateShell(ctx, contact, so)
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
func triangleStageCap(stage, total int) int {
	if stage == 0 {
		return total
	}
	return min(stage, total)
}
