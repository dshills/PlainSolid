package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func igesPlanarFromVec(v geom.Vec3) igesPlanarVec {
	return igesPlanarVec{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}

func (p igesPlanarVec) text() []string {
	s := make([]string, len(p))
	for i, q := range p {
		if q.IsInt() {
			s[i] = q.Num().String() + "."
		} else {
			// Coordinate and direction arithmetic here uses only dyadic
			// inputs. Their exact decimal expansions terminate.
			s[i] = q.FloatString(q.Denom().BitLen() - 1)
		}
	}
	return s
}

func (p igesPlanarVec) scaledDirection() igesPlanarVec {
	var scale int
	have := false
	for _, q := range p {
		e := q.Num().BitLen() - q.Denom().BitLen()
		if q.Sign() != 0 && (!have || e > scale) {
			scale, have = e, true
		}
	}
	factor := big.NewRat(1, 1)
	if scale >= 0 {
		factor.SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(scale)))
	} else {
		factor.SetInt(new(big.Int).Lsh(big.NewInt(1), uint(-scale)))
	}
	var result igesPlanarVec
	for i := range result {
		result[i] = new(big.Rat).Mul(p[i], factor)
	}
	return result
}

// WritePlanarSolid emits a standard model-only type 186 B-rep with original
// vertices, exact planar supports, and explicit outer and inner wires. The
// supported topology is the same as ReadPlanarSolid. One exterior is written
// before its independently verified voids. The generated IGES is re-read and
// checked for unchanged topology before any bytes are written.
func WritePlanarSolid(ctx context.Context, w io.Writer, s brep.Solid, unit float64, o PlanarSolidOptions) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, expected, err := authorPlanarIGES(ctx, []brep.Solid{s}, unit, o.Limits, o.Validation)
	if err != nil {
		return err
	}
	reopened, err := ReadPlanarSolid(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return fmt.Errorf("iges planar solid readback: %w", err)
	}
	if reopened.LengthUnit != unit || !samePlanarIGESBody(s, reopened.Solid, expected[0]) {
		return igesPlanarFailure(0, "reopen changed original planar topology or units")
	}
	return publishPlanarIGES(ctx, w, raw)
}

func samePlanarIGESBody(original, reopened brep.Solid, faces []brep.Face) bool {
	return reflect.DeepEqual(reopened.Vertices(), original.Vertices()) &&
		reflect.DeepEqual(reopened.Edges(), original.Edges()) && reflect.DeepEqual(reopened.Faces(), faces)
}

