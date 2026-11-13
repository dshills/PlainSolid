package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// JSONLimits bounds bytes, topology objects and individual spline nets.
// MaxEntities counts vertices, edges, faces, and all uses.
type JSONLimits struct {
	MaxBytes                      int64
	MaxEntities, MaxControlPoints int
}
type nativeEdge struct {
	Start, End int
	Range      parametric.Interval
	Curve      json.RawMessage
	Surface    json.RawMessage `json:",omitempty"`
	P          *nativeUse      `json:",omitempty"`
}
type nativeUse struct {
	Edge           int
	Reversed       bool
	UVStart, UVEnd geom.Vec2
	Quadratic      *QuadraticUV       `json:",omitempty"`
	ExactQuadratic *[3][3]string      `json:",omitempty"`
	RationalEnds   bool               `json:",omitempty"`
	Ellipse        *EllipseUV         `json:",omitempty"`
	AnchoredCircle bool               `json:",omitempty"`
	Implicit       *nativeImplicitUse `json:",omitempty"`
	Tube           *nativeTubeUse     `json:",omitempty"`
	Wall           *nativeWallTrim    `json:",omitempty"`
	Spline         json.RawMessage    `json:",omitempty"`
	SplineRange    UVInterval         `json:",omitempty"`
	SplineReversed bool               `json:",omitempty"`
	Subcurve       *nativeUse         `json:",omitempty"`
	SubRange       UVInterval         `json:",omitempty"`
	Flip           bool               `json:",omitempty"`
}
type nativeImplicitUse struct {
	Recipe, Contour, Segment int
	OnPlane                  bool
}
type nativeImplicitRecipe struct {
	Surface      json.RawMessage
	Second       json.RawMessage `json:",omitempty"`
	Origin, X, Z geom.Vec3
	Options      intersect.SplinePlaneOptions
}
type nativeWallTrim struct {
	Surface                    json.RawMessage
	A, B                       geom.Vec2
	Length, Height, Start, End float64
}
type nativeFace struct {
	Surface  json.RawMessage
	Outer    []nativeUse
	Holes    [][]nativeUse `json:",omitempty"`
	Reversed bool
}
type nativeSolid struct {
	Version   int                    `json:"version"`
	Kind      string                 `json:"kind"`
	Tolerance float64                `json:"tolerance"`
	Vertices  []geom.Vec3            `json:"vertices"`
	Edges     []nativeEdge           `json:"edges"`
	Faces     []nativeFace           `json:"faces"`
	Implicit  []nativeImplicitRecipe `json:"implicit,omitempty"`
	Tubes     []nativeTubeRecipe     `json:"tubes,omitempty"`
}

type implicitRecipeKey struct {
	surface, second *geom.BSplineSurface
	frame           geom.Frame
	options         intersect.SplinePlaneOptions
}
type trimEncoder struct {
	keys           map[implicitRecipeKey]int
	recipes        []nativeImplicitRecipe
	tubeIDs        map[intersect.TubeRecipe]int
	tubes          []nativeTubeRecipe
	exactQuadratic bool
	rationalEnds   bool
}

