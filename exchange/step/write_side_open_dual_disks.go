package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// SideOpenDualDiskDescriptor is the exact native same-side, equal-radius,
// overlapping two-disc blind/through domain. Cutters start at local z=0 and
// stop at Depth; Depth==Height is through. The constructor's domain is retained.
type SideOpenDualDiskDescriptor struct {
	Name                                 string
	Frame                                geom.Frame
	Min, Max                             geom.Vec2
	Discs                                [2]trimmed.ExtrusionCircle
	Height, Depth, LengthUnit, Tolerance float64
}

// WriteSideOpenDualDiskProduct builds and certifies the native shape before
// exporting its actual sewn topology with direct analytic model curves. No
// rounded angular pcurves, sampled inference or approximate source import is used.
func WriteSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d SideOpenDualDiskDescriptor) error {
	return writeSideOpenDualDiskProduct(ctx, w, d, sideDiskEqual)
}

type sideDiskConstruction uint8

const (
	sideDiskEqual sideDiskConstruction = iota
	sideDiskUnequal
	sideDiskGeneral
)

func writeSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d SideOpenDualDiskDescriptor, kind sideDiskConstruction) error {
	if ctx == nil || w == nil || !utf8.ValidString(d.Name) || len(d.Name) > 1024 {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	si := false
	for power := range unitPrefixes {
		if d.LengthUnit == math.Pow10(power) {
			si = true
		}
	}
	if !si {
		return geom.ErrDomain
	}
	var solid trimmed.Solid
	var wallFaces [2][]int
	var e error
	if kind == sideDiskGeneral {
		built, err := trimmed.CutGeneralSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, d.Tolerance, trimvalidate.Options{})
		if err != nil {
			return err
		}
		solid, wallFaces = built.Solid, built.WallFaces
	} else if kind == sideDiskUnequal {
		built, err := trimmed.CutUnequalSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, d.Tolerance, trimvalidate.Options{})
		if err != nil {
			return err
		}
		solid, wallFaces = built.Solid, built.WallFaces
	} else {
		built, err := trimmed.CutSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, d.Tolerance, trimvalidate.Options{})
		if err != nil {
			return err
		}
		solid, wallFaces = built.Solid, built.WallFaces
	}
	vertices, edges, faces := solid.Vertices(), solid.Edges(), solid.Faces()
	if len(vertices) > 128 || len(edges) > 128 || len(faces) > 32 {
		return geom.ErrDomain
	}
	ownerByFace := map[int]int{}
	for owner, ids := range wallFaces {
		if len(ids) != 1 {
			return geom.ErrDomain
		}
		for _, id := range ids {
			if id < 0 || id >= len(faces) {
				return geom.ErrDomain
			}
			if _, ok := ownerByFace[id]; ok {
				return geom.ErrDomain
			}
			ownerByFace[id] = owner
		}
	}
	surfaces := make([]longEdgeSurface, len(faces))
	owners := make([][]longEdgeOwner, len(edges))
	for i, face := range faces {
		if len(face.Holes) != 0 || len(face.Outer) < 3 {
			return geom.ErrDomain
		}
		if owner, ok := ownerByFace[i]; ok {
			center, e := sideDiskWorld(d.Frame, geom.Vec3{X: d.Discs[owner].Center.X, Y: d.Discs[owner].Center.Y})
			if e != nil {
				return e
			}
			frame, e := geom.NewFrame(center, d.Frame.XAxis(), d.Frame.ZAxis())
			if e != nil {
				return e
			}
			surfaces[i] = longEdgeSurface{kind: "CYLINDRICAL_SURFACE", frame: frame, radius: d.Discs[owner].Radius}
		} else {
			frame, e := sideDiskPlane(face.Surface)
			if e != nil {
				return e
			}
			surfaces[i] = longEdgeSurface{kind: "PLANE", frame: frame}
		}
		for j, u := range face.Outer {
			if u.Edge < 0 || u.Edge >= len(edges) {
				return geom.ErrDomain
			}
			edge := edges[u.Edge]
			if edge.Start < 0 || edge.End < 0 || edge.Start >= len(vertices) || edge.End >= len(vertices) || edge.Start == edge.End {
				return geom.ErrDomain
			}
			next := face.Outer[(j+1)%len(face.Outer)]
			if next.Edge < 0 || next.Edge >= len(edges) {
				return geom.ErrDomain
			}
			end := edge.End
			if u.Reversed {
				end = edge.Start
			}
			start := edges[next.Edge].Start
			if next.Reversed {
				start = edges[next.Edge].End
			}
			if end != start {
				return geom.ErrDomain
			}
			owners[u.Edge] = append(owners[u.Edge], longEdgeOwner{face: i, use: u})
		}
	}
	curves := make([]longEdgeCurve, len(edges))
	for i, edge := range edges {
		if e = ctx.Err(); e != nil {
			return e
		}
		os := owners[i]
		if len(os) != 2 || os[0].face == os[1].face || (os[0].use.Reversed != faces[os[0].face].Reversed) == (os[1].use.Reversed != faces[os[1].face].Reversed) {
			return geom.ErrDomain
		}
		a, b := vertices[edge.Start], vertices[edge.End]
		curve := longEdgeCurve{origin: a, same: true}
		arc := false
		for _, o := range os {
			owner, ok := ownerByFace[o.face]
			if !ok {
				continue
			}
			u := o.use
			if !sideDiskStraightUV(u) {
				return geom.ErrDomain
			}
			if u.UVStart.Y == u.UVEnd.Y && u.UVStart.X != u.UVEnd.X {
				if arc {
					return geom.ErrDomain
				}
				pa, e := sideDiskLocal(d.Frame, a)
				if e != nil {
					return e
				}
				pb, e := sideDiskLocal(d.Frame, b)
				if e != nil || pa.Z != pb.Z {
					return geom.ErrDomain
				}
				disc := d.Discs[owner]
				if !sideDiskOnCircle(pa, disc) || !sideDiskOnCircle(pb, disc) {
					return geom.ErrDomain
				}
				cross := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(sideRat(pa.X), sideRat(disc.Center.X)), new(big.Rat).Sub(sideRat(pb.Y), sideRat(disc.Center.Y))), new(big.Rat).Mul(new(big.Rat).Sub(sideRat(pa.Y), sideRat(disc.Center.Y)), new(big.Rat).Sub(sideRat(pb.X), sideRat(disc.Center.X))))
				if cross.Sign() == 0 {
					return geom.ErrDomain
				}
				same := cross.Sign() > 0
				uForward := u.UVEnd.X > u.UVStart.X
				if u.Reversed {
					uForward = !uForward
				}
				if same == uForward {
					return geom.ErrDomain
				} // native wall U is clockwise
				center, e := sideDiskWorld(d.Frame, geom.Vec3{X: disc.Center.X, Y: disc.Center.Y, Z: pa.Z})
				if e != nil {
					return e
				}
				frame, e := geom.NewFrame(center, d.Frame.XAxis(), d.Frame.ZAxis())
				if e != nil {
					return e
				}
				curve = longEdgeCurve{circle: true, frame: frame, radius: disc.Radius, same: same}
				arc = true
			} else if u.UVStart.X != u.UVEnd.X || u.UVStart.Y == u.UVEnd.Y {
				return geom.ErrDomain
			}
		}
		if !arc {
			direction, e := sideDiskCardinal(a, b)
			if e != nil {
				return e
			}
			curve.direction = direction
		}
		curves[i] = curve
	}
	var raw bytes.Buffer
	var numericErr error
	exact := func(x float64) string {
		q := new(big.Rat).SetFloat64(x)
		if q == nil {
			numericErr = geom.ErrInvalid
			return "0."
		}
		s := q.FloatString(q.Denom().BitLen() - 1)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		if _, err := cylinderSupportNumber(Value{Kind: Number, Text: s}); err != nil {
			numericErr = err
		}
		if !strings.Contains(s, ".") {
			s += "."
		}
		return s
	}
	e = writeStandalone(&raw, d.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", exact(p.X), exact(p.Y), exact(p.Z)))
		}
		direction := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", exact(p.X), exact(p.Y), exact(p.Z)))
		}
		place := func(f geom.Frame) int64 {
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", point(f.Origin()), direction(f.ZAxis()), direction(f.XAxis())))
		}
		surfIDs := make([]int64, len(faces))
		for i, s := range surfaces {
			p := place(s.frame)
			if s.kind == "PLANE" {
				surfIDs[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
			} else {
				surfIDs[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, exact(s.radius)))
			}
		}
		vertexIDs := make([]int64, len(vertices))
		for i, p := range vertices {
			vertexIDs[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(p)))
		}
		edgeIDs := make([]int64, len(edges))
		for i, edge := range edges {
			c := curves[i]
			var model int64
			if c.circle {
				model = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", place(c.frame), exact(c.radius)))
			} else {
				origin, dir := point(c.origin), direction(c.direction)
				vector := emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir))
				model = emit(fmt.Sprintf("LINE('',#%d,#%d)", origin, vector))
			}
			edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,%s)", vertexIDs[edge.Start], vertexIDs[edge.End], model, sideDiskBool(c.same)))
		}
		faceIDs := make([]int64, len(faces))
		for i, face := range faces {
			uses := make([]int64, len(face.Outer))
			for j, u := range face.Outer {
				uses[j] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edgeIDs[u.Edge], sideDiskBool(!u.Reversed)))
			}
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,%s)", loop, sideDiskBool(!face.Reversed)))
			same := !face.Reversed
			if _, ok := ownerByFace[i]; ok {
				same = !same
			}
			faceIDs[i] = emit(fmt.Sprintf("ADVANCED_FACE('',(#%d),#%d,%s)", bound, surfIDs[i], sideDiskBool(same)))
		}
		shell := emit("CLOSED_SHELL(''," + references(faceIDs) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(d.Name), shell))}
	})
	if e != nil {
		return e
	}
	if numericErr != nil {
		return numericErr
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	marker := []byte("PRODUCT('1','geometry','',")
	if bytes.Count(raw.Bytes(), marker) != 1 {
		return geom.ErrInvalid
	}
	data := bytes.Replace(raw.Bytes(), marker, []byte("PRODUCT('1',"+quote(d.Name)+",'',"), 1)
	if _, e = Parse(bytes.NewReader(data), Limits{MaxBytes: 1 << 20, MaxEntities: 2048, MaxValues: 32768, MaxDepth: 32}); e != nil {
		return e
	}
	n, e := w.Write(data)
	if e == nil && n != len(data) {
		e = io.ErrShortWrite
	}
	return e
}
func sideRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
func sideDiskCardinal(a, b geom.Vec3) (geom.Vec3, error) {
	out := [3]float64{}
	n := 0
	for i, x := range []float64{a.X, a.Y, a.Z} {
		y := []float64{b.X, b.Y, b.Z}[i]
		if x != y {
			n++
			out[i] = 1
			if y < x {
				out[i] = -1
			}
		}
	}
	if n != 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
}
func sideDiskWorld(f geom.Frame, p geom.Vec3) (geom.Vec3, error) {
	var out [3]float64
	for j, o := range []float64{f.Origin().X, f.Origin().Y, f.Origin().Z} {
		q := sideRat(o)
		for i, a := range []geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()} {
			q.Add(q, new(big.Rat).Mul(sideRat([]float64{a.X, a.Y, a.Z}[j]), sideRat([]float64{p.X, p.Y, p.Z}[i])))
		}
		v, exact := q.Float64()
		if !exact {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
		out[j] = v
	}
	return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
}
func sideDiskLocal(f geom.Frame, p geom.Vec3) (geom.Vec3, error) {
	var out [3]float64
	for i, a := range []geom.Vec3{f.XAxis(), f.YAxis(), f.ZAxis()} {
		q := new(big.Rat)
		for j, x := range []float64{p.X, p.Y, p.Z} {
			delta := new(big.Rat).Sub(sideRat(x), sideRat([]float64{f.Origin().X, f.Origin().Y, f.Origin().Z}[j]))
			q.Add(q, new(big.Rat).Mul(delta, sideRat([]float64{a.X, a.Y, a.Z}[j])))
		}
		v, exact := q.Float64()
		if !exact {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
		out[i] = v
	}
	return geom.Vec3{X: out[0], Y: out[1], Z: out[2]}, nil
}
func sideDiskOnCircle(p geom.Vec3, d trimmed.ExtrusionCircle) bool {
	x := new(big.Rat).Sub(sideRat(p.X), sideRat(d.Center.X))
	y := new(big.Rat).Sub(sideRat(p.Y), sideRat(d.Center.Y))
	return new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y)).Cmp(new(big.Rat).Mul(sideRat(d.Radius), sideRat(d.Radius))) == 0
}
func sideDiskPlane(s geom.Surface) (geom.Frame, error) {
	b, ok := s.(*geom.BSplineSurface)
	if !ok {
		return geom.Frame{}, geom.ErrDomain
	}
	d := b.Data()
	if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 || len(d.Poles) != 4 {
		return geom.Frame{}, geom.ErrDomain
	}
	for _, w := range d.Weights {
		if w <= 0 || w != d.Weights[0] {
			return geom.Frame{}, geom.ErrDomain
		}
	}
	for i := 0; i < 3; i++ {
		a := []float64{d.Poles[0].X, d.Poles[0].Y, d.Poles[0].Z}[i]
		b := []float64{d.Poles[1].X, d.Poles[1].Y, d.Poles[1].Z}[i]
		c := []float64{d.Poles[2].X, d.Poles[2].Y, d.Poles[2].Z}[i]
		z := []float64{d.Poles[3].X, d.Poles[3].Y, d.Poles[3].Z}[i]
		if new(big.Rat).Add(sideRat(a), sideRat(z)).Cmp(new(big.Rat).Add(sideRat(b), sideRat(c))) != 0 {
			return geom.Frame{}, geom.ErrDomain
		}
	}
	x, e := sideDiskCardinal(d.Poles[0], d.Poles[2])
	if e != nil {
		return geom.Frame{}, e
	}
	y, e := sideDiskCardinal(d.Poles[0], d.Poles[1])
	if e != nil || x.Dot(y) != 0 {
		return geom.Frame{}, geom.ErrDomain
	}
	return geom.NewFrame(d.Poles[0], x, x.Cross(y))
}
func sideDiskStraightUV(u trimmed.Use) bool {
	if u.Curve == nil {
		return true
	}
	q, ok := u.Curve.(trimmed.ExactQuadraticUV)
	if !ok {
		return false
	}
	h := q.HomogeneousControls()
	for _, p := range h {
		if p[2].Sign() <= 0 {
			return false
		}
		if u.UVStart.X == u.UVEnd.X {
			if p[0].Cmp(new(big.Rat).Mul(sideRat(u.UVStart.X), p[2])) != 0 {
				return false
			}
		} else if u.UVStart.Y == u.UVEnd.Y {
			if p[1].Cmp(new(big.Rat).Mul(sideRat(u.UVStart.Y), p[2])) != 0 {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
func sideDiskBool(v bool) string {
	if v {
		return ".T."
	}
	return ".F."
}
