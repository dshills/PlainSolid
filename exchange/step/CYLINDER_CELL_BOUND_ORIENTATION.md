# Exact axial and bound orientation encodings

The bounded two-/four-cell readers additionally accept these equivalent source
declarations without changing the default writers or generic analytic helpers:

- An axial LINE with direction negative local Z and EDGE_CURVE.same_sense false.
  Its base point remains the stored bottom vertex. Decreasing line parameter
  therefore traverses exactly from bottom to top. Direction positive local Z
  still requires true same_sense; either unpaired reversal fails.
- FACE_BOUND or FACE_OUTER_BOUND orientation false, when the underlying loop
  lists the complete boundary in reverse order with each coedge sense reversed.
  The private proof view reverses both order and senses before applying the
  unchanged angular-role and cap-orientation tests. It does not change the
  ADVANCED_FACE support sense or mutate the source graph.

The existing directed physical edge and coverage contracts remain unchanged.
Axial vector magnitude must be positive and exact, line base points must match
exact source vertices, and all circle/surface/frame checks still apply.
Original source IDs survive; canonical re-export need not reproduce the original
parameter direction or bound flag spelling. This is declared orientation
interpretation, not geometric snapping or tolerance-based repair.

Independent hand-authored combined fixtures have reversed axial model directions,
false axial curve senses, and false bound orientations with reversed loops.
OCCT validates both four- and six-face shapes with volume12π, area20π and
centroid(0,0,1.5). Tests reject a lone bound flip, lone line sense/direction flip,
and one incorrect coedge; all prior model/product/native and rounded-source
rejection tests remain active. The optional OCCT helper is unchanged.
