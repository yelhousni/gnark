/*
Copyright © 2020 ConsenSys

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package frontend

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/internal/backend/compiled"

	bls12377r1cs "github.com/consensys/gnark/internal/backend/bls12-377/cs"
	bls12381r1cs "github.com/consensys/gnark/internal/backend/bls12-381/cs"
	bls24315r1cs "github.com/consensys/gnark/internal/backend/bls24-315/cs"
	bn254r1cs "github.com/consensys/gnark/internal/backend/bn254/cs"
	bw6633r1cs "github.com/consensys/gnark/internal/backend/bw6-633/cs"
	bw6761r1cs "github.com/consensys/gnark/internal/backend/bw6-761/cs"
)

// TODO type = doesn't work as designed here
type idCS = int
type idPCS = int

// sparseR1CS extends the ConstraintSystem
// alongside with some intermediate data structures needed to convert from
// ConstraintSystem representataion to SparseR1CS
type sparseR1CS struct {
	*ConstraintSystem

	ccs compiled.SparseR1CS

	// cs_variable_id -> plonk_cs_variable_id (internal variables only)
	mCStoCCS        []int
	solvedVariables []bool
}

func (cs *ConstraintSystem) toSparseR1CS(curveID ecc.ID) (CompiledConstraintSystem, error) {

	res := sparseR1CS{
		ConstraintSystem: cs,
		ccs: compiled.SparseR1CS{
			NbPublicVariables: len(cs.public.variables) - 1, // the ONE_WIRE is discarded as it is not used in PLONK
			NbSecretVariables: len(cs.secret.variables),
			Constraints:       make([]compiled.SparseR1C, 0, len(cs.constraints)),
			Assertions:        make([]compiled.SparseR1C, 0, len(cs.assertions)),
			Logs:              make([]compiled.LogEntry, len(cs.logs)),
		},
		mCStoCCS:        make([]int, len(cs.internal.variables)),
		solvedVariables: make([]bool, len(cs.internal.variables)),
	}

	// convert the constraints invidually
	for i := 0; i < len(cs.constraints); i++ {
		res.r1cToSparseR1C(cs.constraints[i])
	}
	for i := 0; i < len(cs.assertions); i++ {
		res.splitR1C(cs.assertions[i])
	}

	// offset the ID in a term
	offsetIDTerm := func(t *compiled.Term) error {

		// in a PLONK constraint, not all terms are necessarily set,
		// the terms which are not set are equal to zero. We just
		// need to skip them.
		if *t != 0 {
			_, vID, visibility := t.Unpack()
			switch visibility {
			case compiled.Public:
				t.SetVariableID(vID - 1) // -1 because the ONE_WIRE's is not counted
			case compiled.Secret:
				t.SetVariableID(vID + res.ccs.NbPublicVariables)
			case compiled.Internal:
				t.SetVariableID(vID + res.ccs.NbPublicVariables + res.ccs.NbSecretVariables)
			case compiled.Unset:
				//return fmt.Errorf("%w: %s", ErrInputNotSet, cs.unsetVariables[0].format)
				return fmt.Errorf("%w", ErrInputNotSet)
			}
		}

		return nil
	}

	offsetIDs := func(exp *compiled.SparseR1C) error {

		// ensure that L=M[0] and R=M[1] (up to scalar mul)
		if exp.L.CoeffID() == 0 {
			if exp.M[0] != 0 {
				exp.L = exp.M[0]
				exp.L.SetCoeffID(0)
			}
		} else {
			if exp.M[0].CoeffID() == 0 {
				exp.M[0] = exp.L
				exp.M[0].SetCoeffID(0)
			}
		}

		if exp.R.CoeffID() == 0 {
			if exp.M[1] != 0 {
				exp.R = exp.M[1]
				exp.R.SetCoeffID(0)
			}
		} else {
			if exp.M[1].CoeffID() == 0 {
				exp.M[1] = exp.R
				exp.M[1].SetCoeffID(0)
			}
		}

		// offset each term in the constraint
		err := offsetIDTerm(&exp.L)
		if err != nil {
			return err
		}
		err = offsetIDTerm(&exp.R)
		if err != nil {
			return err
		}
		err = offsetIDTerm(&exp.O)
		if err != nil {
			return err
		}
		err = offsetIDTerm(&exp.M[0])
		if err != nil {
			return err
		}
		err = offsetIDTerm(&exp.M[1])
		if err != nil {
			return err
		}
		return nil
	}

	// offset the IDs of all constraints so that the variables are
	// numbered like this: [publicVariables| secretVariables | internalVariables ]
	for i := 0; i < len(res.ccs.Constraints); i++ {
		if err := offsetIDs(&res.ccs.Constraints[i]); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(res.ccs.Assertions); i++ {
		if err := offsetIDs(&res.ccs.Assertions[i]); err != nil {
			return nil, err
		}
	}

	// offset IDs in the logs
	for i := 0; i < len(cs.logs); i++ {
		entry := compiled.LogEntry{
			Format:    cs.logs[i].format,
			ToResolve: make([]int, len(cs.logs[i].toResolve)),
		}
		for j := 0; j < len(cs.logs[i].toResolve); j++ {
			_, cID, cVisibility := cs.logs[i].toResolve[j].Unpack()
			switch cVisibility {
			case compiled.Public:
				entry.ToResolve[j] += cID - 1 //+ res.NbInternalVariables + res.NbSecretVariables // -1 because the ONE_WIRE's is not counted
			case compiled.Secret:
				entry.ToResolve[j] += cID + res.ccs.NbPublicVariables
			case compiled.Internal:
				entry.ToResolve[j] = res.mCStoCCS[cID] + res.ccs.NbSecretVariables + res.ccs.NbPublicVariables
			case compiled.Unset:
				panic("encountered unset visibility on a variable in logs id offset routine")
			}
		}
		res.ccs.Logs[i] = entry
	}

	switch curveID {
	case ecc.BLS12_377:
		return bls12377r1cs.NewSparseR1CS(res.ccs, cs.coeffs), nil
	case ecc.BLS12_381:
		return bls12381r1cs.NewSparseR1CS(res.ccs, cs.coeffs), nil
	case ecc.BN254:
		return bn254r1cs.NewSparseR1CS(res.ccs, cs.coeffs), nil
	case ecc.BW6_761:
		return bw6761r1cs.NewSparseR1CS(res.ccs, cs.coeffs), nil
	case ecc.BLS24_315:
<<<<<<< HEAD
		return bls24315r1cs.NewSparseR1CS(res, res.Coeffs), nil
	case ecc.BW6_633:
		return bw6633r1cs.NewSparseR1CS(res, res.Coeffs), nil
=======
		return bls24315r1cs.NewSparseR1CS(res.ccs, cs.coeffs), nil
>>>>>>> master
	case ecc.UNKNOWN:
		// TODO cleanup ? why does this path exists?
		return &res.ccs, nil
	default:
		panic("not implemtented")
	}

}

// findUnsolvedVariable returns the variable to solve in the r1c. The variables
// which are not internal are considered solve, otherwise the solvedVariables
// slice hold the record of which variables have been solved.
func findUnsolvedVariable(r1c compiled.R1C, solvedVariables []bool) (pos int, id int) {
	// find the variable to solve among L,R,O. pos=0,1,2 corresponds to left,right,o.
	pos = -1
	id = -1
	for i := 0; i < len(r1c.L); i++ {
		v := r1c.L[i].VariableVisibility()
		if v != compiled.Internal {
			continue
		}
		id = r1c.L[i].VariableID()
		if !solvedVariables[id] {
			pos = 0
			break
		}
	}
	if pos == -1 {
		for i := 0; i < len(r1c.R); i++ {
			v := r1c.R[i].VariableVisibility()
			if v != compiled.Internal {
				continue
			}
			id = r1c.R[i].VariableID()
			if !solvedVariables[id] {
				pos = 1
				break
			}
		}
	}
	if pos == -1 {
		for i := 0; i < len(r1c.O); i++ {
			v := r1c.O[i].VariableVisibility()
			if v != compiled.Internal {
				continue
			}
			id = r1c.O[i].VariableID()
			if !solvedVariables[id] {
				pos = 2
				break
			}
		}
	}

	return pos, id
}

// returns l with the term (id+coef) holding the id-th variable removed
// No side effects on l.
func popInternalVariable(l compiled.LinearExpression, id int) (compiled.LinearExpression, compiled.Term) {
	var t compiled.Term
	_l := make([]compiled.Term, len(l)-1)
	c := 0
	for i := 0; i < len(l); i++ {
		v := l[i]
		if v.VariableVisibility() == compiled.Internal && v.VariableID() == id {
			t = v
			continue
		}
		_l[c] = v
		c++
	}
	return _l, t
}

// pops the constant associated to the one_wire in the cs, which will become
// a constant in a PLONK constraint.
// returns the reduced linear expression and the ID of the coeff corresponding to the constant term (in cs.coeffs).
// If there is no constant term, the id is 0 (the 0-th entry is reserved for this purpose).
func (scs *sparseR1CS) popConstantTerm(l compiled.LinearExpression) (compiled.LinearExpression, big.Int) {

	const idOneWire = 0

	// TODO @thomas can "1 public" appear only once?
	for i := 0; i < len(l); i++ {
		if l[i].VariableID() == idOneWire && l[i].VariableVisibility() == compiled.Public {
			lCopy := make(compiled.LinearExpression, len(l)-1)
			copy(lCopy, l[:i])
			copy(lCopy[i:], l[i+1:])
			return lCopy, scs.coeffs[l[i].CoeffID()]
		}
	}

	return l, big.Int{}
}

// change t's ID to scs.mCStoCCS[t.ID] to get the corresponding variable in the ccs,
func (scs *sparseR1CS) getCorrespondingTerm(t compiled.Term) compiled.Term {

	// if the variable is internal, we need the variable
	// that corresponds in the ccs
	if t.VariableVisibility() == compiled.Internal {
		t.SetVariableID(scs.mCStoCCS[t.VariableID()])
	}

	// Otherwise, the variable's ID and visibility is the same
	return t
}

// newTerm creates a new term =1*new_variable and
// records it in the ccs.
// if idCS is set, updates the mapping of the new variable with the cs one
func (scs *sparseR1CS) newTerm(coeff *big.Int, idCS ...int) compiled.Term {
	cID := scs.coeffID(coeff)
	vID := scs.ccs.NbInternalVariables
	res := compiled.Pack(vID, cID, compiled.Internal)
	scs.ccs.NbInternalVariables++

	if len(idCS) > 0 {
		scs.mCStoCCS[idCS[0]] = vID
	}

	return res
}

// addConstraint records a plonk constraint in the ccs
// The function ensures that all variables ID are set, even
// if the corresponding coefficients are 0.
// A plonk constraint will always look like this:
// L+R+L.R+O+K = 0
func (scs *sparseR1CS) addConstraint(c compiled.SparseR1C) {
	if c.L == 0 {
		c.L.SetVariableID(c.M[0].VariableID())
	}
	if c.R == 0 {
		c.R.SetVariableID(c.M[1].VariableID())
	}
	if c.M[0] == 0 {
		c.M[0].SetVariableID(c.L.VariableID())
	}
	if c.M[1] == 0 {
		c.M[1].SetVariableID(c.R.VariableID())
	}
	scs.ccs.Constraints = append(scs.ccs.Constraints, c)
}

// recordAssertion records a plonk constraint (assertion) in the ccs
func (scs *sparseR1CS) recordAssertion(c compiled.SparseR1C) {
	scs.ccs.Assertions = append(scs.ccs.Assertions, c)
}

// if t=a*variable, it returns -a*variable
func (scs *sparseR1CS) negate(t compiled.Term) compiled.Term {
	// non existing term are zero, if we negate it it's no
	// longer zero and checks to see if a variable exist will
	// fail (ex: in r1cToPlonkConstraint we might call negate
	// on non existing variables, when split is called with
	// le = nil)
	if t == 0 {
		return t
	}
	coeff := bigIntPool.Get().(*big.Int)
	defer bigIntPool.Put(coeff)

	coeff.Neg(&scs.coeffs[t.CoeffID()])
	t.SetCoeffID(scs.coeffID(coeff))
	return t
}

// multiplies t by the provided coefficient
func (scs *sparseR1CS) multiply(t compiled.Term, c *big.Int) compiled.Term {
	// fast path
	if c.IsInt64() {
		v := c.Int64()
		switch v {
		case 0:
			t.SetCoeffID(compiled.CoeffIdZero)
			return t
		case 1:
			return t
		case -1:

			switch t.CoeffID() {
			case compiled.CoeffIdZero:
				return t
			case compiled.CoeffIdOne:
				t.SetCoeffID(compiled.CoeffIdMinusOne)
				return t
			case compiled.CoeffIdMinusOne:
				t.SetCoeffID(compiled.CoeffIdOne)
				return t
			}
		}
	}
	coeff := bigIntPool.Get().(*big.Int)
	coeff.Mul(&scs.coeffs[t.CoeffID()], c)
	t.SetCoeffID(scs.coeffID(coeff))
	bigIntPool.Put(coeff)
	return t
}

// split splits a linear expression to plonk constraints
// ex: le = aiwi is split into PLONK constraints (using sums)
// of 3 terms) like this:
// w0' = a0w0+a1w1
// w1' = w0' + a2w2
// ..
// wn' = wn-1'+an-2wn-2
// split returns a term that is equal to aiwi (it's 1xaiwi)
// no side effects on le
func (scs *sparseR1CS) split(acc compiled.Term, le compiled.LinearExpression) compiled.Term {

	// floor case
	if len(le) == 0 {
		return acc
	}

	// first call
	if acc == 0 {
		t := scs.getCorrespondingTerm(le[0])
		return scs.split(t, le[1:])
	}

	// recursive case
	r := scs.getCorrespondingTerm(le[0])
	o := scs.newTerm(bOne)
	scs.addConstraint(compiled.SparseR1C{L: acc, R: r, O: o})
	o = scs.negate(o)
	return scs.split(o, le[1:])

}

func (scs *sparseR1CS) r1cToSparseR1C(r1c compiled.R1C) {
	if r1c.Solver == compiled.SingleOutput {
		scs.r1cToPlonkConstraintSingleOutput(r1c)
	} else {
		scs.r1cToPlonkConstraintBinary(r1c)
	}
}

// r1cToPlonkConstraintSingleOutput splits a r1c constraint
func (scs *sparseR1CS) r1cToPlonkConstraintSingleOutput(r1c compiled.R1C) {

	// find if the variable to solve is in the left, right, or o linear expression
	lro, idCS := findUnsolvedVariable(r1c, scs.solvedVariables)

	o := r1c.O
	l := r1c.L
	r := r1c.R

	// if the unsolved variable in not in o,
	// ensure that it is in r1c.L
	if lro == 1 {
		l, r = r, l
		lro = 0
	}

	var (
		cK big.Int // constant K
		cS big.Int // constant S (associated with toSolve)
	)
	var toSolve compiled.Term

	l, cL := scs.popConstantTerm(l)
	r, cR := scs.popConstantTerm(r)
	o, cO := scs.popConstantTerm(o)

	// pop the unsolved wire from the linearexpression
	if lro == 0 { // unsolved is in L
		l, toSolve = popInternalVariable(l, idCS)
	} else { // unsolved is in O
		o, toSolve = popInternalVariable(o, idCS)
	}

	// set cS to toSolve coeff
	cS.Set(&scs.coeffs[toSolve.CoeffID()])

	// cL*cR = toSolve + cO
	f1 := func() {
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		})
	}

	// cL*(r + cR) = toSolve + cO
	f2 := func() {
		rt := scs.split(0, r)

		cRT := scs.multiply(rt, &cL)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			R: cRT,
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		},
		)
	}

	// (l + cL)*cR = toSolve + cO
	f3 := func() {
		lt := scs.split(0, l)

		cRLT := scs.multiply(lt, &cR)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: cRLT,
			O: scs.newTerm(cS.Neg(&cS), idCS),
			K: scs.coeffID(&cK),
		})
	}

	// (l + cL)*(r + cR) = toSolve + cO
	f4 := func() {
		lt := scs.split(0, l)
		rt := scs.split(0, r)

		cRLT := scs.multiply(lt, &cR)
		cRT := scs.multiply(rt, &cL)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: cRLT,
			R: cRT,
			M: [2]compiled.Term{lt, rt},
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		})
	}

	// cL*cR = toSolve + o + cO
	f5 := func() {
		ot := scs.split(0, o)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)
		cK.Neg(&cK)

		scs.addConstraint(compiled.SparseR1C{
			L: ot,
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		})
	}

	// cL*(r + cR) = toSolve + o + cO
	f6 := func() {
		rt := scs.split(0, r)
		ot := scs.split(0, o)

		cRT := scs.multiply(rt, &cL)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)
		cK.Neg(&cK)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.negate(ot),
			R: cRT,
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		})
	}

	// (l + cL)*cR = toSolve + o + cO
	f7 := func() {
		lt := scs.split(0, l)
		ot := scs.split(0, o)

		cRLT := scs.multiply(lt, &cR)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)
		cK.Neg(&cK)

		scs.addConstraint(compiled.SparseR1C{
			R: scs.negate(ot),
			L: cRLT,
			K: scs.coeffID(&cK),
			O: scs.newTerm(cS.Neg(&cS), idCS),
		})
	}

	// (l + cL)*(r + cR) = toSolve + o + cO
	f8 := func() {
		lt := scs.split(0, l)
		rt := scs.split(0, r)
		ot := scs.split(0, o)

		cRLT := scs.multiply(lt, &cR)
		cRT := scs.multiply(rt, &cL)
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)
		cK.Neg(&cK)

		u := scs.newTerm(bOne)
		scs.addConstraint(compiled.SparseR1C{
			L: cRLT,
			R: cRT,
			M: [2]compiled.Term{lt, rt},
			K: scs.coeffID(&cK),
			O: u,
		})

		scs.addConstraint(compiled.SparseR1C{
			L: u,
			R: ot,
			O: scs.newTerm(&cS, idCS),
		})
	}

	// (toSolve + cL)*cR = cO
	f9 := func() {
		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		cS.Mul(&cS, &cR)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.newTerm(&cS, idCS),
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + cL)*(r + cR) = cO
	f10 := func() {
		res := scs.newTerm(&cS, idCS)

		rt := scs.split(0, r)
		cRT := scs.multiply(rt, &cL)
		cRes := scs.multiply(res, &cR)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: cRes,
			R: cRT,
			M: [2]compiled.Term{res, rt},
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + l + cL)*cR = cO
	f11 := func() {
		lt := scs.split(0, l)
		lt = scs.multiply(lt, &cR)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		cS.Mul(&cS, &cR)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.newTerm(&cS, idCS),
			R: lt,
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + l + cL)*(r + cR) = cO
	// => toSolve*r + toSolve*cR + [ l*r + l*cR +cL*r+cL*cR-cO ]=0
	f12 := func() {
		u := scs.newTerm(bOne)
		lt := scs.split(0, l)
		rt := scs.split(0, r)
		cRLT := scs.multiply(lt, &cR)
		cRT := scs.multiply(rt, &cL)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: cRLT,
			R: cRT,
			M: [2]compiled.Term{lt, rt},
			O: u,
			K: scs.coeffID(&cK),
		})

		res := scs.newTerm(&cS, idCS)
		cRes := scs.multiply(res, &cR)

		scs.addConstraint(compiled.SparseR1C{
			R: cRes,
			M: [2]compiled.Term{res, rt},
			O: scs.negate(u),
		})
	}

	// (toSolve + cL)*cR = o + cO
	f13 := func() {
		ot := scs.split(0, o)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		cS.Mul(&cS, &cR)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.newTerm(&cS, idCS),
			O: scs.negate(ot),
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + cL)*(r + cR) = o + cO
	// toSolve*r + toSolve*cR+cL*r+cL*cR-cO-o=0
	f14 := func() {
		ot := scs.split(0, o)
		res := scs.newTerm(&cS, idCS)

		rt := scs.split(0, r)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.multiply(res, &cR),
			R: scs.multiply(rt, &cL),
			M: [2]compiled.Term{res, rt},
			O: scs.negate(ot),
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + l + cL)*cR = o + cO
	// toSolve*cR + l*cR + cL*cR-cO-o=0
	f15 := func() {
		ot := scs.split(0, o)

		lt := scs.split(0, l)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		cS.Mul(&cS, &cR)

		scs.addConstraint(compiled.SparseR1C{
			L: scs.newTerm(&cS, idCS),
			R: scs.multiply(lt, &cR),
			O: scs.negate(ot),
			K: scs.coeffID(&cK),
		})
	}

	// (toSolve + l + cL)*(r + cR) = o + cO
	// => toSolve*r + toSolve*cR + [ [l*r + l*cR +cL*r+cL*cR-cO]- o ]=0
	f16 := func() {
		// [l*r + l*cR +cL*r+cL*cR-cO] + u = 0
		u := scs.newTerm(bOne)
		lt := scs.split(0, l)
		rt := scs.split(0, r)
		cRLT := scs.multiply(lt, &cR)
		cRT := scs.multiply(rt, &cL)

		cK.Mul(&cL, &cR)
		cK.Sub(&cK, &cO)

		scs.addConstraint(compiled.SparseR1C{
			L: cRLT,
			R: cRT,
			M: [2]compiled.Term{lt, rt},
			O: u,
			K: scs.coeffID(&cK),
		})

		// u+o+v = 0 (v = -u - o = [l*r + l*cR +cL*r+cL*cR-cO] -  o)
		v := scs.newTerm(bOne)
		ot := scs.split(0, o)
		scs.addConstraint(compiled.SparseR1C{
			L: u,
			R: ot,
			O: v,
		})

		// toSolve*r + toSolve*cR + v = 0
		res := scs.newTerm(&cS, idCS)
		cRes := scs.multiply(res, &cR)

		scs.addConstraint(compiled.SparseR1C{
			R: cRes,
			M: [2]compiled.Term{res, rt},
			O: v,
		})
	}

	// we have 16 different cases
	var s uint8
	if lro != 0 {
		s |= 0b1000
	}
	if len(o) != 0 {
		s |= 0b0100
	}
	if len(l) != 0 {
		s |= 0b0010
	}
	if len(r) != 0 {
		s |= 0b0001
	}
	switch s {
	case 0b0000:
		// (toSolve + cL)*cR = cO
		f9()
	case 0b0001:
		// (toSolve + cL)*(r + cR) = cO
		f10()
	case 0b0010:
		// (toSolve + l + cL)*cR = cO
		f11()
	case 0b0011:
		// (toSolve + l + cL)*(r + cR) = cO
		// => toSolve*r + toSolve*cR + [ l*r + l*cR +cL*r+cL*cR-cO ]=0
		f12()
	case 0b0100:
		// (toSolve + cL)*cR = o + cO
		f13()
	case 0b0101:
		// (toSolve + cL)*(r + cR) = o + cO
		// toSolve*r + toSolve*cR+cL*r+cL*cR-cO-o=0
		f14()
	case 0b0110:
		// (toSolve + l + cL)*cR = o + cO
		// toSolve*cR + l*cR + cL*cR-cO-o=0
		f15()
	case 0b0111:
		// (toSolve + l + cL)*(r + cR) = o + cO
		// => toSolve*r + toSolve*cR + [ [l*r + l*cR +cL*r+cL*cR-cO]- o ]=0
		f16()
	case 0b1000:
		// cL*cR = toSolve + cO
		f1()
	case 0b1001:
		// cL*(r + cR) = toSolve + cO
		f2()
	case 0b1010:
		// (l + cL)*cR = toSolve + cO
		f3()
	case 0b1011:
		// (l + cL)*(r + cR) = toSolve + cO
		f4()
	case 0b1100:
		// cL*cR = toSolve + o + cO
		f5()
	case 0b1101:
		// cL*(r + cR) = toSolve + o + cO
		f6()
	case 0b1110:
		// (l + cL)*cR = toSolve + o + cO
		f7()
	case 0b1111:
		// (l + cL)*(r + cR) = toSolve + o + cO
		f8()
	}

	scs.solvedVariables[idCS] = true
}

// r1cToPlonkConstraintBinary splits a r1c constraint corresponding
// to a binary decomposition.
func (scs *sparseR1CS) r1cToPlonkConstraintBinary(r1c compiled.R1C) {

	// from cs_api, le binary decomposition is r1c.L
	binDec := make(compiled.LinearExpression, len(r1c.L))
	copy(binDec, r1c.L)

	// reduce r1c.O (in case it's a linear combination)
	var ot compiled.Term
	o, cO := scs.popConstantTerm(r1c.O)
	cOID := scs.coeffID(&cO)
	if len(o) == 0 { // o is a constant term
		ot = scs.newTerm(bOne)
		scs.addConstraint(compiled.SparseR1C{L: scs.negate(ot), K: cOID})
	} else {
		ot = scs.split(0, o)
		if cOID != 0 {
			_ot := scs.newTerm(bOne)
			scs.addConstraint(compiled.SparseR1C{L: ot, O: scs.negate(_ot), K: cOID}) // _ot+ot+K = 0
			ot = _ot
		}
	}

	// split the linear expression
	nbBits := len(binDec)
	two := big.NewInt(2)
	acc := big.NewInt(1)

	// accumulators for the quotients and remainders when dividing by 2
	accRi := make([]compiled.Term, nbBits) // accRi[0] -> LSB
	accQi := make([]compiled.Term, nbBits+1)
	accQi[0] = ot

	for i := 0; i < nbBits; i++ {

		accRi[i] = scs.newTerm(bOne)
		accQi[i+1] = scs.newTerm(bOne)

		// find the variable corresponding to the i-th bit (it's not ordered since getLinExpCopy is not deterministic)
		// so we can update scs.varPcsToVarCs
		for k := 0; k < len(binDec); k++ {
			t := binDec[k]
			coef := scs.coeffs[t.CoeffID()]
			if coef.Cmp(acc) == 0 {
				scs.mCStoCCS[t.VariableID()] = accRi[i].VariableID()
				scs.solvedVariables[t.VariableID()] = true
				binDec = append(binDec[:k], binDec[k+1:]...)
				break
			}
		}
		acc.Mul(acc, two)

		// 2*q[i+1] + ri - q[i] = 0
		scs.addConstraint(compiled.SparseR1C{
			L:      scs.multiply(accQi[i+1], two),
			R:      accRi[i],
			O:      scs.negate(accQi[i]),
			Solver: compiled.BinaryDec,
		})
	}
}

// splitR1C splits a r1c assertion (meaning that
// it's a r1c constraint that is not used to solve a variable,
// like a boolean constraint).
// (l + cL)*(r + cR) = o + cO
func (scs *sparseR1CS) splitR1C(r1c compiled.R1C) {

	l := r1c.L
	r := r1c.R
	o := r1c.O

	l, cL := scs.popConstantTerm(l)
	r, cR := scs.popConstantTerm(r)
	o, cO := scs.popConstantTerm(o)

	var cK big.Int

	if len(o) == 0 {

		if len(l) == 0 {

			if len(r) == 0 { // cL*cR = cO (should never happen...)

				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{K: scs.coeffID(&cK)})

			} else { // cL*(r + cR) = cO

				rt := scs.split(0, r)

				cosntlrt := scs.multiply(rt, &cL)
				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{R: cosntlrt, K: scs.coeffID(&cK)})
			}

		} else {

			if len(r) == 0 { // (l + cL)*cR = cO
				lt := scs.split(0, l)

				cRLT := scs.multiply(lt, &cR)
				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{L: cRLT, K: scs.coeffID(&cK)})

			} else { // (l + cL)*(r + cR) = cO

				lt := scs.split(0, l)
				rt := scs.split(0, r)

				cRLT := scs.multiply(lt, &cR)
				cRT := scs.multiply(rt, &cL)
				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{
					L: cRLT,
					R: cRT,
					M: [2]compiled.Term{lt, rt},
					K: scs.coeffID(&cK),
				})
			}
		}

	} else {
		if len(l) == 0 {

			if len(r) == 0 { // cL*cR = o + cO

				ot := scs.split(0, o)

				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{K: scs.coeffID(&cK), O: scs.negate(ot)})

			} else { // cL * (r + cR) = o + cO

				rt := scs.split(0, r)
				ot := scs.split(0, o)

				cRT := scs.multiply(rt, &cL)
				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{
					R: cRT,
					K: scs.coeffID(&cK),
					O: scs.negate(ot),
				})
			}

		} else {
			if len(r) == 0 { // (l + cL) * cR = o + cO

				lt := scs.split(0, l)
				ot := scs.split(0, o)

				cRLT := scs.multiply(lt, &cR)
				cK.Mul(&cL, &cR)
				cK.Sub(&cK, &cO)

				scs.recordAssertion(compiled.SparseR1C{
					L: cRLT,
					K: scs.coeffID(&cK),
					O: scs.negate(ot),
				})

			} else { // (l + cL)*(r + cR) = o + cO
				lt := scs.split(0, l)
				rt := scs.split(0, r)
				ot := scs.split(0, o)

				cRT := scs.multiply(rt, &cL)
				cRLT := scs.multiply(lt, &cR)
				cK.Mul(&cR, &cL)
				cK.Sub(&cK, &cO)

				scs.addConstraint(compiled.SparseR1C{
					L: cRLT,
					R: cRT,
					M: [2]compiled.Term{lt, rt},
					K: scs.coeffID(&cK),
					O: scs.negate(ot),
				})
			}
		}
	}
}

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}
