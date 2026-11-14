package iges

import (
	"math/big"
	"plainsolid/geom"
)

func conical45ReferenceSlots(r topologyRecord) ([]int, error) {
	if r.kind == 194 {
		if len(r.values) != 8 {
			return nil, geom.ErrInvalid
		}
		return []int{1, 2, 5}, nil
	}
	return topologyReferenceSlots(r)
}
func conical45VertexRoles(records map[int]topologyRecord, placed bool) (map[int]int, error) {
	var vertices, shell, tr topologyRecord
	for _, r := range records {
		switch r.kind {
		case 502:
			if vertices.id != 0 {
				return nil, geom.ErrInvalid
			}
			vertices = r
		case 514:
			if shell.id != 0 {
				return nil, geom.ErrInvalid
			}
			shell = r
		case 124:
			if tr.id != 0 {
				return nil, geom.ErrInvalid
			}
			tr = r
		}
	}
	if vertices.id == 0 || len(vertices.values) != 8 || shell.id == 0 || len(shell.values) != 8 {
		return nil, geom.ErrInvalid
	}
	pose := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	if placed {
		if tr.id == 0 || tr.transform != 0 || len(tr.values) != 13 || compositeSourceNumbers(124, tr.values) != nil {
			return nil, geom.ErrInvalid
		}
		var e error
		pose, e = decodeMatrix(tr.values)
		if e != nil || !roofRigid(pose) {
			return nil, geom.ErrInvalid
		}
	} else if tr.id != 0 {
		return nil, geom.ErrInvalid
	}
	expected := make([][3]*big.Rat, 2)
	for k := 0; k < 2; k++ {
		// The already-classified shell order is side, upper cap, lower cap.
		fid, e := intValue(shell.values[4+2*k])
		if e != nil {
			return nil, e
		}
		face := records[fid]
		if face.kind != 510 || len(face.values) < 2 {
			return nil, geom.ErrInvalid
		}
		pid, e := intValue(face.values[1])
		if e != nil {
			return nil, e
		}
		plane := records[pid]
		if plane.kind != 190 || len(plane.values) != 4 {
			return nil, geom.ErrInvalid
		}
		oid, e := intValue(plane.values[1])
		if e != nil {
			return nil, e
		}
		origin := records[oid]
		if origin.kind != 116 || len(origin.values) != 5 {
			return nil, geom.ErrInvalid
		}
		o, e := ellipseSourcePoint(origin, [3]int{1, 2, 3})
		if e != nil {
			return nil, e
		}
		matches := 0
		for _, c := range records {
			if c.kind != 100 {
				continue
			}
			if len(c.values) != 8 || compositeSourceNumbers(110, c.values) != nil {
				return nil, geom.ErrInvalid
			}
			center, e := ellipseSourcePoint(c, [3]int{2, 3, 1})
			if e != nil {
				return nil, e
			}
			if center[0].Cmp(o[0]) != 0 || center[1].Cmp(o[1]) != 0 || center[2].Cmp(o[2]) != 0 {
				continue
			}
			p, e := ellipseSourcePoint(c, [3]int{4, 5, 1})
			if e != nil {
				return nil, e
			}
			end, e := ellipseSourcePoint(c, [3]int{6, 7, 1})
			if e != nil {
				return nil, e
			}
			for j := 0; j < 3; j++ {
				if p[j].Cmp(end[j]) != 0 {
					return nil, geom.ErrInvalid
				}
			}
			if p[0].Cmp(o[0]) <= 0 || p[1].Cmp(o[1]) != 0 {
				return nil, geom.ErrInvalid
			}
			wantTransform := 0
			if placed {
				wantTransform = tr.id
			}
			if c.transform != wantTransform || plane.transform != wantTransform {
				return nil, geom.ErrInvalid
			}
			xyz := [3]float64{}
			for j := 0; j < 3; j++ {
				var exact bool
				xyz[j], exact = p[j].Float64()
				if !exact {
					return nil, geom.ErrUnrepresentable
				}
			}
			world, e := roofMapPoint(pose, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}, false)
			if e != nil {
				return nil, e
			}
			expected[k] = [3]*big.Rat{compositeRat(world.X), compositeRat(world.Y), compositeRat(world.Z)}
			matches++
		}
		if matches != 1 {
			return nil, geom.ErrInvalid
		}
	}
	return conicMatchVertices(records, vertices, expected)
}
