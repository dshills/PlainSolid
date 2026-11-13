package parametric

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"

	"plainsolid/geom"
)

// JSONLimits bounds native parametric topology and control data.
type JSONLimits struct {
	MaxBytes                      int64
	MaxEntities, MaxControlPoints int
}

type frameData struct{ Origin, X, Z geom.Vec3 }

func frameToData(f geom.Frame) *frameData { return &frameData{f.Origin(), f.XAxis(), f.ZAxis()} }
func (f *frameData) decode() (geom.Frame, error) {
	if f == nil {
		return geom.Frame{}, geom.ErrInvalid
	}
	return geom.NewFrame(f.Origin, f.X, f.Z)
}

type surfaceData struct {
	ObliqueSplit     *obliqueSplitData     `json:",omitempty"`
	SplitCylinderCut *splitCylinderCutData `json:",omitempty"`
	SteinmetzUnion   *steinmetzUnionData   `json:",omitempty"`
	ObliqueUnion     *obliqueUnionData     `json:",omitempty"`
	SteinmetzDualCap *steinmetzDualCapData `json:",omitempty"`
	SteinmetzSlab    *steinmetzSlabData    `json:",omitempty"`
	Steinmetz        *[2]int               `json:",omitempty"`
	Transform        *geom.Affine          `json:",omitempty"`
	Base             *surfaceData          `json:",omitempty"`
	Kind             string
	Frame            *frameData               `json:",omitempty"`
	A, B             float64                  `json:",omitempty"`
	Meridian         *[2]geom.Vec2            `json:",omitempty"`
	Spline           *geom.BSplineSurfaceData `json:",omitempty"`
	Center           *geom.Vec3               `json:",omitempty"`
	Other            *surfaceData             `json:",omitempty"`
	U0, U1           *Interval                `json:",omitempty"`
	Rim              *rimData                 `json:",omitempty"`
	CutTop           bool                     `json:",omitempty"`
	Band             *[2][4]float64           `json:",omitempty"`
	Wrap             *wrappedSurfaceData      `json:",omitempty"`
	Helical          *helicalSurfaceData      `json:",omitempty"`
	Twisted          *[2]float64              `json:",omitempty"`
	AngularReversed  bool                     `json:",omitempty"`
	OffsetBore       *offsetBoreData          `json:",omitempty"`
	OffsetCommon     *offsetCommonData        `json:",omitempty"`
	PartialCommon    *offsetPartialData       `json:",omitempty"`
	DoubleCommon     *offsetDoubleData        `json:",omitempty"`
	SameSideCommon   *offsetDoubleData        `json:",omitempty"`
	UnequalCylinder  *unequalCylinderData     `json:",omitempty"`
	SkewCylinder     *skewCylinderData        `json:",omitempty"`
	SkewOverlap      *skewOverlapData         `json:",omitempty"`
	ObliqueCylinder  *obliqueCylinderData     `json:",omitempty"`
	GeneralCylinder  *generalCylinderData     `json:",omitempty"`
}

type obliqueSplitData struct {
	Center, StockAxis, CutterAxis geom.Vec3
	Stock, Cutter                 float64
	StockCaps, CutterCaps         [2]float64
	Side, Half                    int
	Cap                           bool
}

type splitCylinderCutData struct {
	Center             geom.Vec3
	Stock, Cutter, End float64
	Side, Half         int
	Cap                bool
}

type obliqueUnionData struct {
	Center, U, V      geom.Vec3
	Radius            float64
	Caps              [2][2]float64
	Owner, Side, Half int
	Cap               bool
}

type steinmetzUnionData struct {
	Center             geom.Vec3
	Radius, HalfLength float64
	Axis, Side, Half   int
	Cap                bool
}

type generalCylinderData struct {
	Center, U, V     geom.Vec3
	RadiusU, RadiusV float64
	Index            int
}

type skewOverlapData struct {
	Center                   geom.Vec3
	RadiusX, RadiusZ, Offset float64
	Chart, Sign              int
	Reflected                bool
}

type obliqueCylinderData struct {
	Center, U, V geom.Vec3
	Radius       float64
	Index        int
}

type skewCylinderData struct {
	unequalCylinderData
	Shift float64
}

type unequalCylinderData struct {
	Center       geom.Vec3
	Small, Large float64
	Chart, Sign  int
	Swap         bool
}

type offsetBoreData struct {
	Radius, Bore, Offset, Floor float64
	Chart                       int
}

type offsetCommonData struct {
	Radius, Tool, Offset, Lower, Upper float64
	Chart                              int
}

type offsetPartialData struct {
	offsetCommonData
	Clipped   bool
	ChordMode bool `json:",omitempty"`
}

type offsetDoubleData struct {
	offsetCommonData
	Clipped [2]bool
}

type helicalSurfaceData struct {
	Lead       float64
	Handedness geom.HelixHandedness
	Turns      Interval
}

type wrappedSurfaceData struct {
	U, V            bool
	ContinuityOrder int
}

type rimData struct {
	Frame                        *frameData
	Radius, Height, Cosine, Sine float64
}

func encodeRim(r slantedRim) *rimData {
	f, rad := r.cyl.Parameters()
	return &rimData{frameToData(f), rad, r.center, r.cosine, r.sine}
}
func (r *rimData) decode() (slantedRim, error) {
	if r == nil || !finite(r.Height) || !finite(r.Cosine) || !finite(r.Sine) {
		return slantedRim{}, geom.ErrInvalid
	}
	f, e := r.Frame.decode()
	if e != nil {
		return slantedRim{}, e
	}
	s, e := geom.NewCylinderSurface(f, r.Radius)
	return slantedRim{s, r.Height, r.Cosine, r.Sine}, e
}

