package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/geom"
)

// WrappedSplineFaceModel contains independent one-face OPEN_SHELL surface
// models. Source topology IDs remain local to the parsed STEP file.
type WrappedSplineFaceModel struct {
	Faces      []WrappedSplineFace
	FaceIDs    []int64
	LengthUnit float64
}

// ReadWrappedSplineFaces accepts standalone manifold surface representations,
// each containing independent one-face open shells. It rejects assemblies,
// solid shells, shared supports and unowned faces. It does not sew faces.
func ReadWrappedSplineFaces(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedSplineFaceModel, error) {
	return ReadWrappedSplineFacesContext(context.Background(), r, l, o)
}

// ReadWrappedSplineFacesContext divides the arithmetic budget equally among
// source faces, bounding aggregate certification work as well as input size.
func ReadWrappedSplineFacesContext(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedSplineFaceModel, error) {
	fail := func() (WrappedSplineFaceModel, error) {
		return WrappedSplineFaceModel{}, errors.New("step: unsupported wrapped surface model")
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return WrappedSplineFaceModel{}, e
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedSplineFaceModel{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedSplineFaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedSplineFaceModel{}, e
	}
	return readWrappedSplineFacesFile(ctx, f, l, o)
}

func readWrappedSplineFacesFile(ctx context.Context, f *File, l Limits, o WrappedSurfaceOptions) (WrappedSplineFaceModel, error) {
	fail := func() (WrappedSplineFaceModel, error) {
		return WrappedSplineFaceModel{}, errors.New("step: unsupported wrapped surface model")
	}
	count := 0
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "ADVANCED_FACE" {
				count++
			}
			if rec.Type == "CLOSED_SHELL" || rec.Type == "MANIFOLD_SOLID_BREP" || rec.Type == "BREP_WITH_VOIDS" {
				return fail()
			}
		}
	}
	if count < 1 || count > o.MaxSurfaces {
		return fail()
	}
	o.MaxOperations /= count
	if o.MaxOperations < 1 {
		return WrappedSplineFaceModel{}, geom.ErrConvergence
	}
	result := WrappedSplineFaceModel{}
	b, e := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		local := o
		local.MaxPoles = min(local.MaxPoles, maxPoints)
		if local.MaxPoles < 4 {
			return BoundedSplineFace{}, geom.ErrConvergence
		}
		face, e := DecodeWrappedSplineFace(ctx, f, id, local)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		recipe, e := face.Geometry.ExportRecipe()
		if e != nil {
			return BoundedSplineFace{}, e
		}
		result.Faces = append(result.Faces, face)
		result.FaceIDs = append(result.FaceIDs, id)
		return BoundedSplineFace{Name: face.Name, Geometry: recipe.Surface, Domain: face.Geometry.Domain()}, nil
	}, 4)
	if e != nil {
		return WrappedSplineFaceModel{}, e
	}
	result.LengthUnit = b.LengthUnit
	return result, nil
}
