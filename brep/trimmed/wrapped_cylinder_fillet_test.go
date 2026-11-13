package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactWrappedCylinderRimFillet(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := FilletWrappedCylinderRims(ctx, stock, .5, 1, o)
	if e != nil || s.Topology().Faces != 5 || s.Topology().EulerCharacteristic != 2 || h.Blends != ([2]int{1, 2}) {
		t.Fatal(h, s.Topology(), e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	// Integrate the cylindrical middle plus the two fillet meridians
	// rho(z)=R-r+sqrt(r²-z²), z measured from the lateral tangency.
	volume, area := 9*3.5*math.Pi, 2*math.Pi*3*3.5
	for _, r := range []float64{.5, 1} {
		major := 3 - r
		volume += math.Pi * (major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
		area += math.Pi*major*major + 2*math.Pi*r*(major*math.Pi/2+r)
	}
	if e != nil || math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatal(p, volume, area, e)
	}
	// Verify the blend meridian at a nonbinary parameter lies at the
	// precise rolling-circle radius. u=0 is the positive-X meridian.
	for i, face := range h.Blends {
		support := s.faces[face].Surface.(*geom.WrappedSurfacePatch)
		tm := [2]*big.Rat{big.NewRat(10, 3), big.NewRat(2, 3)}[i]
		q, e := support.EvaluateRational(ctx, new(big.Rat), tm, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		major := [2]*big.Rat{big.NewRat(5, 2), big.NewRat(2, 1)}[i]
		z := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(4, 1)}[i]
		a, b := new(big.Rat).Sub(q.Point[0], major), new(big.Rat).Sub(q.Point[2], z)
		a.Add(a.Mul(a, a), b.Mul(b, b))
		if a.Cmp([2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 1)}[i]) != 0 {
			t.Fatal("wrong exact rolling radius", a)
		}
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !agreement.Complete {
		t.Fatal(agreement, e)
	}
	proof, e := s.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil || !proof.Complete || proof.Fillets[0].Cmp(big.NewRat(1, 2)) != 0 || proof.Fillets[1].Cmp(big.NewRat(1, 1)) != 0 ||
		proof.Caps != h.Caps || proof.Blends != h.Blends {
		t.Fatal(proof, e)
	}
	for _, c := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{
		{geom.Vec3{X: 3, Z: .5}, parametric.Boundary},
		{geom.Vec3{X: 3, Z: .25}, parametric.Outside},
		{geom.Vec3{X: 2.75, Z: .25}, parametric.Inside},
		{geom.Vec3{X: 2.5}, parametric.Boundary},
		{geom.Vec3{X: 2, Z: 5}, parametric.Boundary},
		{geom.Vec3{X: 2.125, Z: 5}, parametric.Outside},
	} {
		if r, e := s.LocateWrappedCylinderRimFillet(ctx, c.p, o); e != nil || r != c.want {
			t.Fatal(c, r, e)
		}
	}
	if s, _, e := FilletWrappedCylinderRims(ctx, stock, 1.5, 0, o); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
		t.Fatal("expected toroid restriction rejection", e)
	}
}

func TestWrappedCylinderRimFilletFiniteLine(t *testing.T) {
	ctx := context.Background()
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := NewWrappedCylinder(ctx, w, 1e-9, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	s, history, e := FilletWrappedCylinderRims(ctx, stock, .5, 1, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	query := func(p, d geom.Vec3, lo, hi float64) WrappedLoftLineIntersections {
		t.Helper()
		r, e := s.IntersectWrappedCylinderRimFilletLine(ctx, p, d, lo, hi, o)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	check := func(q geom.ToroidLineRoot, want float64) {
		t.Helper()
		l, _ := q.Min.Float64()
		h, _ := q.Max.Float64()
		if l-1e-15 > want || h+1e-15 < want || h-l > 1.01*o.ParameterTolerance {
			t.Fatal(q, want, l, h)
		}
	}
	// The horizontal line crosses both halves of the implicit toroid.
	// Only the two outer rolling-circle contacts belong to the result.
	r := query(geom.Vec3{X: -4, Z: .25}, geom.Vec3{X: 1}, 0, 8)
	if len(r.Hits) != 2 || len(r.Overlaps) != 0 {
		t.Fatal(r)
	}
	rho := 2.5 + math.Sqrt(.25-.0625)
	for i, want := range []float64{4 - rho, 4 + rho} {
		check(r.Hits[i].Parameter, want)
		if r.Hits[i].Parts != geom.CylinderLowerBlend || len(r.Hits[i].Faces) != 1 || r.Hits[i].Faces[0] != history.Blends[0] {
			t.Fatal(r.Hits[i])
		}
	}
	// Axial generator endpoints occur at nonbinary original t. Their
	// lateral overlap is explicit, with both surfaces owning each endpoint.
	r = query(geom.Vec3{X: 3, Z: -1}, geom.Vec3{Z: 3}, 0, 2)
	if len(r.Hits) != 2 || len(r.Overlaps) != 1 || r.Overlaps[0].Face != history.Lateral {
		t.Fatal(r)
	}
	for i, want := range []*big.Rat{big.NewRat(1, 2), big.NewRat(5, 3)} {
		h := r.Hits[i]
		if h.Parameter.Min.Cmp(want) != 0 || h.Parameter.Max.Cmp(want) != 0 ||
			h.Parts != geom.CylinderLateral|[2]geom.CylinderBoundaryPart{geom.CylinderLowerBlend, geom.CylinderUpperBlend}[i] ||
			len(h.Faces) != 2 {
			t.Fatal(h, want)
		}
	}
	// Nonrational cap chord endpoints retain both face owners.
	r = query(geom.Vec3{X: -4, Y: .25}, geom.Vec3{X: 1}, 0, 8)
	if len(r.Hits) != 2 || len(r.Overlaps) != 1 || r.Overlaps[0].Face != history.Caps[0] {
		t.Fatal(r)
	}
	chord := math.Sqrt(2.5*2.5 - .25*.25)
	check(r.Overlaps[0].Start, 4-chord)
	check(r.Overlaps[0].End, 4+chord)
	for _, h := range r.Hits {
		if h.Parts != geom.CylinderLowerCap|geom.CylinderLowerBlend || len(h.Faces) != 2 {
			t.Fatal(h)
		}
	}
	r = query(geom.Vec3{X: -3, Y: 2.5}, geom.Vec3{X: 1}, 0, 7)
	if len(r.Hits) != 1 || r.Hits[0].Parameter.Multiplicity != 2 || len(r.Overlaps) != 0 {
		t.Fatal("tangent at the cap rim", r)
	}
	r = query(geom.Vec3{X: 2.75, Z: 6}, geom.Vec3{Z: -3}, 0, 3)
	if len(r.Hits) != 2 || r.Hits[0].Parts != geom.CylinderUpperBlend || r.Hits[1].Parts != geom.CylinderLowerBlend {
		t.Fatal(r)
	}
	check(r.Hits[0].Parameter, (2-math.Sqrt(1-.75*.75))/3)
	check(r.Hits[1].Parameter, (5.5+math.Sqrt(.25-.25*.25))/3)
}

func TestWrappedCylinderRimFilletDistance(t *testing.T) {
	ctx := context.Background()
	o := geom.FrustumDistanceOptions{SquaredTolerance: 1e-12}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: 1, Y: -2}, 3, 5, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := NewWrappedCylinder(ctx, w, 1e-9, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := FilletWrappedCylinderRims(ctx, stock, .5, 1, o.Surface)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		p    geom.Vec3
		part geom.CylinderBoundaryPart
		id   int
		want float64
	}{
		{geom.Vec3{X: 4, Y: -1.75, Z: .125}, geom.CylinderLowerBlend, h.Blends[0],
			math.Pow(math.Hypot(math.Hypot(3, .25)-2.5, -.375)-.5, 2)},
		{geom.Vec3{X: 3.75, Y: -2, Z: 4.5}, geom.CylinderUpperBlend, h.Blends[1],
			math.Pow(math.Hypot(.75, .5)-1, 2)},
		{geom.Vec3{X: 1, Y: -2, Z: 7}, geom.CylinderUpperCap, h.Caps[1], 4},
		{geom.Vec3{X: 6, Y: -2, Z: 3}, geom.CylinderLateral, h.Lateral, 4},
	} {
		r, face, e := s.BoundWrappedCylinderRimFilletDistance(ctx, c.p, o)
		if e != nil || face != c.id || r.Witness.Part != c.part {
			t.Fatal(c, r, face, e)
		}
		lo, _ := r.Lower.Float64()
		hi, _ := r.Upper.Float64()
		if lo-2e-15 > c.want || hi+2e-15 < c.want || hi-lo > o.SquaredTolerance*1.01 {
			t.Fatal(c, lo, hi)
		}
		w := r.Witness
		f := func(q *big.Rat) float64 {
			v, _ := q.Float64()
			return v
		}
		radius, z := f(w.Radial), f(w.Height)
		if w.ArcNormSquared != nil {
			n := f(w.ArcRadius) / math.Sqrt(f(w.ArcNormSquared))
			radius, z = radius+n*f(w.ArcRadial), z+n*f(w.ArcHeight)
		}
		n := radius / math.Sqrt(f(w.AzimuthSquared))
		v := geom.Vec3{X: f(w.Center[0]) + n*f(w.AzimuthX), Y: f(w.Center[1]) + n*f(w.AzimuthY), Z: z}
		if d := v.Sub(c.p).Dot(v.Sub(c.p)); d > hi+2e-15 {
			t.Fatal("witness exceeds upper bound", c, d, hi)
		}
	}
	q := geom.Vec3{X: 4, Y: -1.75, Z: .125}
	o.MaxBisections = 1
	r, _, e := s.BoundWrappedCylinderRimFilletDistance(ctx, q, o)
	if !errors.Is(e, geom.ErrConvergence) || r.Lower != nil {
		t.Fatal("exhausted precision must not expose an uncertified result", r, e)
	}
}

func TestWrappedCylinderRimFilletHorizontalSplit(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	s, history, e := FilletWrappedCylinderRims(ctx, stock, .5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	section, e := s.SectionWrappedCylinderRimFillet(ctx, .25, o)
	if e != nil || section.Major.Cmp(big.NewRat(5, 2)) != 0 || section.Radical.Cmp(big.NewRat(3, 16)) != 0 ||
		section.Curve != nil || len(section.Faces) != 1 || section.Faces[0] != history.Blends[0] {
		t.Fatal(section, e)
	}
	section, e = s.SectionWrappedCylinderRimFillet(ctx, .5, o)
	if e != nil || section.Curve == nil || len(section.Faces) != 2 || section.Radical.Sign() != 0 ||
		section.Major.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(section, e)
	}
	section, e = s.SectionWrappedCylinderRimFillet(ctx, 0, o)
	if e != nil || section.Curve == nil || len(section.Faces) != 2 || section.Major.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal(section, e)
	}
	lo, hi, h, e := s.SplitWrappedCylinderRimFilletAtZ(ctx, 2, o)
	if e != nil || h.LowerSource[h.LowerCut] != -1 || h.UpperSource[h.UpperCut] != -1 {
		t.Fatal(h, e)
	}
	for i, q := range []Solid{lo, hi} {
		p, e := q.ValidateWrappedCylinderRimFillet(ctx, o)
		if e != nil || p.Blends[i] < 0 || p.Blends[1-i] >= 0 ||
			[2][]int{h.LowerSource, h.UpperSource}[i][p.Blends[i]] != history.Blends[i] {
			t.Fatal(p, h, e)
		}
		m, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		r, z := [2]float64{.5, 1}[i], [2]float64{1.5, 2}[i]
		major := 3 - r
		want := math.Pi * (9*z + major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
		if e != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatal(m, want, e)
		}
	}
	if lo, hi, _, e := s.SplitWrappedCylinderRimFilletAtZ(ctx, .25, o); !errors.Is(e, geom.ErrDomain) || len(lo.faces)+len(hi.faces) != 0 {
		t.Fatal("cut enters a toroidal band", e)
	}
}