func encodeSurface(s geom.Surface) (surfaceData, error) {
	switch x := s.(type) {
	case mappedSurface:
		base, e := encodeSurface(x.surface)
		if e != nil {
			return surfaceData{}, e
		}
		return surfaceData{Kind: "mapped", Transform: &x.transform, Base: &base}, nil
	case geom.PlaneSurface:
		return surfaceData{Kind: "plane", Frame: frameToData(x.Frame())}, nil
	case geom.CylinderSurface:
		f, r := x.Parameters()
		return surfaceData{Kind: "cylinder", Frame: frameToData(f), A: r}, nil
	case geom.TwistedCylinderSurface:
		f, r, twist, phase := x.Parameters()
		return surfaceData{Kind: "twisted-cylinder", Frame: frameToData(f), A: r, Twisted: &[2]float64{twist, phase}, AngularReversed: x.AngularReversed()}, nil
	case geom.TwistedAnnularSurface:
		f, inner, outer, z, phase := x.Parameters()
		return surfaceData{Kind: "twisted-annulus", Frame: frameToData(f), A: inner, B: outer, Twisted: &[2]float64{z, phase}}, nil
	case geom.ConeSurface:
		f, a := x.Parameters()
		return surfaceData{Kind: "cone", Frame: frameToData(f), A: a}, nil
	case geom.SphereSurface:
		f, r := x.Parameters()
		return surfaceData{Kind: "sphere", Frame: frameToData(f), A: r}, nil
	case offsetBoreChart:
		return surfaceData{Kind: "offset-sphere-bore", Frame: frameToData(x.frame), OffsetBore: &offsetBoreData{x.radius, x.bore, x.offset, x.floor, x.kind}}, nil
	case steinmetzExteriorCylinder:
		return surfaceData{Kind: "steinmetz-exterior-cylinder", Center: &x.center, A: x.radius, B: x.end, Steinmetz: &[2]int{x.side, x.sign}}, nil
	case steinmetzDualCapChart:
		d := &steinmetzDualCapData{Axis: x.axis, Side: x.side, Large: x.large, WideAxis: x.wideAxis}
		if x.large == 0 && x.caps[0][0] == -x.radius && x.caps[1][0] == -x.radius && x.caps[0][1] < x.radius && x.caps[1][1] < x.radius {
			d.Caps = [2]float64{x.caps[0][1], x.caps[1][1]}
			d.Fixed = [2]bool{x.fixed[0][1], x.fixed[1][1]}
		} else {
			d.Windows, d.Clipped = &x.caps, &x.fixed
		}
		return surfaceData{Kind: "steinmetz-dual-cap", Center: &x.center, A: x.radius, SteinmetzDualCap: d}, nil
	case steinmetzSlabChart:
		return surfaceData{Kind: "steinmetz-slab", Center: &x.center, A: x.radius, SteinmetzSlab: &steinmetzSlabData{x.lower, x.upper, x.lowFixed, x.highFixed, x.kind}}, nil
	case steinmetzClippedCylinder:
		return surfaceData{Kind: "steinmetz-clipped-cylinder", Center: &x.center, A: x.radius, B: x.cap, Steinmetz: &[2]int{0, x.sign}}, nil
	case steinmetzUnionChart:
		return surfaceData{Kind: "steinmetz-union", SteinmetzUnion: &steinmetzUnionData{x.center, x.radius, x.halfLength, x.axis, x.side, x.half, x.cap}}, nil
	case obliqueCylinderUnionChart:
		return surfaceData{Kind: "oblique-cylinder-union", ObliqueUnion: &obliqueUnionData{x.center, x.u, x.v, x.radius, x.caps, x.owner, x.side, x.half, x.cap}}, nil
	case splitCylinderCutChart:
		return surfaceData{Kind: "split-cylinder-cut", SplitCylinderCut: &splitCylinderCutData{x.center, x.stock, x.cutter, x.end, x.side, x.half, x.cap}}, nil
	case obliqueSplitChart:
		return surfaceData{Kind: "oblique-split-cylinder-cut", ObliqueSplit: &obliqueSplitData{x.center, x.stockAxis, x.cutterAxis, x.stock, x.cutter, x.stockCaps, x.cutterCaps, x.side, x.half, x.cap}}, nil
	case steinmetzDisk:
		return surfaceData{Kind: "steinmetz-disk", Center: &x.center, A: x.radius}, nil
	case steinmetzChart:
		return surfaceData{Kind: "steinmetz-cylinder", Center: &x.center, A: x.radius, Steinmetz: &[2]int{x.axis, x.sign}}, nil
	case unequalCylinderCommonChart:
		return surfaceData{Kind: "perpendicular-cylinder-common", UnequalCylinder: &unequalCylinderData{x.center, x.small, x.large, x.k, x.sign, x.swap}}, nil
	case skewCylinderChart:
		return surfaceData{Kind: "skew-cylinder-common", SkewCylinder: &skewCylinderData{unequalCylinderData{x.center, x.small, x.large, x.k, x.sign, x.swap}, x.shift}}, nil
	case skewOverlapCylinderChart:
		return surfaceData{Kind: "skew-cylinder-overlap", SkewOverlap: &skewOverlapData{x.center, x.radiusX, x.radiusZ, x.offset, x.k, x.sign, x.reflected}}, nil
	case obliqueCylinderChart:
		return surfaceData{Kind: "oblique-cylinder-common", ObliqueCylinder: &obliqueCylinderData{x.center, x.u, x.v, x.radius, x.index}}, nil
	case generalCylinderCommonChart:
		return surfaceData{Kind: "general-cylinder-common", GeneralCylinder: &generalCylinderData{x.center, x.u, x.v, x.ru, x.rv, x.index}}, nil
	case toolBoreChart:
		return surfaceData{Kind: "offset-sphere-bore", Frame: frameToData(x.frame), OffsetBore: &offsetBoreData{x.radius, x.bore, x.offset, x.floor, x.kind}}, nil
	case offsetCommonChart:
		return surfaceData{Kind: "offset-sphere-common", Frame: frameToData(x.frame), OffsetCommon: &offsetCommonData{x.R, x.r, x.offset, x.lower, x.upper, x.kind}}, nil
	case offsetPartialCommon:
		return surfaceData{Kind: "offset-sphere-partial-common", Frame: frameToData(x.frame), PartialCommon: &offsetPartialData{offsetCommonData{x.R, x.r, x.offset, x.lower, x.upper, x.kind}, x.clipped, x.chordMode}}, nil
	case offsetDoubleCommon:
		return surfaceData{Kind: "offset-sphere-double-common", Frame: frameToData(x.frame), DoubleCommon: &offsetDoubleData{offsetCommonData{x.R, x.r, x.d, x.lower, x.upper, x.kind}, x.clipped}}, nil
	case offsetSameSideCommon:
		return surfaceData{Kind: "offset-sphere-same-side-common", Frame: frameToData(x.frame), SameSideCommon: &offsetDoubleData{offsetCommonData{x.R, x.r, x.d, x.lower, x.upper, x.kind}, x.clipped}}, nil
	case geom.TorusSurface:
		f, a, b := x.Parameters()
		return surfaceData{Kind: "torus", Frame: frameToData(f), A: a, B: b}, nil
	case reparameterizedCylinder:
		base, e := encodeSurface(x.s)
		if e != nil {
			return surfaceData{}, e
		}
		return surfaceData{Kind: "reparameterized-cylinder", Base: &base, U0: &x.u}, nil
	case ruledArcs:
		base, e := encodeSurface(x.a)
		if e != nil {
			return surfaceData{}, e
		}
		other, e := encodeSurface(x.b)
		if e != nil {
			return surfaceData{}, e
		}
		return surfaceData{Kind: "ruled-arcs", Base: &base, Other: &other, U0: &x.u, U1: &x.v, A: x.height}, nil

	case cutCylinder:
		return surfaceData{Kind: "cut-cylinder", Rim: encodeRim(x.rim), A: x.constant, CutTop: x.cutTop}, nil
	case slantedFan:
		return surfaceData{Kind: "slanted-fan", Rim: encodeRim(x.rim), Center: &x.center}, nil

	case arcFan:
		f, r := x.surface.Parameters()
		return surfaceData{Kind: "arc-fan", Frame: frameToData(f), Center: &x.center, A: r, B: x.height}, nil

	case disk:
		return surfaceData{Kind: "disk", Frame: frameToData(x.frame)}, nil
	case ellipticPatch:
		return surfaceData{Kind: "elliptic", Frame: frameToData(x.frame), A: x.height, Meridian: &[2]geom.Vec2{x.lower, x.upper}}, nil
	case ellipticBand:
		return surfaceData{Kind: "elliptic-band", Frame: frameToData(x.frame), A: x.z, B: x.h, Band: &[2][4]float64{x.x, x.y}}, nil
	case revolvedSide:
		return surfaceData{Kind: "revolved", Frame: frameToData(x.frame), Meridian: &[2]geom.Vec2{x.a, x.b}}, nil
	case geom.HelicalProfileSurface:
		f, a, b, lead, hand := x.Parameters()
		d := x.Domain()
		return surfaceData{Kind: "helical-profile", Frame: frameToData(f), Meridian: &[2]geom.Vec2{a, b},
			Helical: &helicalSurfaceData{lead, hand, Interval{Min: d.UMin, Max: d.UMax}}}, nil
	case *geom.BSplineSurface:
		d := x.Data()
		return surfaceData{Kind: "bspline", Spline: &d}, nil
	case *geom.WrappedSurfacePatch:
		if x == nil || x.FundamentalPatch() == nil {
			return surfaceData{}, geom.ErrInvalid
		}
		recipe, e := x.ExportRecipe()
		if e != nil {
			return surfaceData{}, e
		}
		d := recipe.Surface.Data()
		if len(d.Poles) > 4096 || d.DegreeU > 16 || d.DegreeV > 16 {
			return surfaceData{}, fmt.Errorf("parametric: wrapped spline surface exceeds native codec limits")
		}
		o := recipe.Options
		w := wrappedSurfaceData{U: o.WrapU, V: o.WrapV, ContinuityOrder: o.ContinuityOrder}
		return surfaceData{Kind: "wrapped-surface", Spline: &d, Wrap: &w}, nil
	default:
		return surfaceData{}, fmt.Errorf("parametric: unsupported surface type %T", s)
	}
}