func (t *trimEncoder) encodeUses(uses []Use) ([]nativeUse, error) {
	r := make([]nativeUse, len(uses))
	for i, u := range uses {
		x := nativeUse{Edge: u.Edge, Reversed: u.Reversed, UVStart: u.UVStart, UVEnd: u.UVEnd}
		switch c := u.Curve.(type) {
		case nil:
		case QuadraticUV:
			x.Quadratic = &c
		case ExactQuadraticUV:
			if c.control[0][0] == nil {
				return nil, geom.ErrInvalid
			}
			var net [3][3]string
			for j, p := range c.control {
				for k, n := range p {
					net[j][k] = n.RatString()
				}
			}
			x.ExactQuadratic = &net
			x.RationalEnds = c.rationalEndpoints
			t.exactQuadratic = true
			t.rationalEnds = t.rationalEnds || c.rationalEndpoints
		case EllipseUV:
			x.Ellipse = &c
		case anchoredCircularUV:
			x.Ellipse, x.AnchoredCircle = &c.EllipseUV, true
			x.UVStart, x.UVEnd = c.Start, c.End
		case intersect.CertifiedTrim:
			a := c.Data()
			key := implicitRecipeKey{a.Surface, a.Second, a.Plane, a.Options}
			id, ok := t.keys[key]
			if !ok {
				b, e := parametric.EncodeSurface(a.Surface)
				if e != nil {
					return nil, e
				}
				id = len(t.recipes)
				t.keys[key] = id
				var second json.RawMessage
				if a.Second != nil {
					second, e = parametric.EncodeSurface(a.Second)
					if e != nil {
						return nil, e
					}
				}
				t.recipes = append(t.recipes, nativeImplicitRecipe{b, second, a.Plane.Origin(), a.Plane.XAxis(), a.Plane.ZAxis(), a.Options})
			}
			x.Implicit = &nativeImplicitUse{id, a.Contour, a.Segment, a.OnPlane}
		case intersect.TubeTrim:
			recipe, segment, second := c.Recipe()
			id, ok := t.tubeIDs[recipe]
			if !ok {
				data, e := encodeTubeRecipe(recipe.Data())
				if e != nil {
					return nil, e
				}
				id = len(t.tubes)
				t.tubeIDs[recipe] = id
				t.tubes = append(t.tubes, data)
			}
			x.Tube = &nativeTubeUse{id, segment, second}
		case wallTrim:
			b, e := parametric.EncodeSurface(c.surface)
			if e != nil {
				return nil, e
			}
			x.Wall = &nativeWallTrim{b, c.a, c.b, c.length, c.height, c.start, c.end}
		case reverseTrim:
			base, e := t.encodeUses([]Use{{Curve: c.differentiableTrim}})
			if e != nil {
				return nil, e
			}
			x.Subcurve, x.Flip = &base[0], true
		case SplineUV:
			b, e := parametric.EncodeCurve(c.Curve)
			if e != nil {
				return nil, e
			}
			x.Spline, x.SplineRange, x.SplineReversed = b, c.Range, c.Reversed
		case subTrim:
			base, e := t.encodeUses([]Use{{Curve: c.base}})
			if e != nil {
				return nil, e
			}
			x.Subcurve, x.SubRange = &base[0], UVInterval{c.min, c.max}

		default:
			return nil, fmt.Errorf("trimmed: unsupported p-curve %T", c)
		}
		r[i] = x
	}
	return r, nil
}

