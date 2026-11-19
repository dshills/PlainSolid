package boolean

import (
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
)

// Result is what Combine returns (P5). On an error it is the zero value.
type Result struct {
	// Solid is the regularized result. The zero value is the empty solid.
	Solid trimmed.Solid
	// Shells lists, for each shell of Solid, the indices of its faces.
	Shells [][]int
	// Voids reports for each shell whether it bounds a void.
	Voids []bool
	// Owners gives for each shell the index of the solid it belongs to.
	Owners []int
	// Solids is the number of solids: outer shells, each with its voids.
	Solids int
	// Sources lists, for each face of Solid, the input faces it came
	// from. An entry is {operand, face}, with operand 0 for A and 1 for
	// B, and face an index into that operand's faces. A list has no
	// duplicates and is ordered A before B, then by face index.
	Sources [][][2]int

	// Tolerance is the working tolerance ε of the call (T1).
	Tolerance float64
	// NodeRadius is the largest node radius of the arrangement.
	NodeRadius float64
	// Nodes, Spans, Bundles, Traces and Pieces count the arrangement.
	Nodes, Spans, Bundles, Traces, Pieces int
	// VolumeA, VolumeB and Volume are the integrated volumes of the
	// operands and of the result.
	VolumeA, VolumeB, Volume float64
	// Properties are the mass properties of the result as V5 integrated
	// them: its area, centroid and inertia beside the volume. They are
	// zero for the empty solid.
	Properties parametric.Properties
	// EdgeResidual and VertexResidual are the largest residuals that
	// validation found (V3, V4).
	EdgeResidual, VertexResidual float64
	// Touching lists the touching contacts found. Entry i holds the
	// faces of A and then the faces of B that meet at contact i.
	Touching [][2][]int
	// TouchingAt gives a point of each touching contact.
	TouchingAt [][3]float64
}