func publishPlanarIGES(ctx context.Context, w io.Writer, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

func authorPlanarIGES(ctx context.Context, solids []brep.Solid, unit float64, l Limits, validation collision.LocationOptions) ([]byte, [][]brep.Face, error) {
	opt, err := l.defaults()
	if err != nil {
		return nil, nil, err
	}
	var v []geom.Vec3
	var e []brep.Edge
	var f []brep.Face
	bodyGroups, expectedFaces := make([][][]int, len(solids)), make([][]brep.Face, len(solids))
	entityCount := 2 + len(solids)
	if len(solids) > 1 {
		entityCount++
	}
	for i, s := range solids {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := s.Validate(); err != nil {
			return nil, nil, err
		}
		groups, err := igesPlanarEmbeddedShells(ctx, s, validation)
		if err != nil {
			return nil, nil, err
		}
		vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
		entityCount += len(groups) + len(edges) + 6*len(faces)
		for _, face := range faces {
			entityCount += len(face.Holes)
		}
		if entityCount > opt.MaxEntities {
			return nil, nil, igesPlanarFailure(0, "output entity limit exceeded")
		}
		vo, eo := len(v), len(e)
		v = append(v, vertices...)
		for _, edge := range edges {
			e = append(e, brep.Edge{Start: edge.Start + vo, End: edge.End + vo})
		}
		shift := func(loop []brep.EdgeUse) []brep.EdgeUse {
			uses := make([]brep.EdgeUse, len(loop))
			for j, use := range loop {
				uses[j] = brep.EdgeUse{Edge: use.Edge + eo, Reversed: use.Reversed}
			}
			return uses
		}
		bodyGroups[i] = make([][]int, len(groups))
		for j, group := range groups {
			for _, id := range group {
				face := faces[id]
				expectedFaces[i] = append(expectedFaces[i], face)
				shifted := brep.Face{Boundary: shift(face.Boundary)}
				for _, hole := range face.Holes {
					shifted.Holes = append(shifted.Holes, shift(hole))
				}
				bodyGroups[i][j] = append(bodyGroups[i][j], len(f))
				f = append(f, shifted)
			}
		}
	}
	// The shared fixed-record writer uses one ordinary geometry entity to
	// generate the Global section; this datum is not emitted as a directory entry.
	a := topologyAuthor{geometry: Model{Entities: []Entity{{Point: &geom.Vec3{}}}}, groupedSolids: len(solids) > 1}
	points := make([]igesPlanarVec, len(v))
	vp := []string{"502", fmt.Sprint(len(v))}
	for i, p := range v {
		points[i] = igesPlanarFromVec(p)
		vp = append(vp, points[i].text()...)
	}
	vid := a.add(502, 1, joinTopologyValues(vp))
	ev := []int{504, len(e)}
	for _, edge := range e {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		data := append([]string{"110"}, points[edge.Start].text()...)
		data = append(data, points[edge.End].text()...)
		line := a.add(110, 0, joinTopologyValues(data))
		ev = append(ev, line, vid, edge.Start+1, vid, edge.End+1)
	}
	eid := a.add(504, 1, topologyNumbers(ev...))
	faceIDs := make([]int, len(f))
	startVertex := func(u brep.EdgeUse) int {
		edge := e[u.Edge]
		if u.Reversed {
			return edge.End
		}
		return edge.Start
	}
	for faceIndex, face := range f {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		outer := face.Boundary
		p := points[startVertex(outer[0])]
		x := points[startVertex(outer[1])].sub(p)
		z := igesPlanarFromVec(geom.Vec3{})
		for i := 1; i+1 < len(outer); i++ {
			a := points[startVertex(outer[i])].sub(p)
			b := points[startVertex(outer[i+1])].sub(p)
			n := a.cross(b)
			for j := range z {
				z[j].Add(z[j], n[j])
			}
		}
		pi := a.add(116, 0, joinTopologyValues(append(append([]string{"116"}, p.text()...), "0")))
		zi := a.add(123, 0, joinTopologyValues(append([]string{"123"}, z.scaledDirection().text()...)))
		xi := a.add(123, 0, joinTopologyValues(append([]string{"123"}, x.scaledDirection().text()...)))
		plane := a.add(190, 1, topologyNumbers(190, pi, zi, xi))
		fv := []int{510, plane, 1 + len(face.Holes), 1}
		for _, loop := range append([][]brep.EdgeUse{outer}, face.Holes...) {
			lv := []int{508, len(loop)}
			for _, use := range loop {
				sense := 1
				if use.Reversed {
					sense = 0
				}
				lv = append(lv, 0, eid, use.Edge+1, sense, 0)
			}
			fv = append(fv, a.add(508, 1, topologyNumbers(lv...)))
		}
		faceIDs[faceIndex] = a.add(510, 1, topologyNumbers(fv...))
	}
	roots := make([]int, 0, len(solids))
	for _, groups := range bodyGroups {
		shells := make([]int, len(groups))
		for i, group := range groups {
			sv := []int{514, len(group)}
			for _, j := range group {
				sv = append(sv, faceIDs[j], 1)
			}
			shells[i] = a.add(514, 1, topologyNumbers(sv...))
		}
		root := []int{186, shells[0], 1, len(shells) - 1}
		for _, shell := range shells[1:] {
			root = append(root, shell, 1)
		}
		roots = append(roots, a.add(186, 0, topologyNumbers(root...)))
	}
	if len(roots) > 1 {
		group := append([]int{402, len(roots)}, roots...)
		a.add(402, 1, topologyNumbers(group...))
	}
	raw, err := a.file(unit)
	if err != nil {
		return nil, nil, err
	}
	return raw, expectedFaces, nil
}
