package document

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"strconv"
	"strings"
)

type RepairedNAUOInstance struct {
	ID, AssetID string
	Occurrence  step.ExactDAGNAUOOccurrence
}

// RepairedCircularNAUODocument keeps exact local planar assets and explicitly
// reconstructed circular assets shared by exact source occurrence paths. Repair
// evidence is returned separately, never trusted from the native Local payload.
type RepairedCircularNAUODocument struct {
	model          step.RepairedCircularNAUOAssembly
	source, native []byte
	local          State
	options        step.CircularNAUORepairOptions
}
type repairedCircularNAUOEnvelope struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	MaxDistance float64         `json:"maxDistance"`
	Source      string          `json:"source"`
	Local       json.RawMessage `json:"local"`
}

// ReadRepairedCircularNAUOSTEPWithEvidence requires explicit repair options and returns one
// source-bound displacement report per repaired local asset. Occurrences do not
// trigger independent repairs or duplicate native local geometry.
func ReadRepairedCircularNAUOSTEPWithEvidence(r io.Reader, l step.Limits, o step.CircularNAUORepairOptions) (*RepairedCircularNAUODocument, *RepairedNAUOEvidence, error) {
	if r == nil {
		return nil, nil, rationalMappedDocumentError()
	}
	var original bytes.Buffer
	a, e := step.ReadRepairedCircularNAUOAssembly(io.TeeReader(r, &original), l, o)
	if e != nil {
		return nil, nil, e
	}
	s := State{LengthUnit: a.LengthUnit, Planar: map[string]brep.Solid{}, Parametric: map[string]parametric.Solid{}, Labels: map[string]Label{}}
	for _, p := range a.Parts {
		key := "product-" + strconv.FormatInt(p.Product.DefinitionID, 10)
		switch p.Family {
		case step.RepairedNAUOExactPlanar:
			s.Planar[key] = p.Planar.Solid
		case step.RepairedNAUOCircular, step.RepairedNAUOTube:
			s.Parametric[key] = p.Circular
		default:
			return nil, nil, rationalMappedDocumentError()
		}
		s.Labels["source:"+key] = Label{Target: AssetLabel, TargetID: key, Name: p.Product.Name, Attributes: map[string]Attribute{"step.source.product_definition": {Kind: "integer", Integer: p.Product.DefinitionID}, "step.source.body": {Kind: "integer", Integer: p.SourceBodyID}, "step.geometry.family": {Kind: "string", Text: string(p.Family)}}}
	}
	local, e := New(s, 0)
	if e != nil {
		return nil, nil, e
	}
	var native bytes.Buffer
	if e = local.Write(&native); e != nil {
		return nil, nil, e
	}
	state, _ := local.Snapshot()
	reports := &RepairedNAUOEvidence{Cylinders: a.Repairs, Tubes: a.TubeRepairs}
	a.TubeRepairs = nil
	a.Repairs = nil // evidence is deliberately outside the stored native model
	return &RepairedCircularNAUODocument{model: a, source: append([]byte(nil), original.Bytes()...), native: native.Bytes(), local: state, options: o}, reports, nil
}
func (d *RepairedCircularNAUODocument) Name() string {
	if d == nil {
		return ""
	}
	return d.model.Root.Name
}
func (d *RepairedCircularNAUODocument) LengthUnit() float64 {
	if d == nil {
		return 0
	}
	return d.model.LengthUnit
}
func (d *RepairedCircularNAUODocument) Root() step.AssemblyProduct {
	if d == nil {
		return step.AssemblyProduct{}
	}
	p := d.model.Root
	p.Representations = append([]int64(nil), p.Representations...)
	return p
}
func (d *RepairedCircularNAUODocument) Groups() []step.AssemblyProduct {
	if d == nil {
		return nil
	}
	p := append([]step.AssemblyProduct(nil), d.model.Groups...)
	for i := range p {
		p[i].Representations = append([]int64(nil), p[i].Representations...)
	}
	return p
}
func (d *RepairedCircularNAUODocument) Parts() []step.RepairedNAUOProduct {
	if d == nil {
		return nil
	}
	p := append([]step.RepairedNAUOProduct(nil), d.model.Parts...)
	for i := range p {
		p[i].Product.Representations = append([]int64(nil), p[i].Product.Representations...)
	}
	return p
}
func (d *RepairedCircularNAUODocument) Relationships() []step.ExactNAUOOccurrence {
	if d == nil {
		return nil
	}
	return append([]step.ExactNAUOOccurrence(nil), d.model.Relationships...)
}

