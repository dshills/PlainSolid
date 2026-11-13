package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedToroidalBody contains a canonical tensor toroid with separately
// certified quotient embedding and outward orientation. Face retains source
// seam topology; Solid and Embedding are derived, not trusted writer inputs.
type WrappedToroidalBody struct {
	Name      string
	Face      WrappedSplineFace
	Solid     trimmed.Solid
	Embedding geom.WrappedToroidEmbedding
}
type WrappedToroidalModel struct {
	Bodies     []WrappedToroidalBody
	LengthUnit float64
}

// ReadWrappedToroidalBodies requires MANIFOLD_SOLID_BREP -> CLOSED_SHELL ->
// one positively oriented full double-seam face. The native constructive
// certificate additionally proves the restricted canonical toroid embedding.
func ReadWrappedToroidalBodies(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedToroidalModel, error) {
	return ReadWrappedToroidalBodiesContext(context.Background(), r, l, o)
}
func ReadWrappedToroidalBodiesContext(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedToroidalModel, error) {
	fail := func() (WrappedToroidalModel, error) {
		return WrappedToroidalModel{}, errors.New("step: unsupported wrapped toroidal solid graph")
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return WrappedToroidalModel{}, e
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedToroidalModel{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedToroidalModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedToroidalModel{}, e
	}
	view := *f
	view.Entities = make(map[int64]Entity, len(f.Entities))
	for id, ent := range f.Entities {
		view.Entities[id] = ent
	}
	names := map[int64]string{}
	shells := map[int64]bool{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			switch rec.Type {
			case "OPEN_SHELL", "SHELL_BASED_SURFACE_MODEL", "BREP_WITH_VOIDS", "FACETED_BREP", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION":
				return fail()
			}
		}
		if len(ent.Records) != 1 {
			continue
		}
		rec := ent.Records[0]
		if rec.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			rec.Type = "MANIFOLD_SURFACE_SHAPE_REPRESENTATION"
			view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
		}
		if rec.Type != "MANIFOLD_SOLID_BREP" {
			continue
		}
		body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return fail()
		}
		name, e := textValue(body.Args[0])
		if e != nil {
			return fail()
		}
		sid, e := ref(body.Args[1])
		if e != nil || shells[sid] {
			return fail()
		}
		shell, e := simple(f, sid, "CLOSED_SHELL", 2)
		if e != nil {
			return fail()
		}
		faces, e := listRefs(shell.Args[1], 1)
		if e != nil || len(faces) != 1 {
			return fail()
		}
		if _, exists := names[faces[0]]; exists {
			return fail()
		}
		names[faces[0]] = name
		shells[sid] = true
		shell.Type = "OPEN_SHELL"
		view.Entities[sid] = Entity{ID: sid, Records: []Record{shell}}
		body.Type = "SHELL_BASED_SURFACE_MODEL"
		body.Args = append([]Value(nil), body.Args...)
		body.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: sid}}}
		view.Entities[id] = Entity{ID: id, Records: []Record{body}}
	}
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "CLOSED_SHELL" && !shells[id] {
				return fail()
			}
		}
	}
	// Divide the requested budget between source graph proof and material proof.
	o.MaxOperations /= 2
	if o.MaxOperations < 1 {
		return WrappedToroidalModel{}, geom.ErrConvergence
	}
	m, e := readWrappedSplineFacesFile(ctx, &view, l, o)
	if e != nil {
		return WrappedToroidalModel{}, e
	}
	if len(m.Faces) != len(names) {
		return fail()
	}
	out := WrappedToroidalModel{LengthUnit: m.LengthUnit}
	budget := o.MaxOperations / len(m.Faces)
	if budget < 1 {
		return WrappedToroidalModel{}, geom.ErrConvergence
	}
	for i, face := range m.Faces {
		name, ok := names[m.FaceIDs[i]]
		if !ok {
			return fail()
		}
		solid, proof, e := trimmed.NewWrappedToroid(ctx, face.Geometry, 1e-10, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: budget, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return WrappedToroidalModel{}, e
		}
		if !proof.Complete || !proof.Outward {
			return fail()
		}
		out.Bodies = append(out.Bodies, WrappedToroidalBody{Name: name, Face: face, Solid: solid, Embedding: proof})
	}
	return out, nil
}

// WriteWrappedToroidalBodies treats names and certified face supports as input;
// regenerates seam IDs and proves material embedding before emitting any bytes.
func WriteWrappedToroidalBodies(w io.Writer, m WrappedToroidalModel) error {
	if w == nil || len(m.Bodies) == 0 || len(m.Bodies) > 64 {
		return geom.ErrInvalid
	}
	surfaces := WrappedSurfaceModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		surfaces.Surfaces = append(surfaces.Surfaces, WrappedSplineSurface{Name: b.Face.Name, Geometry: b.Face.Geometry})
	}
	var raw bytes.Buffer
	if e := WriteWrappedSplineFaces(&raw, surfaces); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	faces := []int64{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			faces = append(faces, id)
		}
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		roots := []int64{}
		for i, id := range faces {
			face := copySplineGeometry(emit, f, []int64{id})[0]
			shell := emit(fmt.Sprintf("CLOSED_SHELL('',(#%d))", face))
			roots = append(roots, emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Bodies[i].Name), shell)))
		}
		return roots
	})
	if e != nil {
		return e
	}
	if _, e = ReadWrappedToroidalBodies(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
