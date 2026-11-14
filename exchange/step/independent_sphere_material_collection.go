package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// IndependentSphereMaterial retains the original natural-sphere pole topology.
// Native rectangular analytic sphere patches canonicalize the physical seam.
// Their trigonometric evaluation is not claimed to be exact rational sewing.
type IndependentSphereMaterial struct {
	Frame                                                      geom.Frame
	Radius                                                     float64
	SourceBodyID, SourceShellID, SourceFaceID, SourceSurfaceID int64
	SourceBoundID, SourceLoopID, SourceVertexID, SourcePointID int64
	NativeFaceSourceIDs                                        []int64
}

// ReadIndependentSphereMaterials admits full one-pole spheres alongside the
// independently proven planar, elliptical and model-only cylinder products.
// It requires at least one sphere and does not infer assembly occurrences.
func ReadIndependentSphereMaterials(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentExactMaterialCollection, error) {
	m, e := readIndependentExactMaterialCollection(ctx, r, l, tol, 1, 16, true, true, true)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	for _, p := range m.Members {
		if p.Sphere != nil {
			return m, nil
		}
	}
	return IndependentExactMaterialCollection{}, geom.ErrInvalid
}
func WriteIndependentSphereMaterials(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeIndependentExactMaterialCollection(ctx, &staged, sources, l, tol, 1, 16, true, true); e != nil {
		return e
	}
	if _, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(staged.Bytes()), l, tol); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func readIndependentSphereProduct(ctx context.Context, f *File, bodyID int64, l Limits, tol float64) (IndependentExactMaterial, error) {
	bad := func() (IndependentExactMaterial, error) { return IndependentExactMaterial{}, geom.ErrInvalid }
	// A closed natural sphere has exactly this dependency graph, independently
	// of IDs, root metadata and record ordering. Additional trim owners fail.
	keep, e := nauoClosure(f, []int64{bodyID}, l.MaxDepth)
	if e != nil || len(keep) != 12 {
		return bad()
	}
	counts := map[string]int{}
	for id := range keep {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			return bad()
		}
		counts[en.Records[0].Type]++
	}
	expected := map[string]int{"MANIFOLD_SOLID_BREP": 1, "CLOSED_SHELL": 1, "ADVANCED_FACE": 1, "FACE_BOUND": 1, "VERTEX_LOOP": 1, "VERTEX_POINT": 1, "CARTESIAN_POINT": 2, "DIRECTION": 2, "AXIS2_PLACEMENT_3D": 1, "SPHERICAL_SURFACE": 1}
	if len(counts) != len(expected) {
		return bad()
	}
	for k, n := range expected {
		if counts[k] != n {
			return bad()
		}
	}
	get := func(kind string) int64 {
		for id := range keep {
			if f.Entities[id].Records[0].Type == kind {
				return id
			}
		}
		return 0
	}
	sr, e := simple(f, get("SPHERICAL_SURFACE"), "SPHERICAL_SURFACE", 3)
	if e != nil {
		return bad()
	}
	radius, e := cylinderSupportNumber(sr.Args[2])
	if e != nil || radius <= 0 {
		return bad()
	}
	pid, e := ref(sr.Args[1])
	if e != nil {
		return bad()
	}
	p, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return bad()
	}
	v := [3]geom.Vec3{}
	for i, k := range []string{"CARTESIAN_POINT", "DIRECTION", "DIRECTION"} {
		id, e := ref(p.Args[i+1])
		if e != nil {
			return bad()
		}
		v[i], e = splitTriple(f, id, k)
		if e != nil {
			return bad()
		}
	}
	if !cylinderSupportUnit(v[1]) || !cylinderSupportUnit(v[2]) || v[1].Dot(v[2]) != 0 {
		return bad()
	}
	frame, e := geom.NewFrame(v[0], v[2], v[1])
	if e != nil {
		return bad()
	}
	vertex, e := simple(f, get("VERTEX_POINT"), "VERTEX_POINT", 2)
	if e != nil {
		return bad()
	}
	pointID, e := ref(vertex.Args[1])
	if e != nil {
		return bad()
	}
	point, e := splitTriple(f, pointID, "CARTESIAN_POINT")
	if e != nil {
		return bad()
	}
	// Check the authored pole using rational subtraction, never float proximity.
	a, b, z := [3]float64{point.X, point.Y, point.Z}, [3]float64{v[0].X, v[0].Y, v[0].Z}, [3]float64{v[1].X, v[1].Y, v[1].Z}
	valid := false
	for _, sign := range []int64{-1, 1} {
		ok := true
		for i := 0; i < 3; i++ {
			d := new(big.Rat).Sub(new(big.Rat).SetFloat64(a[i]), new(big.Rat).SetFloat64(b[i]))
			want := new(big.Rat).Mul(new(big.Rat).SetFloat64(radius), new(big.Rat).SetFloat64(z[i]))
			want.Mul(want, big.NewRat(sign, 1))
			if d.Cmp(want) != 0 {
				ok = false
			}
		}
		valid = valid || ok
	}
	if !valid {
		return bad()
	}
	proof, e := analyticBody(f, bodyID)
	if e != nil {
		return bad()
	}
	solid, e := trimmed.FromParametric(proof.Solid)
	if e != nil {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterial{}, e
	}
	// Obtain units through the existing strict representation context checker.
	unit := 0.0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			cid, e := ref(en.Records[0].Args[2])
			if e != nil {
				return bad()
			}
			unit, e = contextUnits(f, cid)
			if e != nil {
				return bad()
			}
		}
	}
	if unit <= 0 {
		return bad()
	}
	q := IndependentSphereMaterial{frame, radius, bodyID, get("CLOSED_SHELL"), get("ADVANCED_FACE"), get("SPHERICAL_SURFACE"), get("FACE_BOUND"), get("VERTEX_LOOP"), get("VERTEX_POINT"), pointID, nil}
	for range solid.Faces() {
		q.NativeFaceSourceIDs = append(q.NativeFaceSourceIDs, q.SourceFaceID)
	}
	return IndependentExactMaterial{Name: proof.Name, LengthUnit: unit, Source: f, Solid: solid, Sphere: &q, SourceBodyID: bodyID}, nil
}