// LocalAssets contains no approximate floating occurrence nodes. Instances retain
// exact poses separately; the returned native asset maps and labels are copied.
func (d *RepairedCircularNAUODocument) LocalAssets() State {
	if d == nil {
		return State{}
	}
	return clone(d.local)
}
func (d *RepairedCircularNAUODocument) Instances() []RepairedNAUOInstance {
	if d == nil {
		return nil
	}
	out := make([]RepairedNAUOInstance, len(d.model.Occurrences))
	for i, u := range d.model.Occurrences {
		ids := make([]string, len(u.Path))
		for j, p := range u.Path {
			ids[j] = strconv.FormatInt(p.NAUOID, 10)
		}
		u.Path = append([]step.ExactNAUOOccurrence(nil), u.Path...)
		out[i] = RepairedNAUOInstance{ID: "nauo-" + strings.Join(ids, "/"), AssetID: "product-" + strconv.FormatInt(u.Path[len(u.Path)-1].ChildID, 10), Occurrence: u}
	}
	return out
}

// ExactWorldBounds returns rational extrema when every circular projected radius
// has an exact rational square root. Otherwise it fails without rounding; exact
// placements and local assets remain available for separate bounded queries.
func (d *RepairedCircularNAUODocument) ExactWorldBounds() (b RationalWorldBounds, e error) {
	if d == nil {
		return b, rationalMappedDocumentError()
	}
	first := true
	grow := func(lo, hi [3]*big.Rat) {
		for i := range lo {
			if first || lo[i].Cmp(b.Min[i]) < 0 {
				b.Min[i] = new(big.Rat).Set(lo[i])
			}
			if first || hi[i].Cmp(b.Max[i]) > 0 {
				b.Max[i] = new(big.Rat).Set(hi[i])
			}
		}
		first = false
	}
	sqrt := func(q *big.Rat) (*big.Rat, error) {
		n, z := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
		if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(z, z).Cmp(q.Denom()) != 0 {
			return nil, rationalMappedDocumentError()
		}
		return new(big.Rat).SetFrac(n, z), nil
	}
	for _, u := range d.model.Occurrences {
		var part step.RepairedNAUOProduct
		for _, p := range d.model.Parts {
			if p.Product.DefinitionID == u.Path[len(u.Path)-1].ChildID {
				part = p
			}
		}
		if part.Family == step.RepairedNAUOExactPlanar {
			for _, p := range part.Planar.Solid.Vertices() {
				q, e := u.ApplyExact(p)
				if e != nil {
					return RationalWorldBounds{}, e
				}
				grow(q, q)
			}
			continue
		}
		if part.Family == step.RepairedNAUOTube {
			part.Cylinder = step.CylinderBody{Name: part.Tube.Name, Frame: part.Tube.Frame, Radius: part.Tube.OuterRadius, Height: part.Tube.Height}
		}
		if part.Family != step.RepairedNAUOCircular && part.Family != step.RepairedNAUOTube {
			return RationalWorldBounds{}, rationalMappedDocumentError()
		}
		c, e := u.ApplyExact(part.Cylinder.Frame.Origin())
		if e != nil {
			return RationalWorldBounds{}, e
		}
		m := u.Matrix()
		direction := func(v geom.Vec3) (a [3]*big.Rat) {
			values := []float64{v.X, v.Y, v.Z}
			for i := range a {
				a[i] = new(big.Rat)
				for j, x := range values {
					a[i].Add(a[i], new(big.Rat).Mul(m[i][j], new(big.Rat).SetFloat64(x)))
				}
			}
			return
		}
		x, y, z := direction(part.Cylinder.Frame.XAxis()), direction(part.Cylinder.Frame.YAxis()), direction(part.Cylinder.Frame.ZAxis())
		radius, height := new(big.Rat).SetFloat64(part.Cylinder.Radius), new(big.Rat).SetFloat64(part.Cylinder.Height)
		var lo, hi [3]*big.Rat
		for i := range c {
			radial, e := sqrt(new(big.Rat).Add(new(big.Rat).Mul(x[i], x[i]), new(big.Rat).Mul(y[i], y[i])))
			if e != nil {
				return RationalWorldBounds{}, e
			}
			radial.Mul(radial, radius)
			end := new(big.Rat).Add(c[i], new(big.Rat).Mul(height, z[i]))
			a, b := c[i], end
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			lo[i] = new(big.Rat).Sub(a, radial)
			hi[i] = new(big.Rat).Add(b, radial)
		}
		grow(lo, hi)
	}
	if first {
		return RationalWorldBounds{}, rationalMappedDocumentError()
	}
	return b, nil
}
func (d *RepairedCircularNAUODocument) Write(w io.Writer) error {
	if d == nil || w == nil || len(d.source) == 0 {
		return rationalMappedDocumentError()
	}
	raw, e := json.Marshal(repairedCircularNAUOEnvelope{Format: "plainsolid-repaired-circular-nauo", Version: d.nativeVersion(), MaxDistance: d.options.MaxDistance, Source: string(d.source), Local: d.native})
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// WriteOriginalSTEP emits retained original source, including its measured
// defects. It never labels that source as the newly reconstructed native shape.
func (d *RepairedCircularNAUODocument) WriteOriginalSTEP(w io.Writer) error {
	if d == nil || w == nil || len(d.source) == 0 {
		return rationalMappedDocumentError()
	}
	n, e := w.Write(d.source)
	if e == nil && n != len(d.source) {
		return io.ErrShortWrite
	}
	return e
}

// ReadRepairedCircularNAUODocumentWithEvidence requires the same caller repair tolerance as
// the saved declaration, re-proves all source/asset bindings, and returns fresh
// repair evidence separately. Serialized reports are neither accepted nor used.
func ReadRepairedCircularNAUODocumentWithEvidence(r io.Reader, maxBytes int64, l step.Limits, o step.CircularNAUORepairOptions) (*RepairedCircularNAUODocument, *RepairedNAUOEvidence, error) {
	if r == nil || maxBytes <= 0 || maxBytes > 1<<30 {
		return nil, nil, rationalMappedDocumentError()
	}
	raw, e := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if e != nil {
		return nil, nil, e
	}
	if int64(len(raw)) > maxBytes {
		return nil, nil, rationalMappedDocumentError()
	}
	var env repairedCircularNAUOEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&env); e != nil {
		return nil, nil, e
	}
	var extra any
	if e = dec.Decode(&extra); e != io.EOF || env.Format != "plainsolid-repaired-circular-nauo" || (env.Version != 1 && env.Version != 2) || env.MaxDistance != o.MaxDistance {
		return nil, nil, rationalMappedDocumentError()
	}
	fresh, reports, e := ReadRepairedCircularNAUOSTEPWithEvidence(bytes.NewReader([]byte(env.Source)), l, o)
	if e != nil {
		return nil, nil, e
	}
	if env.Version != fresh.nativeVersion() {
		return nil, nil, rationalMappedDocumentError()
	}
	local, e := Read(bytes.NewReader(env.Local), maxBytes, 0)
	if e != nil {
		return nil, nil, e
	}
	var canonical bytes.Buffer
	if e = local.Write(&canonical); e != nil {
		return nil, nil, e
	}
	if !bytes.Equal(canonical.Bytes(), fresh.native) {
		return nil, nil, rationalMappedDocumentError()
	}
	return fresh, reports, nil
}

