package step

import (
	"bytes"
	"io"
	"plainsolid/brep/trimmed"
)

// ReadDiskHoleExtrusions recovers the canonical circular stock graphs emitted
// by WriteDiskHoleExtrusions, including eccentric bores and blind pockets.
// It revalidates construction and compares all geometric entities, uses,
// orientations, trim curves, and sharing against the regenerated STEP graph.
// The first blind pocket selects the original cap when both caps have entries.
// Without a blind pocket the sign of the authored sweep cannot be recovered;
// such bodies start at the lower cap with positive height. Alternate graph
// orderings and frames and nonrepresentable parameter recovery are unsupported.
func ReadDiskHoleExtrusions(r io.Reader, l Limits) (DiskHoleExtrusionModel, error) {
	f, repID, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	m := DiskHoleExtrusionModel{LengthUnit: unit}
	for _, id := range ids {
		var outer trimmed.ExtrusionCircle
		b, e := recoverCircularExtrusion(f, id, &outer, nil)
		if e != nil {
			return DiskHoleExtrusionModel{}, e
		}
		m.Bodies = append(m.Bodies, DiskHoleExtrusionBody{
			Name: b.Name, Frame: b.Frame, Outer: outer, Holes: b.Holes, Blind: b.Blind, Height: b.Height,
		})
	}
	var out bytes.Buffer
	if e := WriteDiskHoleExtrusions(&out, m); e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	if e := verifyCanonicalCircularExtrusionGraph(f, repID, &out); e != nil {
		return DiskHoleExtrusionModel{}, e
	}
	return m, nil
}
