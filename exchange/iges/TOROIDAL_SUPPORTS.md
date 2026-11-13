# Exact standalone ring-torus supports

`ReadToroidalSupports` and `WriteToroidalSupports` exchange IGES198 form1:
center116, axis123, major radius, minor radius and reference123. `Geometry`
returns `geom.TorusSurface`. U is the major-circle angle measured from the
reference ray around the axis; V is the tube angle. Both are periodic in radians.
The complete analytic support has no authored finite trims or seam edges. No
material orientation, shell closure or solid is inferred from periodicity.

Only regular ring tori, major radius > minor radius > 0, are accepted. Horn and
spindle cases fail. Frames require perpendicular signed coordinate unit vectors,
and source geometry decimals must convert exactly to float64. No normalization,
projection, rounding repair or unit rescaling is performed. All four labels and
subscripts and explicit length units survive. Presentation styling is outside
this descriptor; geometry usage status is emitted canonically.

Form0 implicit meridians, transformations, topology, shared dependencies and
unused entities fail. Limits include 64 supports, 256 entities and ordinary
parser byte/record/value limits. Writer output is staged and re-read before
caller I/O. Existing analytic and topology modes retain their acceptance gates.

The retained source was independently authored using OCCT's type198 API.
Optional `PLAINSOLID_OCCT_IGES_TORUS` tests compare source and native-authored
analytic support points and derivatives for both axis signs and a nondefault
reference ray. Native tests reject horn/spindle radii and exercise both periodic
parameters. Numerical evaluation is qualification evidence, not the exact
source-identity gate or a solid-validity proof.