// RepairedNAUOEvidence keeps independently reconstructed local families separate.
// Tables contain one entry per product, never per occurrence; native reload
// regenerates them rather than accepting serialized evidence.
type RepairedNAUOEvidence struct {
	Cylinders []step.RepairedCircularNAUO
	Tubes     []step.RepairedCircularTubeNAUO
}

func (d *RepairedCircularNAUODocument) nativeVersion() int {
	for _, p := range d.model.Parts {
		if p.Family == step.RepairedNAUOTube {
			return 2
		}
	}
	return 1
}

// ReadRepairedCircularNAUOSTEP retains the cylinder-only reporting contract.
// Tube-bearing sources require the explicit WithEvidence API.
func ReadRepairedCircularNAUOSTEP(r io.Reader, l step.Limits, o step.CircularNAUORepairOptions) (*RepairedCircularNAUODocument, []step.RepairedCircularNAUO, error) {
	d, evidence, e := ReadRepairedCircularNAUOSTEPWithEvidence(r, l, o)
	if e != nil {
		return nil, nil, e
	}
	if len(evidence.Tubes) != 0 {
		return nil, nil, rationalMappedDocumentError()
	}
	return d, evidence.Cylinders, nil
}

// ReadRepairedCircularNAUODocument retains version-1 cylinder-only evidence.
func ReadRepairedCircularNAUODocument(r io.Reader, maxBytes int64, l step.Limits, o step.CircularNAUORepairOptions) (*RepairedCircularNAUODocument, []step.RepairedCircularNAUO, error) {
	d, evidence, e := ReadRepairedCircularNAUODocumentWithEvidence(r, maxBytes, l, o)
	if e != nil {
		return nil, nil, e
	}
	if len(evidence.Tubes) != 0 {
		return nil, nil, rationalMappedDocumentError()
	}
	return d, evidence.Cylinders, nil
}
