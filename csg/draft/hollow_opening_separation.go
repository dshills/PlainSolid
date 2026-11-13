package draft

import "plainsolid/brep"

// The source is already certified convex and every opening is one complete
// unique support face. A linear support functional attains its extrema over
// a planar polygon at vertices. Strict signs therefore prove separation over
// each WHOLE original opening, not only the offset cavity cap or a sample.
// This also excludes an arbitrarily short shared edge and a single shared
// corner. No normal-angle threshold or floating-point gap is used.
func proveSeparatedHollowOpenings(s brep.Solid, ends [2]int, base Result, work *budget) error {
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for k, face := range ends {
		opposite := base.FacePlanes[base.OldToNew[ends[1-k]]]
		loops := append([][]brep.EdgeUse{faces[face].Boundary}, faces[face].Holes...)
		for _, loop := range loops {
			for _, use := range loop {
				if !work.tick() {
					return work.err
				}
				edge := edges[use.Edge]
				id := edge.Start
				if use.Reversed {
					id = edge.End
				}
				if dot(opposite.Normal, exact(vertices[id])).Cmp(opposite.Offset) >= 0 {
					return ErrUnsupported
				}
			}
		}
	}
	return nil
}
