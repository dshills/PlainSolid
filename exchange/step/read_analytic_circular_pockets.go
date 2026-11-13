package step

import (
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadAnalyticCircularPocketExtrusions extends the analytic through-hole subset
// to blind circular pockets with one planar floor each. All blind pockets must
// open at the same exterior cap; signed height preserves that entry side.
// Continuous geometry uses the same relative 1e-9 tolerance and footprint
// clearance as ReadAnalyticCircularThroughExtrusions. This does not broaden the
// separate exact-decimal ReadCircularHoleExtrusions source contract.
func ReadAnalyticCircularPocketExtrusions(r io.Reader, l Limits) (CircularHoleExtrusionModel, error) {
	return readAnalyticCircularExtrusions(r, l, true)
}

func collectCircularSourcePockets(g circularSourceGraph, cylinders []int, caps [2]int, outerLoops map[int]int, floors map[int]bool, frame geom.Frame, height, tol float64, out *CircularHoleExtrusionBody) error {
	fail := circularSourceFailure
	entry := -1
	usedFloors := map[int]bool{}
	capCounts := [2]int{}
	for _, ci := range cylinders {
		face := g.faces[ci]
		rims := []circularSourceEdge{}
		owners := []int{}
		seam := int64(0)
		seamUses := 0
		for _, u := range face.loops[0] {
			edge := g.edges[u.id]
			if edge.kind == "LINE" {
				if seam != 0 && seam != edge.id {
					return fail()
				}
				seam = edge.id
				seamUses++
				continue
			}
			if math.Abs(edge.radius-face.radius) > tol {
				return fail()
			}
			rims = append(rims, edge)
			os := g.owners[edge.id]
			other := os[0]
			if other == ci {
				other = os[1]
			}
			owners = append(owners, other)
		}
		if len(rims) != 2 || rims[0].id == rims[1].id || seamUses != 2 || g.edges[seam].wrapper != "SEAM_CURVE" {
			return fail()
		}
		centers := [2]geom.Vec3{frame.ToLocal(rims[0].frame.Origin()), frame.ToLocal(rims[1].frame.Origin())}
		if math.Hypot(centers[0].X-centers[1].X, centers[0].Y-centers[1].Y) > tol || rims[0].radius <= 64*tol {
			return fail()
		}
		roles := [2]int{-1, -1}
		floorIndex := -1
		for k, owner := range owners {
			switch owner {
			case caps[0]:
				roles[k] = 0
			case caps[1]:
				roles[k] = 1
			default:
				if !floors[owner] || usedFloors[owner] || floorIndex >= 0 {
					return fail()
				}
				floorIndex = k
			}
		}
		localEntry := -1
		if floorIndex >= 0 {
			localEntry = roles[1-floorIndex]
			if localEntry < 0 || entry >= 0 && entry != localEntry {
				return fail()
			}
			entry = localEntry
			floorZ := centers[floorIndex].Z
			if floorZ <= 64*tol || height-floorZ <= 64*tol {
				return fail()
			}
			usedFloors[owners[floorIndex]] = true
		} else if roles[0] < 0 || roles[1] < 0 || roles[0] == roles[1] {
			return fail()
		}
		for k, owner := range owners {
			ownerFace := g.faces[owner]
			want := frame.ZAxis()
			floor := k == floorIndex
			if floor {
				if localEntry == 0 {
					want = want.Scale(-1)
				}
				normal := ownerFace.frame.ZAxis()
				if !ownerFace.sense {
					normal = normal.Scale(-1)
				}
				if normal.Sub(want).Length() > 1e-9 {
					return fail()
				}
			} else {
				j := roles[k]
				if math.Abs(centers[k].Z-float64(j)*height) > tol {
					return fail()
				}
				capCounts[j]++
				if j == 1 {
					want = want.Scale(-1)
				}
			}
			found := false
			for j, loop := range ownerFace.loops {
				if !floor && j == outerLoops[owner] {
					continue
				}
				if len(loop) != 1 || loop[0].id != rims[k].id {
					continue
				}
				n := rims[k].frame.ZAxis()
				if !loop[0].sense {
					n = n.Scale(-1)
				}
				if n.Sub(want).Length() > 1e-9 {
					return fail()
				}
				found = true
			}
			if !found {
				return fail()
			}
		}
		circle := trimmed.ExtrusionCircle{Center: geom.Vec2{X: centers[0].X, Y: centers[0].Y}, Radius: rims[0].radius}
		if floorIndex < 0 {
			out.Holes = append(out.Holes, circle)
		} else {
			depth := centers[floorIndex].Z
			if localEntry == 1 {
				depth = height - depth
			}
			out.Blind = append(out.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: circle, Depth: depth})
		}
	}
	if len(usedFloors) != len(floors) || len(g.faces[caps[0]].loops) != capCounts[0]+1 || len(g.faces[caps[1]].loops) != capCounts[1]+1 {
		return fail()
	}
	if entry == 1 {
		var e error
		out.Frame, e = geom.NewFrame(frame.ToWorld(geom.Vec3{Z: height}), frame.XAxis(), frame.ZAxis())
		if e != nil {
			return e
		}
		out.Height = -height
	}
	return nil
}