func (d surfaceData) decode(budget *int) (geom.Surface, error) {
	if (d.Kind == "oblique-split-cylinder-cut") != (d.ObliqueSplit != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.ObliqueSplit; x != nil {
		if x.Side != 1 && x.Side != -1 || x.Half != 1 && x.Half != -1 {
			return nil, geom.ErrInvalid
		}
		s, _, e := newObliqueSplitChart(context.Background(), x.Center, x.StockAxis, x.CutterAxis, x.Stock, x.Cutter, x.StockCaps, x.CutterCaps)
		if e != nil {
			return nil, e
		}
		s.side, s.half, s.cap = x.Side, x.Half, x.Cap
		return s, nil
	}
	if (d.Kind == "split-cylinder-cut") != (d.SplitCylinderCut != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.SplitCylinderCut; x != nil {
		if !x.Center.IsFinite() || !finite(x.Stock) || !finite(x.Cutter) || !finite(x.End) || x.Stock <= 0 || x.Cutter <= x.Stock || x.End <= x.Cutter || x.Side != 1 && x.Side != -1 || x.Half != 1 && x.Half != -1 {
			return nil, geom.ErrInvalid
		}
		return splitCylinderCutChart{x.Center, x.Stock, x.Cutter, x.End, x.Side, x.Half, x.Cap}, nil
	}
	if (d.Kind == "oblique-cylinder-union") != (d.ObliqueUnion != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.ObliqueUnion; x != nil {
		if x.Owner < 0 || x.Owner > 1 || x.Side != 1 && x.Side != -1 || x.Half != 1 && x.Half != -1 {
			return nil, geom.ErrInvalid
		}
		s, e := newObliqueCylinderUnionChart(context.Background(), x.Center, x.U, x.V, x.Radius, x.Caps[0], x.Caps[1])
		if e != nil {
			return nil, e
		}
		s.owner, s.side, s.half, s.cap = x.Owner, x.Side, x.Half, x.Cap
		return s, nil
	}
	if (d.Kind == "steinmetz-union") != (d.SteinmetzUnion != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.SteinmetzUnion; x != nil {
		if !x.Center.IsFinite() || !finite(x.Radius) || !finite(x.HalfLength) || x.Radius <= 0 || x.HalfLength <= x.Radius || x.Axis < 0 || x.Axis > 1 || x.Side != 1 && x.Side != -1 || x.Half != 1 && x.Half != -1 {
			return nil, geom.ErrInvalid
		}
		return steinmetzUnionChart{x.Center, x.Radius, x.HalfLength, x.Axis, x.Side, x.Half, x.Cap}, nil
	}
	if (d.Kind == "general-cylinder-common") != (d.GeneralCylinder != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.GeneralCylinder; x != nil {
		if x.Index < 0 || x.Index >= 4 {
			return nil, geom.ErrInvalid
		}
		walls, e := geom.NewCylinderCommonWalls(context.Background(), x.Center, x.U, x.V, x.RadiusU, x.RadiusV, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return nil, e
		}
		return generalCylinderCommonChart{walls[x.Index], x.Center, x.U, x.V, x.RadiusU, x.RadiusV, x.Index}, nil
	}
	if (d.Kind == "skew-cylinder-overlap") != (d.SkewOverlap != nil) || (d.Kind == "oblique-cylinder-common") != (d.ObliqueCylinder != nil) {
		return nil, geom.ErrInvalid
	}
	if x := d.SkewOverlap; x != nil {
		if !x.Center.IsFinite() || !finite(x.RadiusX) || !finite(x.RadiusZ) || !finite(x.Offset) || x.RadiusX <= 0 || x.RadiusZ <= 0 || x.Offset <= 0 || x.Chart < 0 || x.Chart > 1 || x.Sign != 1 && x.Sign != -1 {
			return nil, geom.ErrInvalid
		}
		q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
		L := new(big.Rat).Sub(new(big.Rat).Add(q(x.RadiusX), q(x.RadiusZ)), q(x.Offset))
		if L.Sign() <= 0 || L.Cmp(new(big.Rat).Mul(big.NewRat(2, 1), q(min(x.RadiusX, x.RadiusZ)))) >= 0 {
			return nil, geom.ErrInvalid
		}
		length, e := geom.RepresentedExactScalar(L)
		if e != nil {
			return nil, e
		}
		return skewOverlapCylinderChart{x.Center, x.RadiusX, x.RadiusZ, x.Offset, length, x.Chart, x.Sign, x.Reflected}, nil
	}
	if x := d.ObliqueCylinder; x != nil {
		if x.Index < 0 || x.Index >= 4 {
			return nil, geom.ErrInvalid
		}
		walls, e := geom.NewEqualCylinderCommonWalls(context.Background(), x.Center, x.U, x.V, x.Radius, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			return nil, e
		}
		return obliqueCylinderChart{walls[x.Index], x.Center, x.U, x.V, x.Radius, x.Index}, nil
	}
	if (d.Kind == "skew-cylinder-common") != (d.SkewCylinder != nil) {
		return nil, geom.ErrInvalid
	}
	if d.SkewCylinder != nil {
		x := d.SkewCylinder
		if !x.Center.IsFinite() || !finite(x.Small) || !finite(x.Large) || !finite(x.Shift) || x.Small <= 0 || x.Large <= x.Small || x.Shift == 0 || x.Chart < 0 || x.Chart > 1 || x.Sign != 1 && x.Sign != -1 {
			return nil, geom.ErrInvalid
		}
		q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
		if new(big.Rat).Add(q(x.Small), q(math.Abs(x.Shift))).Cmp(q(x.Large)) >= 0 {
			return nil, geom.ErrInvalid
		}
		offset := x.Shift
		if !x.Swap {
			offset = -offset
		}
		return skewCylinderChart{x.Center, x.Small, x.Large, x.Shift, offset, x.Chart, x.Sign, x.Swap}, nil
	}
	if (d.Kind == "perpendicular-cylinder-common") != (d.UnequalCylinder != nil) {
		return nil, geom.ErrInvalid
	}
	if d.UnequalCylinder != nil {
		x := d.UnequalCylinder
		if !x.Center.IsFinite() || !finite(x.Small) || !finite(x.Large) || x.Small <= 0 || x.Large <= x.Small || x.Chart < 0 || x.Chart > 1 || x.Sign != 1 && x.Sign != -1 {
			return nil, geom.ErrInvalid
		}
		return unequalCylinderCommonChart{x.Center, x.Small, x.Large, x.Chart, x.Sign, x.Swap}, nil
	}
	if (d.Kind == "offset-sphere-bore") != (d.OffsetBore != nil) {
		return nil, geom.ErrInvalid
	}
	if (d.Kind == "offset-sphere-common") != (d.OffsetCommon != nil) {
		return nil, geom.ErrInvalid
	}
	if (d.Kind == "offset-sphere-partial-common") != (d.PartialCommon != nil) {
		return nil, geom.ErrInvalid
	}
	if (d.Kind == "offset-sphere-double-common") != (d.DoubleCommon != nil) {
		return nil, geom.ErrInvalid
	}
	if (d.Kind == "offset-sphere-same-side-common") != (d.SameSideCommon != nil) {
		return nil, geom.ErrInvalid
	}
	if d.Kind != "wrapped-surface" && d.Wrap != nil {
		return nil, geom.ErrInvalid
	}
	if d.Kind != "helical-profile" && d.Helical != nil {
		return nil, geom.ErrInvalid
	}
	if d.Kind != "twisted-cylinder" && d.Kind != "twisted-annulus" && d.Twisted != nil {
		return nil, geom.ErrInvalid
	}
	if d.Kind != "twisted-cylinder" && d.AngularReversed {
		return nil, geom.ErrInvalid
	}
	if d.Kind == "mapped" {
		if d.Base == nil || d.Base.Kind == "mapped" || d.Transform == nil {
			return nil, geom.ErrInvalid
		}
		if _, e := d.Transform.Inverse(); e != nil {
			return nil, e
		}
		s, e := d.Base.decode(budget)
		if e != nil {
			return nil, e
		}
		return mappedSurface{s, *d.Transform}, nil
	}
	if d.Kind == "reparameterized-cylinder" || d.Kind == "ruled-arcs" {
		cylinder := func(x *surfaceData) (geom.CylinderSurface, error) {
			if x == nil || x.Kind != "cylinder" {
				return geom.CylinderSurface{}, geom.ErrInvalid
			}
			f, e := x.Frame.decode()
			if e != nil {
				return geom.CylinderSurface{}, e
			}
			return geom.NewCylinderSurface(f, x.A)
		}
		if d.U0 == nil || !d.U0.valid() {
			return nil, geom.ErrInvalid
		}
		a, e := cylinder(d.Base)
		if e != nil {
			return nil, e
		}
		if d.Kind == "reparameterized-cylinder" {
			return reparameterizedCylinder{a, *d.U0}, nil
		}
		b, e := cylinder(d.Other)
		if e != nil || d.U1 == nil || !d.U1.valid() || !finite(d.A) {
			return nil, geom.ErrInvalid
		}
		return ruledArcs{a, b, *d.U0, *d.U1, d.A}, nil
	}

	if d.Kind == "cut-cylinder" || d.Kind == "slanted-fan" {
		r, e := d.Rim.decode()
		if e != nil {
			return nil, e
		}
		if d.Kind == "cut-cylinder" {
			if !finite(d.A) {
				return nil, geom.ErrInvalid
			}
			return cutCylinder{r, d.A, d.CutTop}, nil
		}
		if d.Center == nil || !d.Center.IsFinite() {
			return nil, geom.ErrInvalid
		}
		return slantedFan{r, *d.Center}, nil
	}

	if d.Base != nil || d.Transform != nil {
		return nil, geom.ErrInvalid
	}
	if d.Kind == "steinmetz-exterior-cylinder" {
		if d.Center == nil || d.Steinmetz == nil || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		s := steinmetzExteriorCylinder{*d.Center, d.A, d.B, d.Steinmetz[0], d.Steinmetz[1]}
		if !s.valid() {
			return nil, geom.ErrInvalid
		}
		return s, nil
	}
	if d.Kind == "steinmetz-dual-cap" {
		if d.Center == nil || d.SteinmetzDualCap == nil || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		x := d.SteinmetzDualCap
		w := [2][2]float64{{-d.A, x.Caps[0]}, {-d.A, x.Caps[1]}}
		c := [2][2]bool{{false, x.Fixed[0]}, {false, x.Fixed[1]}}
		if x.Windows != nil || x.Clipped != nil {
			if x.Windows == nil || x.Clipped == nil || x.Caps != ([2]float64{}) || x.Fixed != ([2]bool{}) {
				return nil, geom.ErrInvalid
			}
			w, c = *x.Windows, *x.Clipped
		}
		s := steinmetzDualCapChart{center: *d.Center, radius: d.A, large: x.Large, wideAxis: x.WideAxis, caps: w, fixed: c, axis: x.Axis, side: x.Side}
		if !s.valid() {
			return nil, geom.ErrInvalid
		}
		return s, nil
	}
	if d.Kind == "steinmetz-slab" {
		if d.Center == nil || d.SteinmetzSlab == nil || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		x := d.SteinmetzSlab
		s := steinmetzSlabChart{*d.Center, d.A, x.Lower, x.Upper, x.LowFixed, x.HighFixed, x.Kind}
		if !s.valid() {
			return nil, geom.ErrInvalid
		}
		return s, nil
	}
	if d.Kind == "steinmetz-clipped-cylinder" {
		if d.Center == nil || !d.Center.IsFinite() || !finite(d.A) || !finite(d.B) || d.B <= -d.A || d.B >= d.A || d.Steinmetz == nil || d.Steinmetz[0] != 0 || (d.Steinmetz[1] != 1 && d.Steinmetz[1] != -1) || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		return steinmetzClippedCylinder{*d.Center, d.A, d.B, d.Steinmetz[1]}, nil
	}
	if d.Kind == "steinmetz-disk" {
		if d.Center == nil || !d.Center.IsFinite() || !finite(d.A) || d.A <= 0 || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		return steinmetzDisk{*d.Center, d.A}, nil
	}
	if d.Kind == "steinmetz-cylinder" {
		if d.Center == nil || !d.Center.IsFinite() || !finite(d.A) || d.A <= 0 || d.Steinmetz == nil || d.Steinmetz[0] < 0 || d.Steinmetz[0] > 1 || (d.Steinmetz[1] != 1 && d.Steinmetz[1] != -1) || d.Frame != nil {
			return nil, geom.ErrInvalid
		}
		return steinmetzChart{*d.Center, d.A, d.Steinmetz[0], d.Steinmetz[1]}, nil
	}
	if d.Kind == "bspline" || d.Kind == "wrapped-surface" {
		x := d.Spline
		if x == nil || d.Frame != nil || d.Meridian != nil || len(x.Poles) > *budget {
			return nil, fmt.Errorf("parametric: invalid spline surface or control point limit")
		}
		*budget -= len(x.Poles)
		if d.Kind == "wrapped-surface" {
			if d.Wrap == nil || len(x.Poles) > 4096 || x.DegreeU > 16 || x.DegreeV > 16 {
				return nil, geom.ErrInvalid
			}
			maxWeight := 0.
			for _, w := range x.Weights {
				maxWeight = max(maxWeight, w)
			}
			// Native lossless export writes a normalized net. Reject a
			// malicious recipe whose normalization would change its ratios.
			if len(x.Weights) != 0 && maxWeight != 1 {
				return nil, geom.ErrUnrepresentable
			}
		}
		p, e := geom.NewBSplineSurface(x.DegreeU, x.DegreeV, x.CountU, x.CountV, x.Poles, x.KnotsU, x.KnotsV, x.Weights)
		if e != nil || d.Kind == "bspline" {
			return p, e
		}
		q, e := geom.NewSurfacePatch(p, geom.SurfacePatchOptions{})
		if e != nil {
			return nil, e
		}
		w, _, e := geom.NewWrappedSurfacePatch(context.Background(), q, geom.WrappedSurfaceOptions{
			WrapU: d.Wrap.U, WrapV: d.Wrap.V, ContinuityOrder: d.Wrap.ContinuityOrder,
		})
		return w, e
	}
	if d.Spline != nil || d.Kind != "revolved" && d.Kind != "elliptic" && d.Kind != "helical-profile" && d.Meridian != nil {
		return nil, fmt.Errorf("parametric: unexpected surface data")
	}
	f, e := d.Frame.decode()
	if e != nil {
		return nil, e
	}
	switch d.Kind {
	case "plane":
		return geom.NewPlaneSurface(f)
	case "cylinder":
		return geom.NewCylinderSurface(f, d.A)
	case "twisted-cylinder":
		if d.Twisted == nil {
			return nil, geom.ErrInvalid
		}
		s, e := geom.NewTwistedCylinderSurface(f, d.A, d.Twisted[0], d.Twisted[1])
		if d.AngularReversed {
			s = s.ReversedAngular()
		}
		return s, e
	case "twisted-annulus":
		if d.Twisted == nil {
			return nil, geom.ErrInvalid
		}
		return geom.NewTwistedAnnularSurface(f, d.A, d.B, d.Twisted[0], d.Twisted[1])
	case "cone":
		return geom.NewConeSurfaceSlope(f, d.A)
	case "sphere":
		return geom.NewSphereSurface(f, d.A)
	case "offset-sphere-bore":
		x := d.OffsetBore
		s := offsetBoreChart{f, x.Radius, x.Bore, x.Offset, x.Floor, x.Chart}
		if x.Chart >= toolBoreLower {
			return newToolBoreChart(context.Background(), s, 0)
		}
		if e := s.validate(0); e != nil {
			return nil, e
		}
		return s, nil
	case "offset-sphere-common":
		x := d.OffsetCommon
		if x.Chart < offsetCommonWall || x.Chart > offsetCommonUpper {
			return nil, geom.ErrInvalid
		}
		s, e := newOffsetCommonChart(context.Background(), f, x.Radius, x.Tool, x.Offset, x.Lower, x.Upper, 0)
		s.kind = x.Chart
		return s, e
	case "offset-sphere-partial-common":
		x := d.PartialCommon
		if x.Chart < partialCommonWall || x.Chart > partialCommonStrip || x.Chart == partialCommonStrip && !x.Clipped {
			return nil, geom.ErrInvalid
		}
		s, _, e := newOffsetPartialCommonMode(context.Background(), f, x.Radius, x.Tool, x.Offset, x.Lower, x.Upper, 0, x.ChordMode)
		s.kind, s.clipped = x.Chart, x.Clipped
		return s, e
	case "offset-sphere-double-common":
		x := d.DoubleCommon
		if x.Lower >= 0 || x.Upper <= 0 || x.Chart < doubleWall || x.Chart > doubleUpperStrip || x.Chart == doubleLowerStrip && !x.Clipped[0] || x.Chart == doubleUpperStrip && !x.Clipped[1] {
			return nil, geom.ErrInvalid
		}
		s, _, e := newOffsetDoubleCommon(context.Background(), f, x.Radius, x.Tool, x.Offset, x.Lower, x.Upper, 0)
		s.kind, s.clipped = x.Chart, x.Clipped
		return s, e
	case "offset-sphere-same-side-common":
		x := d.SameSideCommon
		if x.Chart < sameSideWall || x.Chart > sameSideStrip {
			return nil, geom.ErrInvalid
		}
		s, _, e := newOffsetSameSideCommon(context.Background(), f, x.Radius, x.Tool, x.Offset, x.Lower, x.Upper, 0)
		if e != nil {
			return nil, e
		}
		if x.Chart == sameSideWall && x.Clipped[s.near] || x.Chart == sameSideStrip && !x.Clipped[s.far] {
			return nil, geom.ErrInvalid
		}
		s.kind, s.clipped = x.Chart, x.Clipped
		return s, nil
	case "torus":
		return geom.NewTorusSurface(f, d.A, d.B)
	case "arc-fan":
		if d.Center == nil || !d.Center.IsFinite() || !finite(d.B) {
			return nil, geom.ErrInvalid
		}
		s, e := geom.NewCylinderSurface(f, d.A)
		if e != nil {
			return nil, e
		}
		return arcFan{s, *d.Center, d.B}, nil

	case "disk":
		return disk{f}, nil
	case "elliptic":
		if d.Meridian == nil || d.A < 0 || !finite(d.A) {
			return nil, geom.ErrInvalid
		}
		a, b := d.Meridian[0], d.Meridian[1]
		if b.X <= 0 || b.Y <= 0 || a.X < 0 || a.Y < 0 || !finite(a.X) || !finite(a.Y) || !finite(b.X) || !finite(b.Y) ||
			d.A == 0 && (a.X != 0 || a.Y != 0) || d.A > 0 && (a.X == 0 || a.Y == 0) {
			return nil, geom.ErrInvalid
		}
		return ellipticPatch{f, a, b, d.A}, nil
	case "elliptic-band":
		if d.Band == nil {
			return nil, geom.ErrInvalid
		}
		s := ellipticBand{f, d.A, d.B, d.Band[0], d.Band[1]}
		if !s.valid() {
			return nil, geom.ErrInvalid
		}
		return s, nil
	case "revolved":
		if d.Meridian == nil {
			return nil, fmt.Errorf("parametric: missing meridian")
		}
		a, b := d.Meridian[0], d.Meridian[1]
		if a.X < 0 || b.X < 0 || !finite(a.X) || !finite(a.Y) || !finite(b.X) || !finite(b.Y) {
			return nil, geom.ErrInvalid
		}
		return revolvedSide{f, a, b}, nil
	case "helical-profile":
		if d.Meridian == nil || d.Helical == nil {
			return nil, geom.ErrInvalid
		}
		x := d.Helical
		return geom.NewHelicalProfileSurface(f, d.Meridian[0], d.Meridian[1], x.Lead, x.Handedness, x.Turns.Min, x.Turns.Max)
	default:
		return nil, fmt.Errorf("parametric: unsupported surface kind %q", d.Kind)
	}
}

type curveData struct {
	Steinmetz         *[3]int      `json:",omitempty"`
	Transform         *geom.Affine `json:",omitempty"`
	Base              *curveData   `json:",omitempty"`
	Kind              string
	Origin, Direction *geom.Vec3               `json:",omitempty"`
	Frame             *frameData               `json:",omitempty"`
	A, B              float64                  `json:",omitempty"`
	Branch            int                      `json:",omitempty"`
	Degree            int                      `json:",omitempty"`
	Poles             []geom.Vec3              `json:",omitempty"`
	Knots, Weights    []float64                `json:",omitempty"`
	Surface           *surfaceData             `json:",omitempty"`
	U, V              *Interval                `json:",omitempty"`
	Side              int                      `json:",omitempty"`
	Start, Period     float64                  `json:",omitempty"`
	Seam              *geom.ClosedCurveOptions `json:",omitempty"`
}

func encodeCurve(c geom.Curve) (*curveData, error) {
	switch x := c.(type) {
	case SteinmetzSeam:
		if !x.valid() {
			return nil, geom.ErrInvalid
		}
		rev := 0
		if x.reversed {
			rev = 1
		}
		return &curveData{Kind: "steinmetz-seam", Origin: &x.center, A: x.radius, Steinmetz: &[3]int{x.sx, x.sz, rev}}, nil
	case mappedCurve:
		base, e := encodeCurve(x.curve)
		if e != nil {
			return nil, e
		}
		return &curveData{Kind: "mapped", Transform: &x.transform, Base: base}, nil
	case nil:
		return nil, nil
	case geom.Line:
		o, v := x.Origin(), x.Direction()
		return &curveData{Kind: "line", Origin: &o, Direction: &v}, nil
	case geom.Ellipse:
		f, a, b := x.Parameters()
		return &curveData{Kind: "ellipse", Frame: frameToData(f), A: a, B: b}, nil
	case geom.Parabola:
		f, a := x.Parameters()
		return &curveData{Kind: "parabola", Frame: frameToData(f), A: a}, nil
	case geom.Hyperbola:
		f, a, b, sign := x.Parameters()
		return &curveData{Kind: "hyperbola", Frame: frameToData(f), A: a, B: b, Branch: sign}, nil
	case *geom.WrappedBSpline:
		p := x.FundamentalCurve()
		if p == nil {
			return nil, fmt.Errorf("parametric: nil wrapped spline")
		}
		d := x.SeamDiagnostics()
		seam := geom.ClosedCurveOptions{DerivativeOrder: d.DerivativeOrder, PositionTolerance: up(d.PositionResidual), DerivativeTolerance: up(d.DerivativeResidual)}
		return &curveData{Kind: "wrapped-spline", Degree: p.Degree(), Poles: p.Poles(), Knots: p.Knots(), Weights: p.Weights(), Seam: &seam}, nil
	case *geom.PeriodicBSpline:
		if x == nil {
			return nil, fmt.Errorf("parametric: nil periodic spline")
		}
		start, _ := x.Domain()
		return &curveData{Kind: "periodic-spline", Degree: x.Degree(), Poles: x.Poles(), Weights: x.Weights(), Start: start, Period: x.Period()}, nil

	case *geom.BSpline:
		if x == nil {
			return nil, fmt.Errorf("parametric: nil spline")
		}
		return &curveData{Kind: "bspline", Degree: x.Degree(), Poles: x.Poles(), Knots: x.Knots(), Weights: x.Weights()}, nil
	case iso:
		s, e := encodeSurface(x.face.Surface)
		if e != nil {
			return nil, e
		}
		u, v := x.face.U, x.face.V
		return &curveData{Kind: "iso", Surface: &s, U: &u, V: &v, Side: x.side}, nil
	default:
		return nil, fmt.Errorf("parametric: unsupported curve type %T", c)
	}
}
func (d *curveData) decode(budget *int) (geom.Curve, error) {
	if d == nil {
		return nil, nil
	}
	if d.Kind == "mapped" {
		if d.Base == nil || d.Base.Kind == "mapped" || d.Transform == nil {
			return nil, geom.ErrInvalid
		}
		if _, e := d.Transform.Inverse(); e != nil {
			return nil, e
		}
		c, e := d.Base.decode(budget)
		if e != nil {
			return nil, e
		}
		return mappedCurve{c, *d.Transform}, nil
	}
	if d.Base != nil || d.Transform != nil {
		return nil, geom.ErrInvalid
	}
	switch d.Kind {
	case "steinmetz-seam":
		if d.Origin == nil || d.Steinmetz == nil || d.Steinmetz[2] < 0 || d.Steinmetz[2] > 1 {
			return nil, geom.ErrInvalid
		}
		s := SteinmetzSeam{*d.Origin, d.A, d.Steinmetz[0], d.Steinmetz[1], d.Steinmetz[2] == 1}
		if !s.valid() {
			return nil, geom.ErrInvalid
		}
		return s, nil
	case "line":
		if d.Origin == nil || d.Direction == nil {
			return nil, geom.ErrInvalid
		}
		return geom.NewLine(*d.Origin, *d.Direction)
	case "ellipse":
		f, e := d.Frame.decode()
		if e != nil {
			return nil, e
		}
		return geom.NewEllipse(f, d.A, d.B)
	case "parabola":
		f, e := d.Frame.decode()
		if e != nil {
			return nil, e
		}
		return geom.NewParabola(f, d.A)
	case "hyperbola":
		f, e := d.Frame.decode()
		if e != nil {
			return nil, e
		}
		return geom.NewHyperbolaBranch(f, d.A, d.B, d.Branch)
	case "periodic-spline":
		if len(d.Poles) > *budget {
			return nil, fmt.Errorf("parametric: control point limit")
		}
		*budget -= len(d.Poles)
		return geom.NewPeriodicBSpline(d.Degree, d.Poles, d.Weights, d.Start, d.Period)
	case "wrapped-spline":
		if d.Seam == nil || len(d.Poles) > *budget {
			return nil, geom.ErrInvalid
		}
		*budget -= len(d.Poles)
		p, e := geom.NewBSpline(d.Degree, d.Poles, d.Knots, d.Weights)
		if e != nil {
			return nil, e
		}
		r, _, e := geom.NewWrappedBSpline(p, *d.Seam)
		return r, e

	case "bspline":
		if len(d.Poles) > *budget {
			return nil, fmt.Errorf("parametric: control point limit")
		}
		*budget -= len(d.Poles)
		return geom.NewBSpline(d.Degree, d.Poles, d.Knots, d.Weights)
	case "iso":
		if d.Surface == nil || d.U == nil || d.V == nil || !d.U.valid() || !d.V.valid() || d.Side < 0 || d.Side > 3 {
			return nil, geom.ErrInvalid
		}
		s, e := d.Surface.decode(budget)
		if e != nil {
			return nil, e
		}
		return iso{Face{Surface: s, U: *d.U, V: *d.V}, d.Side}, nil
	default:
		return nil, fmt.Errorf("parametric: unsupported curve kind %q", d.Kind)
	}
}

type persistedEdge struct {
	Start, End int
	Curve      *curveData
	Range      Interval
}
type persistedFace struct {
	Surface  surfaceData
	U, V     Interval
	Boundary []Use
	Reversed bool
}
type persistedSolid struct {
	Version   int             `json:"version"`
	Kind      string          `json:"kind"`
	Tolerance float64         `json:"tolerance"`
	Vertices  []geom.Vec3     `json:"vertices"`
	Edges     []persistedEdge `json:"edges"`
	Faces     []persistedFace `json:"faces"`
}

// WriteJSON preserves topology and model geometry in version 1 native JSON.
// Supported evaluators are built-in analytic and spline types and generated
// primitive sides. Custom evaluators return an error.
func WriteJSON(w io.Writer, s Solid) error {
	if w == nil {
		return fmt.Errorf("parametric: nil writer")
	}
	d := persistedSolid{Version: 1, Kind: "parametric-brep", Tolerance: s.tolerance, Vertices: s.vertices}
	for _, e := range s.edges {
		curve, err := encodeCurve(e.Curve)
		if err != nil {
			return err
		}
		d.Edges = append(d.Edges, persistedEdge{e.Start, e.End, curve, e.Range})
	}
	for _, f := range s.faces {
		surface, err := encodeSurface(f.Surface)
		if err != nil {
			return err
		}
		d.Faces = append(d.Faces, persistedFace{surface, f.U, f.V, append([]Use(nil), f.Boundary[:]...), f.Reversed})
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

// ReadJSON validates both geometric data and the reconstructed boundary. Zero
// limits default to 32 MiB, 10,000 entities and 100,000 control points.
func ReadJSON(r io.Reader, l JSONLimits) (Solid, error) {
	if r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxEntities < 0 || l.MaxControlPoints < 0 {
		return Solid{}, fmt.Errorf("parametric: invalid input or limits")
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 10000
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 100000
	}
	b, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return Solid{}, err
	}
	if int64(len(b)) > l.MaxBytes {
		return Solid{}, fmt.Errorf("parametric: JSON byte limit exceeded")
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var d persistedSolid
	if err = decoder.Decode(&d); err != nil {
		return Solid{}, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return Solid{}, fmt.Errorf("parametric: trailing JSON data")
	}
	if d.Version != 1 || d.Kind != "parametric-brep" {
		return Solid{}, fmt.Errorf("parametric: unsupported JSON schema")
	}
	if len(d.Vertices)+len(d.Edges)+len(d.Faces) > l.MaxEntities {
		return Solid{}, fmt.Errorf("parametric: entity limit exceeded")
	}
	budget := l.MaxControlPoints
	e, f := make([]Edge, len(d.Edges)), make([]Face, len(d.Faces))
	for i, x := range d.Edges {
		c, err := x.Curve.decode(&budget)
		if err != nil {
			return Solid{}, fmt.Errorf("parametric: edge %d: %w", i, err)
		}
		e[i] = Edge{x.Start, x.End, c, x.Range}
	}
	for i, x := range d.Faces {
		s, err := x.Surface.decode(&budget)
		if err != nil {
			return Solid{}, fmt.Errorf("parametric: face %d: %w", i, err)
		}
		if len(x.Boundary) != 4 {
			return Solid{}, fmt.Errorf("parametric: invalid boundary size")
		}
		f[i] = Face{s, x.U, x.V, [4]Use(x.Boundary), x.Reversed}
	}
	return NewSolid(d.Vertices, e, f, d.Tolerance)
}

// EncodeSurface encodes a supported surface using the native geometry schema.
// It is used by boundary representations that store geometry separately.
func EncodeSurface(s geom.Surface) (json.RawMessage, error) {
	d, e := encodeSurface(s)
	if e != nil {
		return nil, e
	}
	return json.Marshal(d)
}

// DecodeSurface validates native geometry. maxControlPoints must be positive.
func DecodeSurface(raw json.RawMessage, maxControlPoints int) (geom.Surface, error) {
	if maxControlPoints <= 0 {
		return nil, geom.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var s surfaceData
	if e := d.Decode(&s); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("parametric: trailing surface data")
	}
	return s.decode(&maxControlPoints)
}

func EncodeCurve(c geom.Curve) (json.RawMessage, error) {
	d, e := encodeCurve(c)
	if e != nil {
		return nil, e
	}
	return json.Marshal(d)
}
func DecodeCurve(raw json.RawMessage, maxControlPoints int) (geom.Curve, error) {
	if maxControlPoints <= 0 {
		return nil, geom.ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var c *curveData
	if e := d.Decode(&c); e != nil {
		return nil, e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("parametric: trailing curve data")
	}
	return c.decode(&maxControlPoints)
}
