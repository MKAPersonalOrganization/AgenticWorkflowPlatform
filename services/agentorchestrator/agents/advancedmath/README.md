# Advanced Mathematics Agents

This directory contains specialized professor agents for advanced mathematical topics, powered by the [gosl](https://github.com/cpmech/gosl) scientific computing library.

## Agent Structure

Each mathematical branch has its own dedicated agent under `/advancedmath/<field>/`:

### 1. Linear Algebra (`linearalgebra/`)
**Agent:** `linearalgebra-professor`

**Capabilities:**
- Vector operations (add, subtract, dot/cross product, norms)
- Matrix operations (multiply, transpose, inverse, determinant)
- Matrix decompositions (LU, QR, Cholesky, SVD)
- Linear system solvers (dense and sparse: Umfpack, MUMPS)
- Eigenvalue and eigenvector computation

**Tools:**
- `vector_operations`
- `matrix_operations`
- `linear_solver`
- `eigenvalue_solver`

### 2. Numerical Methods (`numericalmethods/`)
**Agent:** `numericalmethods-professor`

**Capabilities:**
- Root finding (Brent, Newton-Raphson, secant, bisection)
- Nonlinear system solvers
- Numerical derivatives (finite differences, Richardson extrapolation)
- Numerical integration (Gauss-Legendre, Simpson, Romberg, QUADPACK)
- Jacobian and Hessian computation

**Tools:**
- `root_finding`
- `nonlinear_solver`
- `numerical_derivative`
- `numerical_integration`

### 3. Differential Equations (`diffeq/`)
**Agent:** `diffeq-professor`

**Capabilities:**
- ODE solvers (explicit Runge-Kutta: Dormand-Prince 5, 8)
- Stiff ODE solvers (Radau5, BDF methods)
- Adaptive step-size control
- Event detection in ODEs
- Initial value problems

**Tools:**
- `ode_solver`
- `stiff_ode_solver`

### 4. Optimization (`optimization/`)
**Agent:** `optimization-professor`

**Capabilities:**
- Unconstrained optimization (gradient descent, conjugate gradient, Powell)
- Constrained optimization (interior point, barrier methods)
- Line search strategies
- Convergence analysis
- KKT condition checking

**Tools:**
- `unconstrained_optimization`
- `constrained_optimization`

### 5. Special Functions (`specialfunctions/`)
**Agent:** `specialfunctions-professor`

**Capabilities:**
- Bessel functions (J, Y, I, K) and their zeros
- Orthogonal polynomials (Legendre, Chebyshev, Hermite, Laguerre, Jacobi)
- Elliptic integrals (complete and incomplete)
- Interpolation (Lagrange, Chebyshev, cubic spline, B-spline, Fourier)
- FFT and DFT operations

**Tools:**
- `bessel_functions`
- `orthogonal_polynomials`
- `elliptic_integrals`
- `interpolation`
- `fft`

### 6. Computational Geometry (`geometry/`)
**Agent:** `geometry-professor`

**Capabilities:**
- Geometric primitives (points, lines, polygons)
- Distance and intersection computations
- Convex hull and triangulation
- Bézier curves and B-splines
- NURBS (Non-Uniform Rational B-Splines)
- Grid generation and octree structures

**Tools:**
- `geometric_primitives`
- `curves`
- `meshing`

## Implementation Status

⚠️ **Current Status:** Agent structure and tool stubs created. Full implementations require:

1. Integration with gosl library (CGO dependencies)
2. Complete tool implementations for each mathematical operation
3. Proper error handling and numerical precision management
4. Test coverage for mathematical computations

## Usage Pattern

All agents follow the same event-driven pattern:

1. **Receive:** Question event (e.g., `question.linearalgebra`)
2. **Read:** Whiteboard content to understand the problem
3. **Compute:** Execute appropriate mathematical operations
4. **Write:** Results to whiteboard with proper LaTeX formatting
5. **Emit:** `question.answered` event to signal completion

## Dependencies

The gosl library requires:
- gcc
- gfortran
- OpenBLAS
- LAPACK
- FFTW3
- MUMPS
- UMFPACK (SuiteSparse)

For Docker-based development, these are pre-configured in the gosl container.

## Next Steps

1. Set up gosl library integration (via CGO or Docker)
2. Implement concrete tool methods for each mathematical operation
3. Add comprehensive testing with known mathematical results
4. Document numerical precision and performance characteristics
5. Create example problems demonstrating each agent's capabilities