// WriteJSON preserves supported model curves, surfaces and p-curves and all
// indexed topology. Evaluators outside the native codec are explicit errors.
func WriteJSON(w io.Writer, s Solid) error {
	if w == nil {
		return fmt.Errorf("trimmed: nil writer")
	}
	d := nativeSolid{Version: 1, Kind: "trimmed-brep", Tolerance: s.tolerance, Vertices: s.vertices}
	t := trimEncoder{keys: map[implicitRecipeKey]int{}, tubeIDs: map[intersect.TubeRecipe]int{}}
	for _, e := range s.edges {
		r := nativeEdge{Start: e.Start, End: e.End, Range: e.Range}
		if c, ok := e.Curve.(surfaceTrimCurve); ok {
			v, err := parametric.EncodeSurface(c.surface)
			if err != nil {
				return err
			}
			x, err := t.encodeUses([]Use{{UVStart: c.a, UVEnd: c.b, Curve: c.curve}})
			if err != nil {
				return err
			}
			r.Surface, r.P = v, &x[0]
		} else {
			var err error
			r.Curve, err = parametric.EncodeCurve(e.Curve)
			if err != nil {
				return err
			}
		}
		d.Edges = append(d.Edges, r)
	}
	for _, f := range s.faces {
		surface, err := parametric.EncodeSurface(f.Surface)
		if err != nil {
			return err
		}
		outer, err := t.encodeUses(f.Outer)
		if err != nil {
			return err
		}
		n := nativeFace{Surface: surface, Outer: outer, Reversed: f.Reversed}
		for _, h := range f.Holes {
			u, err := t.encodeUses(h)
			if err != nil {
				return err
			}
			n.Holes = append(n.Holes, u)
		}
		d.Faces = append(d.Faces, n)
	}
	if len(t.recipes) > 0 {
		if len(t.recipes) > 8 {
			return fmt.Errorf("trimmed: implicit recipe count limit")
		}
		d.Version, d.Implicit = 2, t.recipes
		for _, r := range t.recipes {
			if !persistableImplicitOptions(r.Options, 4096) {
				return fmt.Errorf("trimmed: implicit recipe exceeds work limit")
			}
			if len(r.Second) > 0 {
				d.Version = 3
			}
		}
	}
	if len(t.tubes) != 0 {
		if e := validateTubeRecipes(t.tubes, 4096); e != nil {
			return e
		}
		d.Version, d.Tubes = 3, t.tubes
	}
	if t.exactQuadratic {
		if e := validateNativeExactQuadratics(context.Background(), s); e != nil {
			return e
		}
		d.Version = 4
	}
	if t.rationalEnds {
		d.Version = 5
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

type trimDecoder struct {
	maxControls int
	version     int
	recipes     [][]intersect.CertifiedContour
	tubes       [][]intersect.TubeSegment
}

func (d trimDecoder) decodeUses(n []nativeUse, depth int) ([]Use, error) {
	if depth > 2 {
		return nil, fmt.Errorf("trimmed: nested p-curve limit")
	}
	r := make([]Use, len(n))
	for i, x := range n {
		u := Use{Edge: x.Edge, Reversed: x.Reversed, UVStart: x.UVStart, UVEnd: x.UVEnd}
		if x.RationalEnds && (d.version < 5 || x.ExactQuadratic == nil) {
			return nil, fmt.Errorf("trimmed: invalid rational endpoint representation")
		}
		if x.AnchoredCircle && x.Ellipse == nil {
			return nil, fmt.Errorf("trimmed: missing anchored circle data")
		}
		if x.ExactQuadratic != nil {
			if d.version < 4 || d.maxControls < 3 || x.Implicit != nil || x.Tube != nil || x.Wall != nil ||
				x.Quadratic != nil || x.Ellipse != nil || len(x.Spline) > 0 || x.Subcurve != nil ||
				x.SplineReversed || x.SplineRange != (UVInterval{}) || x.Flip || x.SubRange != (UVInterval{}) {
				return nil, fmt.Errorf("trimmed: invalid exact quadratic trim representation")
			}
			var h [3][3]*big.Rat
			for j, p := range *x.ExactQuadratic {
				for k, raw := range p {
					// A 4096-bit numerator and denominator require
					// fewer than 2500 characters in canonical decimal.
					if len(raw) == 0 || len(raw) > 2500 {
						return nil, fmt.Errorf("trimmed: exact trim coefficient exceeds bounds")
					}
					q, ok := new(big.Rat).SetString(raw)
					if !ok || q.RatString() != raw {
						return nil, fmt.Errorf("trimmed: invalid exact trim coefficient")
					}
					h[j][k] = q
				}
			}
			q, e := newExactQuadraticUV(h, x.RationalEnds)
			if e != nil {
				return nil, e
			}
			u.Curve = q
			r[i] = u
			continue
		}
		if x.Subcurve != nil {
			if x.Implicit != nil || x.Tube != nil || x.Wall != nil || x.Quadratic != nil || x.Ellipse != nil || len(x.Spline) > 0 ||
				x.Flip && x.SubRange != (UVInterval{}) ||
				!x.Flip && (!finite(x.SubRange.Min) || !finite(x.SubRange.Max) || x.SubRange.Min < 0 || x.SubRange.Max > 1 || x.SubRange.Max <= x.SubRange.Min) {
				return nil, fmt.Errorf("trimmed: invalid p-curve subinterval")
			}
			p, e := d.decodeUses([]nativeUse{*x.Subcurve}, depth+1)
			if e != nil {
				return nil, e
			}
			if p[0].Curve == nil {
				return nil, fmt.Errorf("trimmed: missing base p-curve")
			}
			if x.Flip {
				d, ok := p[0].Curve.(differentiableTrim)
				if !ok {
					return nil, fmt.Errorf("trimmed: missing differentiable p-curve")
				}
				u.Curve = reverseTrim{d}
			} else {
				u.Curve = subTrim{p[0].Curve, x.SubRange.Min, x.SubRange.Max}
			}
			r[i] = u
			continue
		}

		if x.Flip || x.SubRange != (UVInterval{}) {
			return nil, fmt.Errorf("trimmed: missing base p-curve")
		}
		if w := x.Wall; w != nil {
			if x.Implicit != nil || x.Tube != nil || x.Quadratic != nil || x.Ellipse != nil || len(x.Spline) > 0 ||
				!uvFinite(w.A) || !uvFinite(w.B) || w.A == w.B || !finite(w.Length) || w.Length <= 0 ||
				!finite(w.Height) || !finite(w.Start) || !finite(w.End) {
				return nil, fmt.Errorf("trimmed: invalid wall trim")
			}
			c, e := parametric.DecodeSurface(w.Surface, d.maxControls)
			if e != nil {
				return nil, e
			}
			s, ok := c.(*geom.BSplineSurface)
			if !ok {
				return nil, fmt.Errorf("trimmed: wall trim requires spline")
			}
			u.Curve = wallTrim{s, w.A, w.B, w.Length, w.Height, w.Start, w.End}
			r[i] = u
			continue
		}
		if a := x.Tube; a != nil {
			if x.Implicit != nil || x.Quadratic != nil || x.Ellipse != nil || len(x.Spline) > 0 || a.Recipe < 0 || a.Recipe >= len(d.tubes) ||
				a.Segment < 0 || a.Segment >= len(d.tubes[a.Recipe]) {
				return nil, fmt.Errorf("trimmed: invalid tube p-curve reference")
			}
			s := d.tubes[a.Recipe][a.Segment]
			if a.Second {
				u.Curve = s.TrimB()
			} else {
				u.Curve = s.TrimA()
			}
		} else if a := x.Implicit; a != nil {
			if x.Quadratic != nil || x.Ellipse != nil || len(x.Spline) > 0 || a.Recipe < 0 || a.Recipe >= len(d.recipes) ||
				a.Contour < 0 || a.Contour >= len(d.recipes[a.Recipe]) || a.Segment < 0 || a.Segment >= len(d.recipes[a.Recipe][a.Contour].Segments) {
				return nil, fmt.Errorf("trimmed: invalid implicit p-curve reference")
			}
			segment := d.recipes[a.Recipe][a.Contour].Segments[a.Segment]
			if a.OnPlane {
				u.Curve = segment.PlaneTrim()
			} else {
				u.Curve = segment.SplineTrim()
			}
		} else if len(x.Spline) > 0 {
			if x.Quadratic != nil || x.Ellipse != nil {
				return nil, fmt.Errorf("trimmed: multiple p-curve representations")
			}
			c, e := parametric.DecodeCurve(x.Spline, d.maxControls)
			if e != nil {
				return nil, e
			}
			s, ok := c.(*geom.BSpline)
			if !ok {
				return nil, fmt.Errorf("trimmed: p-curve spline type mismatch")
			}
			u.Curve = SplineUV{Curve: s, Range: x.SplineRange, Reversed: x.SplineReversed}
		} else if x.Quadratic != nil {
			if x.Ellipse != nil {
				return nil, fmt.Errorf("trimmed: multiple p-curve representations")
			}
			u.Curve = *x.Quadratic
		} else if x.Ellipse != nil {
			u.Curve = *x.Ellipse
			if x.AnchoredCircle {
				q, e := newAnchoredCircularUV(*x.Ellipse, x.UVStart, x.UVEnd)
				if e != nil {
					return nil, e
				}
				u.Curve = q
			}
		}
		r[i] = u
	}
	return r, nil
}

// ReadJSON bounds and validates a version 1 through 5 trimmed boundary. Zero fields
// select 32 MiB, 20,000 entities and 4,096 poles per geometry record.
func ReadJSON(r io.Reader, o JSONLimits) (Solid, error) {
	return ReadJSONContext(context.Background(), r, o)
}

// ReadJSONContext bounds the same schema as ReadJSON and cooperatively
// cancels between model records and during exact contour reconstruction.
// A blocking reader remains responsible for its own I/O cancellation.
func ReadJSONContext(ctx context.Context, r io.Reader, o JSONLimits) (Solid, error) {
	if ctx == nil || r == nil || o.MaxBytes < 0 || o.MaxBytes > 1<<30 || o.MaxEntities < 0 || o.MaxControlPoints < 0 {
		return Solid{}, fmt.Errorf("trimmed: invalid input limits")
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 32 << 20
	}
	if o.MaxEntities == 0 {
		o.MaxEntities = 20000
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	b, err := io.ReadAll(io.LimitReader(r, o.MaxBytes+1))
	if err != nil {
		return Solid{}, err
	}
	if int64(len(b)) > o.MaxBytes {
		return Solid{}, fmt.Errorf("trimmed: byte limit exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var n nativeSolid
	if err = d.Decode(&n); err != nil {
		return Solid{}, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return Solid{}, fmt.Errorf("trimmed: trailing data")
	}
	if n.Version < 1 || n.Version > 5 || n.Kind != "trimmed-brep" || n.Version == 1 && len(n.Implicit) > 0 ||
		n.Version < 3 && len(n.Tubes) > 0 {
		return Solid{}, fmt.Errorf("trimmed: unsupported schema")
	}
	total := len(n.Vertices) + len(n.Edges) + len(n.Faces) + len(n.Implicit) + len(n.Tubes)
	for _, t := range n.Tubes {
		total += len(t.Cells)
	}
	for _, f := range n.Faces {
		total += len(f.Outer)
		for _, h := range f.Holes {
			total += len(h)
		}
	}
	if total > o.MaxEntities {
		return Solid{}, fmt.Errorf("trimmed: entity limit exceeded")
	}
	if len(n.Implicit) > 8 {
		return Solid{}, fmt.Errorf("trimmed: implicit recipe count limit")
	}
	dc := trimDecoder{version: n.Version, maxControls: o.MaxControlPoints, recipes: make([][]intersect.CertifiedContour, len(n.Implicit))}
	if e := validateTubeRecipes(n.Tubes, o.MaxControlPoints); e != nil {
		return Solid{}, e
	}
	for i, t := range n.Tubes {
		curves, e := t.rebuild(ctx, o.MaxControlPoints)
		if e != nil {
			return Solid{}, fmt.Errorf("trimmed: tube recipe %d: %w", i, e)
		}
		dc.tubes = append(dc.tubes, curves)
	}
	for i, r := range n.Implicit {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if !persistableImplicitOptions(r.Options, o.MaxControlPoints) {
			return Solid{}, fmt.Errorf("trimmed: implicit recipe exceeds work limit")
		}
		v, e := parametric.DecodeSurface(r.Surface, o.MaxControlPoints)
		if e != nil {
			return Solid{}, e
		}
		s, ok := v.(*geom.BSplineSurface)
		if !ok {
			return Solid{}, fmt.Errorf("trimmed: implicit recipe requires spline")
		}
		f, e := geom.NewFrame(r.Origin, r.X, r.Z)
		if e != nil {
			return Solid{}, e
		}
		var c []intersect.CertifiedContour
		if len(r.Second) > 0 {
			if n.Version < 3 || f.Origin() != (geom.Vec3{}) || f.XAxis() != (geom.Vec3{X: 1}) || f.ZAxis() != (geom.Vec3{Z: 1}) {
				return Solid{}, fmt.Errorf("trimmed: invalid common-chart implicit recipe")
			}
			v, err := parametric.DecodeSurface(r.Second, o.MaxControlPoints)
			if err != nil {
				return Solid{}, err
			}
			b, ok := v.(*geom.BSplineSurface)
			if !ok {
				return Solid{}, fmt.Errorf("trimmed: implicit height difference requires two splines")
			}
			c, _, e = intersect.TraceSplineHeightDifferenceContext(ctx, s, b, r.Options)
		} else {
			c, _, e = intersect.TraceSplinePlaneContext(ctx, s, f, r.Options)
		}
		if e != nil {
			return Solid{}, e
		}
		dc.recipes[i] = c
	}
	edges := make([]parametric.Edge, len(n.Edges))
	for i, e := range n.Edges {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		var c geom.Curve
		if e.P != nil {
			if len(e.Curve) > 0 && string(e.Curve) != "null" {
				return Solid{}, fmt.Errorf("trimmed: multiple model curve representations")
			}
			v, err := parametric.DecodeSurface(e.Surface, o.MaxControlPoints)
			if err != nil {
				return Solid{}, err
			}
			p, err := dc.decodeUses([]nativeUse{*e.P}, 0)
			if err != nil {
				return Solid{}, err
			}
			x := p[0]
			c = surfaceTrimCurve{v, x.UVStart, x.UVEnd, x.Curve}
		} else {
			if len(e.Surface) > 0 {
				return Solid{}, fmt.Errorf("trimmed: surface curve missing p-curve")
			}
			var err error
			c, err = parametric.DecodeCurve(e.Curve, o.MaxControlPoints)
			if err != nil {
				return Solid{}, fmt.Errorf("trimmed: edge %d: %w", i, err)
			}
		}
		edges[i] = parametric.Edge{Start: e.Start, End: e.End, Curve: c, Range: e.Range}
	}
	faces := make([]Face, len(n.Faces))
	for i, f := range n.Faces {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		s, err := parametric.DecodeSurface(f.Surface, o.MaxControlPoints)
		if err != nil {
			return Solid{}, fmt.Errorf("trimmed: face %d: %w", i, err)
		}
		u, err := dc.decodeUses(f.Outer, 0)
		if err != nil {
			return Solid{}, err
		}
		face := Face{Surface: s, Outer: u, Reversed: f.Reversed}
		for _, h := range f.Holes {
			loop, err := dc.decodeUses(h, 0)
			if err != nil {
				return Solid{}, err
			}
			face.Holes = append(face.Holes, loop)
		}
		faces[i] = face
	}
	solid, e := NewSolidContext(ctx, n.Vertices, edges, faces, n.Tolerance)
	if e != nil || n.Version < 4 {
		return solid, e
	}
	// Version 4's exact trim coefficients carry no proof flags. Rederive
	// topology and continuous agreement for every affected owner on reload.
	if e := validateNativeExactQuadratics(ctx, solid); e != nil {
		return Solid{}, e
	}
	return solid, nil
}

func validateNativeExactQuadratics(ctx context.Context, solid Solid) error {
	if _, e := solid.RationalVertices(ctx); e != nil {
		return e
	}
	var containsExact func(PCurve) bool
	containsExact = func(p PCurve) bool {
		switch c := p.(type) {
		case ExactQuadraticUV:
			return true
		case reverseTrim:
			return containsExact(c.differentiableTrim)
		case subTrim:
			return containsExact(c.base)
		}
		return false
	}
	affected := make(map[int]bool)
	for i, m := range solid.edges {
		if c, ok := m.Curve.(surfaceTrimCurve); ok && containsExact(c.curve) {
			affected[i] = true
		}
	}
	for _, f := range solid.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if containsExact(u.Curve) {
					affected[u.Edge] = true
				}
				if q, ok := u.Curve.(ExactQuadraticUV); ok && q.rationalEndpoints {
					m, ok := solid.edges[u.Edge].Curve.(surfaceTrimCurve)
					model, exact := m.curve.(ExactQuadraticUV)
					if !ok || !exact || !model.rationalEndpoints {
						return geom.ErrConvergence
					}
				}
			}
		}
	}
	if len(affected) == 0 {
		return nil
	}
	if len(affected) > 256 {
		return geom.ErrConvergence
	}
	for _, f := range solid.faces {
		needsTrimCheck := false
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if !affected[u.Edge] {
					continue
				}
				if e := ctx.Err(); e != nil {
					return e
				}
				proven, e := compareSplineRationalTrimUse(ctx, solid.edges[u.Edge], f.Surface, u, edgecheck.Options{})
				if e != nil {
					return e
				}
				if !proven {
					return geom.ErrConvergence
				}
				needsTrimCheck = true
			}
		}
		if needsTrimCheck {
			r, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
			if e != nil {
				return e
			}
			if r.Status != trimvalidate.Certified {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}

func persistableImplicitOptions(o intersect.SplinePlaneOptions, maxControls int) bool {
	a, p, r := o.Trace.Arcs, o.Patch, o.Evaluation
	return p.MaxPoles <= min(maxControls, 4096) && p.MaxOperations <= 1000000 && p.MaxCoefficientBits <= 16384 &&
		a.MaxDepth <= 20 && a.MaxCells <= 16384 && a.MaxQueuedPoles <= 262144 && a.MaxOperations <= 2000000 &&
		a.MaxCoefficientBits <= 16384 && a.MaxRootSubdivisions <= 8192 &&
		o.Trace.MaxJoinComparisons <= 1000000 && o.Trace.MaxJoinOperations <= 2000000 && o.Trace.MaxCoefficientBits <= 16384 &&
		r.MaxOperations <= 1000000 && r.MaxCoefficientBits <= 16384 && r.MaxSubdivisions <= 4096
}
