package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// ObliqueNURBSProductFrame is a source-derived exact rational proper rigid
// change of product-local coordinates, not an authored occurrence transform.
// Basis columns map the private cardinal proof back to original coordinates.
type ObliqueNURBSProductFrame struct {
	DefinitionID, ProductID, BodyID, FaceID, SurfaceID int64
	PoleIDs                                            [4]int64
	Origin                                             [3]string
	Basis                                              [3][3]string
}

// ObliqueNURBSPointEvidence retains every transformed original 3D point,
// including vertices and spline poles. UV points are never transformed here.
type ObliqueNURBSPointEvidence struct {
	DefinitionID, BodyID, PointID int64
	Source, Proof                 [3]string
}

// ObliqueNURBSPlanarAssembly contains native planar bodies in the ORIGINAL
// local coordinates. Its occurrence transforms and Source are also original.
// No rounded unit axes are baked into native geometry.
type ObliqueNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	frames   []ObliqueNURBSProductFrame
	points   []ObliqueNURBSPointEvidence
}

func (m ObliqueNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m ObliqueNURBSPlanarAssembly) ProductFrames() []ObliqueNURBSProductFrame {
	return append([]ObliqueNURBSProductFrame(nil), m.frames...)
}
func (m ObliqueNURBSPlanarAssembly) SourcePoints() []ObliqueNURBSPointEvidence {
	return append([]ObliqueNURBSPointEvidence(nil), m.points...)
}

// ReadObliqueNURBSPlanarAssembly proves one original polynomial planar body per
// local product under an exact rational proper rigid basis. The entire original
// 3D body graph must transform losslessly into the scaled-cardinal NURBS subset.
// It then restores every native vertex exactly and validates global embedding.
// Skew/nonorthogonal charts, irrational basis norms, rounded coordinates and
// general curved or rational-weighted supports remain unsupported.
func ReadObliqueNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (ObliqueNURBSPlanarAssembly, error) {
	var zero ObliqueNURBSPlanarAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	l.MaxEdgeUses = min(l.MaxEdgeUses, 4096)
	l.MaxSolids = min(l.MaxSolids, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrInvalid
	}
	original, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	frames, points, bases, sourceVertices, e := obliqueNURBSProofView(ctx, original, view)
	if e != nil {
		return zero, e
	}
	data, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	proof, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewReader(data), l)
	if e != nil {
		return zero, e
	}
	a := proof.Geometry.Assembly
	if a.LengthUnit != original.LengthUnit || !reflect.DeepEqual(a.Products, original.Products) || !reflect.DeepEqual(a.Roots, original.Roots) || !reflect.DeepEqual(a.Occurrences, original.Occurrences) {
		return zero, geom.ErrInvalid
	}
	for _, product := range original.Products {
		id := product.DefinitionID
		bs := proof.Geometry.ProductBodies[id]
		basis, ok := bases[id]
		if len(bs) == 0 {
			if ok {
				return zero, geom.ErrInvalid
			}
			continue
		}
		if len(bs) != 1 || !ok {
			return zero, geom.ErrInvalid
		}
		s := bs[0].Body.Solid
		vertices := s.Vertices()
		seen := map[geom.Vec3]bool{}
		for i, p := range vertices {
			q := mappedRatVec(p)
			var v [3]*big.Rat
			for j := 0; j < 3; j++ {
				v[j] = new(big.Rat).Set(basis.Origin[j])
				for k, axis := range [][3]*big.Rat{basis.X, basis.Y, basis.Z} {
					v[j].Add(v[j], new(big.Rat).Mul(axis[j], q[k]))
				}
			}
			x, e := obliqueNURBSFloatPoint(v)
			if e != nil {
				return zero, e
			}
			if !sourceVertices[id][x] || seen[x] {
				return zero, geom.ErrInvalid
			}
			seen[x] = true
			vertices[i] = x
		}
		if len(seen) != len(sourceVertices[id]) {
			return zero, geom.ErrInvalid
		}
		native, e := brep.NewSolid(vertices, s.Edges(), s.Faces())
		if e != nil {
			return zero, e
		}
		if e = poly.ValidateEmbeddedBoundary(ctx, native, nurbsPlanarEmbeddingOptions()); e != nil {
			return zero, e
		}
		bs[0].Body.Solid = native
		proof.Geometry.ProductBodies[id] = bs
	}
	proof.Geometry.Assembly = original
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return ObliqueNURBSPlanarAssembly{proof.Geometry, string(raw), frames, points}, nil
}
func WriteObliqueNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m ObliqueNURBSPlanarAssembly, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := io.WriteString(w, p.original)
	if e == nil && n != len(p.original) {
		return io.ErrShortWrite
	}
	return e
}
func obliqueNURBSFloatPoint(q [3]*big.Rat) (geom.Vec3, error) {
	var a [3]float64
	for i, v := range q {
		if v == nil || v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return geom.Vec3{}, geom.ErrInvalid
		}
		x, exact := v.Float64()
		if !exact || !finiteNumber(x) {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
		a[i] = x
	}
	return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
}

