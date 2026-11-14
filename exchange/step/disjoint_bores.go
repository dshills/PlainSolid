package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"sort"
)

// DisjointBore is a full through bore along local Axis (0=X,1=Y,2=Z).
// Center is on the corresponding lower stock cap: its axial coordinate is zero.
type DisjointBore struct {
	Axis   int
	Center geom.Vec3
	Radius float64
}

// DisjointBlindBore retains a finite entry-open bore. Bore.Center remains on
// the lower stock cap; OppositeCap selects entry from the upper cap.
type DisjointBlindBore struct {
	Bore        DisjointBore
	Depth       float64
	OppositeCap bool
}

// DisjointBoresBody supports 2..32 sites using at least two different local
// axes in a signed-cardinal stock frame. Each pair must pass the padded
// bounds or exact cardinal radial separation proof. Dimensions define
// [0,Dimensions], with every circular opening strictly interior.
type DisjointBoresBody struct {
	Name       string
	Frame      geom.Frame
	Dimensions geom.Vec3
	Bores      []DisjointBore
	Blind      []DisjointBlindBore
}
type DisjointBoresModel struct {
	Body       DisjointBoresBody
	LengthUnit float64
}

func sortedDisjointBores(in []DisjointBore) []DisjointBore {
	out := append([]DisjointBore(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Axis != b.Axis {
			return a.Axis < b.Axis
		}
		if a.Center.X != b.Center.X {
			return a.Center.X < b.Center.X
		}
		if a.Center.Y != b.Center.Y {
			return a.Center.Y < b.Center.Y
		}
		if a.Center.Z != b.Center.Z {
			return a.Center.Z < b.Center.Z
		}
		return a.Radius < b.Radius
	})
	return out
}
func (p DisjointBoresBody) Solid(ctx context.Context) (trimmed.Solid, error) {
	if ctx == nil || len(p.Bores)+len(p.Blind) < 2 || len(p.Bores)+len(p.Blind) > 32 || !p.Dimensions.IsFinite() {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return trimmed.Solid{}, e
	}
	x, y, z, o := p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis(), p.Frame.Origin()
	for _, v := range []geom.Vec3{x, y, z} {
		n := 0
		for _, c := range []float64{v.X, v.Y, v.Z} {
			if c != 0 {
				if c != 1 && c != -1 {
					return trimmed.Solid{}, geom.ErrDomain
				}
				n++
			}
		}
		if n != 1 {
			return trimmed.Solid{}, geom.ErrDomain
		}
	}
	base, e := brep.Box(geom.Vec3{}, p.Dimensions)
	if e != nil {
		return trimmed.Solid{}, e
	}
	base, e = base.Transformed(geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}})
	if e != nil {
		return trimmed.Solid{}, e
	}
	axes := []geom.Vec3{x, y, z}
	caps := [][2]int{{5, 3}, {2, 4}, {0, 1}}
	mask := 0
	var cuts []trimmed.CylinderCut
	for _, site := range p.Blind {
		if !finiteNumber(site.Depth) || site.Depth <= 1e-9 {
			return trimmed.Solid{}, geom.ErrDomain
		}
	}
	sites := disjointSites(p)
	for _, site := range sites {
		b := site.Bore
		if b.Axis < 0 || b.Axis > 2 || !b.Center.IsFinite() {
			return trimmed.Solid{}, geom.ErrInvalid
		}
		if []float64{b.Center.X, b.Center.Y, b.Center.Z}[b.Axis] != 0 {
			return trimmed.Solid{}, geom.ErrDomain
		}
		mask |= 1 << b.Axis
		entry, exit := caps[b.Axis][0], caps[b.Axis][1]
		center, dir := b.Center, axes[b.Axis]
		if site.Depth != 0 {
			length := []float64{p.Dimensions.X, p.Dimensions.Y, p.Dimensions.Z}[b.Axis]
			if !finiteNumber(site.Depth) || site.Depth <= 1e-9 || site.Depth >= length-1e-9 {
				return trimmed.Solid{}, geom.ErrDomain
			}
			if site.OppositeCap {
				entry, exit = exit, entry
				center = center.Add([]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}[b.Axis].Scale(length))
				dir = dir.Scale(-1)
			}
		}
		cuts = append(cuts, trimmed.CylinderCut{Entry: entry, Exit: exit, AxisPoint: p.Frame.ToWorld(center), Direction: dir, Radius: b.Radius})
	}
	if mask&(mask-1) == 0 {
		return trimmed.Solid{}, geom.ErrDomain
	}
	solid, history, e := trimmed.BoreCylinders(base, cuts, 1e-9)
	finiteStopped := false
	if e != nil && len(p.Blind) > 0 {
		depths := make([]float64, len(sites))
		for i, site := range sites {
			depths[i] = site.Depth
		}
		solid, history, _, e = trimmed.BoreFiniteCylinders(base, cuts, depths, 1e-9)
		finiteStopped = e == nil
	}
	if e != nil {
		return trimmed.Solid{}, e
	}
	// Perpendicular cap trims are affine chart lines, despite carrying the
	// general oblique-bore EllipseUV representation. Prove that reduction before
	// using the canonical line/circle writer; no curved trim is approximated.
	faces := solid.Faces()
	for i := range faces {
		if _, ok := faces[i].Surface.(geom.CylinderSurface); !ok {
			continue
		}
		for j := range faces[i].Outer {
			u := &faces[i].Outer[j]
			if u.Curve == nil {
				continue
			}
			c, ok := u.Curve.(trimmed.EllipseUV)
			if !ok || c.Cos != (geom.Vec2{}) || c.Sin != (geom.Vec2{}) || c.Linear != (geom.Vec2{X: 1}) {
				return trimmed.Solid{}, geom.ErrDomain
			}
			u.Curve = nil
		}
	}
	solid, e = trimmed.NewSolid(solid.Vertices(), solid.Edges(), faces, 1e-9)
	if e != nil {
		return trimmed.Solid{}, e
	}
	for i, site := range sites {
		if site.Depth != 0 && !finiteStopped {
			length := []float64{p.Dimensions.X, p.Dimensions.Y, p.Dimensions.Z}[site.Bore.Axis]
			solid, _, e = trimmed.StopPerpendicularBore(solid, history[i], cuts[i].Direction, length-site.Depth)
			if e != nil {
				return trimmed.Solid{}, e
			}
		}
	}
	return solid, nil
}
func disjointBoreLimits() nativeLongEdgeLimits { return nativeLongEdgeLimits{166, 396, 264, 33, 4} }
func WriteDisjointBores(w io.Writer, m DisjointBoresModel) error {
	if w == nil {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	if e := writeDisjointBores(&out, m); e != nil {
		return e
	}
	if _, e := ReadDisjointBores(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		return e
	}
	_, e := io.Copy(w, &out)
	return e
}
func writeDisjointBores(w io.Writer, m DisjointBoresModel) error {
	s, e := m.Body.Solid(context.Background())
	if e != nil {
		return e
	}
	return writeNativeLongEdgeTopologyBounded(w, m.LengthUnit, m.Body.Name, s, disjointBoreLimits())
}
func ReadDisjointBores(r io.Reader, limits Limits) (DisjointBoresModel, error) {
	f, rep, unit, ids, e := parseCanonicalCircularExtrusions(r, limits)
	if e != nil || len(ids) != 1 {
		return DisjointBoresModel{}, circularSourceFailure()
	}
	p, e := recoverDisjointBores(f, ids[0])
	if e != nil {
		return DisjointBoresModel{}, e
	}
	m := DisjointBoresModel{p, unit}
	var out bytes.Buffer
	if e = writeDisjointBores(&out, m); e != nil {
		return DisjointBoresModel{}, e
	}
	if e = verifyCanonicalCircularExtrusionGraph(f, rep, &out); e != nil {
		return DisjointBoresModel{}, e
	}
	return m, nil
}
func recoverDisjointBores(f *File, id int64) (DisjointBoresBody, error) {
	fail := func() (DisjointBoresBody, error) { return DisjointBoresBody{}, circularSourceFailure() }
	lim := disjointBoreLimits()
	g, e := readCircularGraphBounded(f, id, true, lim.faces, lim.loops, lim.uses)
	if e != nil || len(g.faces) < 14 {
		return fail()
	}
	for _, face := range g.faces[:6] {
		if face.kind != "PLANE" || !face.sense || len(face.loops[0]) != 4 {
			return fail()
		}
	}
	frame, e := geom.NewFrame(g.faces[0].frame.Origin(), g.faces[3].frame.ZAxis(), g.faces[1].frame.ZAxis())
	if e != nil {
		return fail()
	}
	var vertices []geom.Vec3
	for _, edge := range g.edges {
		for _, id := range []int64{edge.a, edge.b} {
			v, e := analyticVertex(f, id)
			if e != nil {
				return fail()
			}
			vertices = append(vertices, frame.ToLocal(v))
		}
	}
	bounds, e := geom.Bounds(vertices)
	if e != nil || bounds.Min != (geom.Vec3{}) {
		return fail()
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	p := DisjointBoresBody{Name: name, Frame: frame, Dimensions: bounds.Max}
	for i := 6; i < len(g.faces) && g.faces[i].kind == "CYLINDER"; i += 4 {
		face := g.faces[i]
		if face.kind != "CYLINDER" || face.sense {
			return fail()
		}
		axis := -1
		for j, a := range []geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()} {
			if face.frame.ZAxis() == a || face.frame.ZAxis() == a.Scale(-1) {
				axis = j
			}
		}
		if axis < 0 {
			return fail()
		}
		center := frame.ToLocal(face.frame.Origin())
		length := []float64{p.Dimensions.X, p.Dimensions.Y, p.Dimensions.Z}[axis]
		opposite := face.frame.ZAxis().Dot([]geom.Vec3{frame.XAxis(), frame.YAxis(), frame.ZAxis()}[axis]) < 0
		want := 0.
		if opposite {
			want = length
		}
		if []float64{center.X, center.Y, center.Z}[axis] != want {
			return fail()
		}
		if axis == 0 {
			center.X = 0
		} else if axis == 1 {
			center.Y = 0
		} else {
			center.Z = 0
		}
		depth := 0.
		for _, loop := range face.loops {
			for _, use := range loop {
				edge := g.edges[use.id]
				for _, vid := range []int64{edge.a, edge.b} {
					point, err := analyticVertex(f, vid)
					if err != nil {
						return fail()
					}
					depth = max(depth, point.Sub(face.frame.Origin()).Dot(face.frame.ZAxis()))
				}
			}
		}
		bore := DisjointBore{axis, center, face.radius}
		if depth == length && !opposite {
			p.Bores = append(p.Bores, bore)
		} else {
			p.Blind = append(p.Blind, DisjointBlindBore{bore, depth, opposite})
		}
	}
	p.Bores = sortedDisjointBores(p.Bores)
	return p, nil
}

func disjointSites(p DisjointBoresBody) []DisjointBlindBore {
	out := make([]DisjointBlindBore, 0, len(p.Bores)+len(p.Blind))
	for _, b := range p.Bores {
		out = append(out, DisjointBlindBore{Bore: b})
	}
	out = append(out, p.Blind...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Bore, out[j].Bore
		if a.Axis != b.Axis {
			return a.Axis < b.Axis
		}
		if a.Center.X != b.Center.X {
			return a.Center.X < b.Center.X
		}
		if a.Center.Y != b.Center.Y {
			return a.Center.Y < b.Center.Y
		}
		if a.Center.Z != b.Center.Z {
			return a.Center.Z < b.Center.Z
		}
		return a.Radius < b.Radius
	})
	return out
}
