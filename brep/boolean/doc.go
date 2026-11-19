// Package boolean combines two solids bounded by planes with one pipeline:
// intake, an arrangement of the two boundaries, face splitting,
// classification, selection, a manifold test, merging, assembly and
// validation. A result is accepted when it validates within the tolerance
// the caller passed, never on a proof, and an input the pipeline cannot
// resolve returns an explicit error.
//
// The package is the subject of the trial described in
// specs/general-boolean/SPEC.md, whose requirement identifiers (S1, I5, X6
// and so on) the comments here refer to.
package boolean
