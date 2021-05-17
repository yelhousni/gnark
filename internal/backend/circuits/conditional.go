package circuits

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
)

type conditionalCircuit struct {
	X, X2, Y, P1, P2 frontend.Variable
	Y2               frontend.Variable `gnark:",public"`
}

func (circuit *conditionalCircuit) Define(curveID ecc.ID, cs *frontend.ConstraintSystem) error {

	cs.If(circuit.P1, func() {
		cs.AssertIsEqual(circuit.X, circuit.Y)
		cs.If(circuit.P2, func() {
			cs.AssertIsEqual(circuit.X2, circuit.Y2)
		})
	})

	return nil
}

func init() {

	{
		var circuit, good, bad, public conditionalCircuit

		// if p1 and if p2 -> x == y, x2 == y2
		good.P1.Assign(1)
		good.P2.Assign(1)
		good.X.Assign(41)
		good.Y.Assign(41)
		good.X2.Assign(42)
		good.Y2.Assign(42)

		bad.P1.Assign(1)
		bad.P2.Assign(1)
		bad.X.Assign(41)
		bad.Y.Assign(41)
		bad.X2.Assign(43)
		bad.Y2.Assign(42)

		public.Y2.Assign(42)

		addEntry("conditional_0", &circuit, &good, &bad, &public)
	}

	{
		var circuit, good, bad, public conditionalCircuit

		// if p1 and if not p2 -> x == y, x2 can be != y2
		good.P1.Assign(1)
		good.P2.Assign(0)
		good.X.Assign(41)
		good.Y.Assign(41)
		good.X2.Assign(43)
		good.Y2.Assign(42)

		bad.P1.Assign(1)
		bad.P2.Assign(1)
		bad.X.Assign(41)
		bad.Y.Assign(43)
		bad.X2.Assign(43)
		bad.Y2.Assign(42)

		public.Y2.Assign(42)

		addEntry("conditional_1", &circuit, &good, &bad, &public)
	}

}