func obliqueNURBSProofView(ctx context.Context, a Assembly, f *File) ([]ObliqueNURBSProductFrame, []ObliqueNURBSPointEvidence, map[int64]mappedRationalBasis, map[int64]map[geom.Vec3]bool, error) {
	fail := func() error {
		return fmt.Errorf("step: unsupported exact oblique NURBS product ownership/basis: %w", geom.ErrInvalid)
	}
	var frames []ObliqueNURBSProductFrame
	var points []ObliqueNURBSPointEvidence
	bases := map[int64]mappedRationalBasis{}
	vertices := map[int64]map[geom.Vec3]bool{}
	// All incoming references to changed points must belong to that body graph.
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, x := range v.Items {
			walk(id, x)
		}
	}
	for id, en := range f.Entities {
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	owned := map[int64]int64{}
	coord := func(id int64) ([3]*big.Rat, error) {
		var q [3]*big.Rat
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
			return q, fail()
		}
		for i, v := range r.Args[1].Items {
			if _, e = wrappedIsoSourceNumber(v); e != nil {
				return q, e
			}
			q[i], e = liftSourceRat(v)
			if e != nil {
				return q, e
			}
		}
		return q, nil
	}
	for _, product := range a.Products {
		if e := ctx.Err(); e != nil {
			return nil, nil, nil, nil, e
		}
		ids, advanced, e := a.productItems(product.DefinitionID)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		if len(ids) == 0 {
			continue
		}
		if len(ids) != 1 || len(advanced) != 1 || !advanced[0] {
			return nil, nil, nil, nil, fail()
		}
		body := ids[0]
		faceIDs, e := bodyFaceIDs(f, body)
		if e != nil || len(faceIDs) == 0 {
			return nil, nil, nil, nil, fail()
		}
		face, e := simple(f, faceIDs[0], "ADVANCED_FACE", 4)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return nil, nil, nil, nil, e
		}
		surface, e := simple(f, sid, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
		if e != nil || surface.Args[3].Kind != Aggregate || len(surface.Args[3].Items) != 2 {
			return nil, nil, nil, nil, fail()
		}
		var poleIDs [4]int64
		var p [4][3]*big.Rat
		for i := 0; i < 2; i++ {
			row, e := listRefs(surface.Args[3].Items[i], 2)
			if e != nil || len(row) != 2 {
				return nil, nil, nil, nil, fail()
			}
			for j := 0; j < 2; j++ {
				k := 2*i + j
				poleIDs[k] = row[j]
				p[k], e = coord(row[j])
				if e != nil {
					return nil, nil, nil, nil, e
				}
			}
		}
		b := mappedRationalBasis{Origin: p[0]}
		for i := 0; i < 3; i++ {
			b.X[i] = new(big.Rat).Sub(p[2][i], p[0][i])
			b.Y[i] = new(big.Rat).Sub(p[1][i], p[0][i])
		}
		if mappedRationalDot(b.X, b.Y).Sign() != 0 {
			return nil, nil, nil, nil, fail()
		}
		b.X, e = mappedRationalUnit(b.X)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		b.Y, e = mappedRationalUnit(b.Y)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		b.Z = mappedRationalCross(b.X, b.Y)
		if mappedRationalDot(b.Z, b.Z).Cmp(big.NewRat(1, 1)) != 0 {
			return nil, nil, nil, nil, fail()
		}
		closure, e := nauoClosure(f, ids, 16)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		evidence := ObliqueNURBSProductFrame{DefinitionID: product.DefinitionID, ProductID: product.ProductID, BodyID: body, FaceID: faceIDs[0], SurfaceID: sid, PoleIDs: poleIDs}
		for j := 0; j < 3; j++ {
			evidence.Origin[j] = b.Origin[j].RatString()
			for k, axis := range [][3]*big.Rat{b.X, b.Y, b.Z} {
				evidence.Basis[k][j] = axis[j].RatString()
			}
		}
		vertices[product.DefinitionID] = map[geom.Vec3]bool{}
		// Record original vertices before any points are changed.
		for _, id := range f.Order {
			if !closure[id] {
				continue
			}
			r, e := simple(f, id, "VERTEX_POINT", 2)
			if e != nil {
				continue
			}
			pid, e := ref(r.Args[1])
			if e != nil {
				return nil, nil, nil, nil, e
			}
			q, e := coord(pid)
			if e != nil {
				return nil, nil, nil, nil, e
			}
			v, e := obliqueNURBSFloatPoint(q)
			if e != nil {
				return nil, nil, nil, nil, e
			}
			vertices[product.DefinitionID][v] = true
		}
		for _, id := range f.Order {
			if !closure[id] {
				continue
			}
			if e := ctx.Err(); e != nil {
				return nil, nil, nil, nil, e
			}
			en := f.Entities[id]
			metadata := true
			for _, r := range en.Records {
				switch r.Type {
				case "GEOMETRIC_REPRESENTATION_CONTEXT", "PARAMETRIC_REPRESENTATION_CONTEXT", "REPRESENTATION_CONTEXT":
				default:
					metadata = false
				}
			}
			if !metadata {
				if _, exists := owned[id]; exists {
					return nil, nil, nil, nil, fail()
				}
				owned[id] = body
			}
			r, e := simple(f, id, "CARTESIAN_POINT", 2)
			if e != nil {
				continue
			}
			if r.Args[1].Kind != Aggregate {
				return nil, nil, nil, nil, fail()
			}
			if len(r.Args[1].Items) == 2 {
				continue
			}
			if len(r.Args[1].Items) != 3 {
				return nil, nil, nil, nil, fail()
			}
			for _, owner := range incoming[id] {
				if !closure[owner] {
					return nil, nil, nil, nil, fail()
				}
			}
			q, e := coord(id)
			if e != nil {
				return nil, nil, nil, nil, e
			}
			var delta, out [3]*big.Rat
			for j := 0; j < 3; j++ {
				delta[j] = new(big.Rat).Sub(q[j], b.Origin[j])
			}
			for j, axis := range [][3]*big.Rat{b.X, b.Y, b.Z} {
				out[j] = mappedRationalDot(delta, axis)
			}
			if _, e = obliqueNURBSFloatPoint(out); e != nil {
				return nil, nil, nil, nil, e
			}
			point := ObliqueNURBSPointEvidence{DefinitionID: product.DefinitionID, BodyID: body, PointID: id}
			for j := 0; j < 3; j++ {
				point.Source[j] = q[j].RatString()
				point.Proof[j] = out[j].RatString()
				text, e := extrusionDirectionDecimal(out[j])
				if e != nil {
					return nil, nil, nil, nil, e
				}
				v := Value{Kind: Number, Text: text}
				if _, e = wrappedIsoSourceNumber(v); e != nil {
					return nil, nil, nil, nil, e
				}
				r.Args[1].Items[j] = v
			}
			points = append(points, point)
		}
		bases[product.DefinitionID] = b
		frames = append(frames, evidence)
	}
	if len(frames) == 0 {
		return nil, nil, nil, nil, fail()
	}
	return frames, points, bases, vertices, nil
}
