package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedToroidalVoid contains two outward canonical circular-profile toroids.
// STEP reverses only the inner shell occurrence. General nested spline solids
// and touching/overlapping toroidal supports are outside this adapter.
type WrappedToroidalVoid struct {
	Name         string
	Outer, Inner WrappedSplineFace
	Solid        trimmed.Solid
	LengthUnit   float64
}

func ReadWrappedToroidalVoid(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedToroidalVoid, error) {
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	if l.MaxFaces < 2 || l.MaxEdgeUses < 8 {
		return WrappedToroidalVoid{}, geom.ErrInvalid
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	name, e := splitWrappedVoid(f, 1, false, o)
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	internal := l
	internal.MaxSolids = max(2, l.MaxSolids)
	m, e := ReadWrappedToroidalBodies(bytes.NewReader(serializeWrappedProofFile(f)), internal, o)
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	if len(m.Bodies) != 2 {
		return WrappedToroidalVoid{}, geom.ErrInvalid
	}
	a, b := m.Bodies[0], m.Bodies[1]
	if !a.Embedding.CircularProfiles || !b.Embedding.CircularProfiles {
		return WrappedToroidalVoid{}, geom.ErrConvergence
	}
	for k := 0; k < 3; k++ {
		if a.Embedding.Center[k].Cmp(b.Embedding.Center[k]) != 0 {
			return WrappedToroidalVoid{}, geom.ErrConvergence
		}
	}
	if a.Embedding.MajorRadius.Cmp(b.Embedding.MajorRadius) != 0 || a.Embedding.RadialMinorRadius.Cmp(b.Embedding.RadialMinorRadius) <= 0 || a.Embedding.AxialMinorRadius.Cmp(b.Embedding.AxialMinorRadius) <= 0 {
		return WrappedToroidalVoid{}, geom.ErrConvergence
	}
	solid, _, e := trimmed.CombineNestedSplineComponents(context.Background(), a.Solid, b.Solid, parametric.Difference, trimmed.SplineShellBooleanOptions{Embedding: trimmed.SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}}})
	if e != nil {
		return WrappedToroidalVoid{}, e
	}
	if len(solid.Topology().Shells) != 2 {
		return WrappedToroidalVoid{}, geom.ErrConvergence
	}
	return WrappedToroidalVoid{Name: name, Outer: a.Face, Inner: b.Face, Solid: solid, LengthUnit: m.LengthUnit}, nil
}
func WriteWrappedToroidalVoid(w io.Writer, m WrappedToroidalVoid) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var raw bytes.Buffer
	pair := WrappedToroidalModel{LengthUnit: m.LengthUnit, Bodies: []WrappedToroidalBody{{Name: m.Name, Face: m.Outer}, {Name: m.Name, Face: m.Inner}}}
	if e := WriteWrappedToroidalBodies(&raw, pair); e != nil {
		return e
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		return e
	}
	out, e := joinWrappedVoid(f)
	if e != nil {
		return e
	}
	if _, e = ReadWrappedToroidalVoid(bytes.NewReader(out), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}
