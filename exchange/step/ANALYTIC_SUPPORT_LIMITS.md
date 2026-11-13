# Exact analytic frame and unit-context limits

The standalone plane/cylinder/sphere/torus codecs require signed coordinate
frames. This is not merely a conservative search over binary64 rotations:
there is no other exactly unit vector with three dyadic rational components.
Clear a common denominator 2^k from x²+y²+z²=1. For k>0, the integer squared
numerators sum to a multiple of four. Each square is zero or one modulo four;
with only three components, all three numerators must therefore be even.
Repeat until k=0, when the only integer solutions are signed coordinate axes.
Thus non-cardinal exact frames require a rational/algebraic representation,
not another binary64 normalization check. Approximate orthonormality is not
silently promoted to exact source identity by these codecs.

The shared standalone analytic context validator now also rejects input that
the legacy length-only context reader could ignore or round: geometric dimension
must be integer 3; exactly one length, radian and steradian unit must be present;
angular unit components and arities must be valid and angular SI prefixes must
be absent. Conversion-based length dimensions are checked without rounding
near-integer exponents into admissible zero/one values. Existing SI and explicit
foot/inch unit graphs remain supported. Raw coordinate units are not rescaled.

This restriction applies only to the four standalone analytic support readers.
Generic shape, older geometry and swept readers retain their existing contracts;
this milestone makes no claim that all legacy unit readers enforce the same
metadata policy. Prefix-scaled angular contexts and additional context metadata
are unsupported rather than silently discarded. Seven mutation classes across
all four readers reproduce the former acceptance and now fail atomically.
