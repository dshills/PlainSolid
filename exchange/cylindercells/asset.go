// Package cylindercells persists one exact rooted cylinder dialect without
// changing generic STEP or native BRep schemas.
package cylindercells

import (
	"bytes"
	"errors"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const (
	Half         = "half"
	Quarter      = "quarter"
	MaxSTEPBytes = 1 << 20
)

var ErrAsset = errors.New("cylindercells: unsupported or mismatched exact asset")

// SIUnit is exactly 10^MetresExponent metres per model length unit. Unlike a
// float scale this tag preserves the exact SI prefix meaning. Zero is metre.
type SIUnit struct{ MetresExponent int }

func (u SIUnit) scale() (float64, error) {
	for _, p := range []int{-18, -15, -12, -9, -6, -3, -2, -1, 0, 1, 2, 3, 6, 9, 12, 15, 18} {
		if p == u.MetresExponent {
			return math.Pow10(p), nil
		}
	}
	return 0, ErrAsset
}
func unit(v float64) (SIUnit, error) {
	for _, p := range []int{-18, -15, -12, -9, -6, -3, -2, -1, 0, 1, 2, 3, 6, 9, 12, 15, 18} {
		if v == math.Pow10(p) {
			return SIUnit{p}, nil
		}
	}
	return SIUnit{}, ErrAsset
}

// Asset is a neutral geometry/metadata descriptor. SourceSTEP, when nonempty,
// is original rooted evidence and must agree with every descriptor field.
// It is bounded and re-proved whenever geometry or export is requested.
type Asset struct {
	Dialect, Name           string
	Unit                    SIUnit
	Origin, Reference, Axis geom.Vec3
	Radius, Height          float64
	SourceSTEP              string `json:",omitempty"`
}

// Provenance contains original STEP IDs, never native chart indices.
type Provenance struct {
	Body, Shell, Product, Definition, ShapeDefinition int64
	Faces, Edges                                      []int64
}
type Result struct {
	Solid  parametric.Solid
	Source *Provenance
}

func descriptor(a Asset) (step.SplitCylinderDescriptor, error) {
	scale, e := a.Unit.scale()
	if e != nil {
		return step.SplitCylinderDescriptor{}, e
	}
	if a.Dialect != Half && a.Dialect != Quarter {
		return step.SplitCylinderDescriptor{}, ErrAsset
	}
	frame, e := geom.NewFrame(a.Origin, a.Reference, a.Axis)
	if e != nil || frame.Origin() != a.Origin || frame.XAxis() != a.Reference || frame.ZAxis() != a.Axis {
		return step.SplitCylinderDescriptor{}, ErrAsset
	}
	return step.SplitCylinderDescriptor{Name: a.Name, Frame: frame, Radius: a.Radius, Height: a.Height, LengthUnit: scale}, nil
}
func write(w io.Writer, a Asset, d step.SplitCylinderDescriptor) error {
	if a.Dialect == Half {
		return step.WriteSplitCylinderProduct(w, d)
	}
	if a.Dialect == Quarter {
		return step.WriteQuarterCylinderProduct(w, d)
	}
	return ErrAsset
}
func decode(raw []byte, dialect string) (Asset, Result, error) {
	if len(raw) > MaxSTEPBytes {
		return Asset{}, Result{}, ErrAsset
	}
	var d step.SplitCylinderDescriptor
	var result Result
	var p Provenance
	switch dialect {
	case Half:
		m, e := step.ReadSplitCylinderProduct(bytes.NewReader(raw), step.Limits{})
		if e != nil {
			return Asset{}, Result{}, e
		}
		d = step.SplitCylinderDescriptor{Name: m.Model.Name, Frame: m.Model.Frame, Radius: m.Model.Radius, Height: m.Model.Height, LengthUnit: m.Model.LengthUnit}
		result.Solid = m.Model.Solid
		p = Provenance{Body: m.Model.Body, Shell: m.Model.Shell, Product: m.Product, Definition: m.Definition, ShapeDefinition: m.ShapeDefinition, Faces: append([]int64(nil), m.Model.Faces[:]...), Edges: append([]int64(nil), m.Model.Edges[:]...)}
	case Quarter:
		m, e := step.ReadQuarterCylinderProduct(bytes.NewReader(raw), step.Limits{})
		if e != nil {
			return Asset{}, Result{}, e
		}
		d = step.SplitCylinderDescriptor{Name: m.Model.Name, Frame: m.Model.Frame, Radius: m.Model.Radius, Height: m.Model.Height, LengthUnit: m.Model.LengthUnit}
		result.Solid = m.Model.Solid
		p = Provenance{Body: m.Model.Body, Shell: m.Model.Shell, Product: m.Product, Definition: m.Definition, ShapeDefinition: m.ShapeDefinition, Faces: append([]int64(nil), m.Model.Faces[:]...), Edges: append([]int64(nil), m.Model.Edges[:]...)}
	default:
		return Asset{}, Result{}, ErrAsset
	}
	u, e := unit(d.LengthUnit)
	if e != nil {
		return Asset{}, Result{}, e
	}
	result.Source = &p
	return Asset{Dialect: dialect, Name: d.Name, Unit: u, Origin: d.Frame.Origin(), Reference: d.Frame.XAxis(), Axis: d.Frame.ZAxis(), Radius: d.Radius, Height: d.Height, SourceSTEP: string(raw)}, result, nil
}

// FromSTEP retains the exact original bytes as provenance. The dialect is
// explicit; product tags and topology must both agree with it.
func FromSTEP(r io.Reader, dialect string) (Asset, error) {
	if r == nil {
		return Asset{}, ErrAsset
	}
	b, e := io.ReadAll(io.LimitReader(r, MaxSTEPBytes+1))
	if e != nil {
		return Asset{}, e
	}
	a, _, e := decode(b, dialect)
	return a, e
}

// FromNative admits only the existing immutable canonical native cylinder
// classifier. It produces no source provenance or fabricated source IDs.
func FromNative(s parametric.Solid, name string, u SIUnit, dialect string) (Asset, error) {
	scale, e := u.scale()
	if e != nil {
		return Asset{}, e
	}
	d, e := step.NativeSplitCylinderDescriptor(s, name, scale)
	if e != nil {
		return Asset{}, e
	}
	a := Asset{Dialect: dialect, Name: name, Unit: u, Origin: d.Frame.Origin(), Reference: d.Frame.XAxis(), Axis: d.Frame.ZAxis(), Radius: d.Radius, Height: d.Height}
	if _, e = a.Validate(); e != nil {
		return Asset{}, e
	}
	return a, nil
}

// Validate proves geometry and any retained source binding. Returned source ID
// slices are newly owned. Native-only assets have Source=nil in the result.
func (a Asset) Validate() (Result, error) {
	d, e := descriptor(a)
	if e != nil {
		return Result{}, e
	}
	if a.SourceSTEP != "" {
		original, result, e := decode([]byte(a.SourceSTEP), a.Dialect)
		if e != nil {
			return Result{}, e
		}
		if original != a {
			return Result{}, ErrAsset
		}
		return result, nil
	}
	var b bytes.Buffer
	if e = write(&b, a, d); e != nil {
		return Result{}, e
	}
	_, result, e := decode(b.Bytes(), a.Dialect)
	if e != nil {
		return Result{}, e
	}
	result.Source = nil
	return result, nil
}

// WriteSTEP retains its original canonical-export behavior. Use
// WriteOriginalSTEP for exact re-emission of retained evidence.
func WriteSTEP(w io.Writer, a Asset) error { return WriteCanonicalSTEP(w, a) }

// WriteCanonicalSTEP emits canonical rooted geometry after proof. Original
// source orientation flags may be normalized; SourceSTEP remains unmodified.
func WriteCanonicalSTEP(w io.Writer, a Asset) error {
	if w == nil {
		return ErrAsset
	}
	if _, e := a.Validate(); e != nil {
		return e
	}
	d, e := descriptor(a)
	if e != nil {
		return e
	}
	return write(w, a, d)
}

// ErrNoSource means the asset is native-only and has no authored source to emit.
var ErrNoSource = errors.New("cylindercells: no retained source provenance")

// WriteOriginalSTEP reproduces validated original rooted STEP bytes exactly,
// including its header, whitespace and equivalent orientation encodings. It
// never synthesizes missing evidence or strips/adds an envelope. This asset
// schema accepts rooted sources only; bare STEP is not an implicit fallback.
// Validation failures write nothing; underlying I/O failures are propagated.
func WriteOriginalSTEP(w io.Writer, a Asset) error {
	if w == nil {
		return ErrAsset
	}
	if a.SourceSTEP == "" {
		return ErrNoSource
	}
	if _, e := a.Validate(); e != nil {
		return e
	}
	n, e := io.WriteString(w, a.SourceSTEP)
	if e == nil && n != len(a.SourceSTEP) {
		e = io.ErrShortWrite
	}
	return e
}
