// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by gnark DO NOT EDIT

package groth16_test

import (
	"github.com/consensys/gnark-crypto/ecc/bw6-672/fr"

	curve "github.com/consensys/gnark-crypto/ecc/bw6-672"

	"github.com/consensys/gnark/internal/backend/bw6-672/cs"

	bw6_672witness "github.com/consensys/gnark/internal/backend/bw6-672/witness"

	"bytes"
	bw6_672groth16 "github.com/consensys/gnark/internal/backend/bw6-672/groth16"
	"github.com/fxamacker/cbor/v2"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/circuits"
)

func TestCircuits(t *testing.T) {
	for name, circuit := range circuits.Circuits {
		t.Run(name, func(t *testing.T) {
			assert := groth16.NewAssert(t)
			r1cs, err := frontend.Compile(curve.ID, backend.GROTH16, circuit.Circuit)
			assert.NoError(err)
			assert.ProverFailed(r1cs, circuit.Bad)
			assert.ProverSucceeded(r1cs, circuit.Good)
		})
	}
}

//--------------------//
//     benches		  //
//--------------------//

type refCircuit struct {
	nbConstraints int
	X             frontend.Variable
	Y             frontend.Variable `gnark:",public"`
}

func (circuit *refCircuit) Define(curveID ecc.ID, cs *frontend.ConstraintSystem) error {
	for i := 0; i < circuit.nbConstraints; i++ {
		circuit.X = cs.Mul(circuit.X, circuit.X)
	}
	cs.AssertIsEqual(circuit.X, circuit.Y)
	return nil
}

func referenceCircuit() (frontend.CompiledConstraintSystem, frontend.Circuit) {
	const nbConstraints = 40000
	circuit := refCircuit{
		nbConstraints: nbConstraints,
	}
	r1cs, err := frontend.Compile(curve.ID, backend.GROTH16, &circuit)
	if err != nil {
		panic(err)
	}

	var good refCircuit
	good.X.Assign(2)

	// compute expected Y
	var expectedY fr.Element
	expectedY.SetUint64(2)

	for i := 0; i < nbConstraints; i++ {
		expectedY.Mul(&expectedY, &expectedY)
	}

	good.Y.Assign(expectedY)

	return r1cs, &good
}

func TestReferenceCircuit(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	assert := groth16.NewAssert(t)
	r1cs, witness := referenceCircuit()
	assert.ProverSucceeded(r1cs, witness)
}

func BenchmarkSetup(b *testing.B) {
	r1cs, _ := referenceCircuit()

	var pk bw6_672groth16.ProvingKey
	var vk bw6_672groth16.VerifyingKey
	b.ResetTimer()

	b.Run("setup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bw6_672groth16.Setup(r1cs.(*cs.R1CS), &pk, &vk)
		}
	})
}

func BenchmarkProver(b *testing.B) {
	r1cs, _solution := referenceCircuit()
	fullWitness := bw6_672witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	var pk bw6_672groth16.ProvingKey
	bw6_672groth16.DummySetup(r1cs.(*cs.R1CS), &pk)

	b.ResetTimer()
	b.Run("prover", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = bw6_672groth16.Prove(r1cs.(*cs.R1CS), &pk, fullWitness, false)
		}
	})
}

func BenchmarkVerifier(b *testing.B) {
	r1cs, _solution := referenceCircuit()
	fullWitness := bw6_672witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}
	publicWitness := bw6_672witness.Witness{}
	err = publicWitness.FromPublicAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	var pk bw6_672groth16.ProvingKey
	var vk bw6_672groth16.VerifyingKey
	bw6_672groth16.Setup(r1cs.(*cs.R1CS), &pk, &vk)
	proof, err := bw6_672groth16.Prove(r1cs.(*cs.R1CS), &pk, fullWitness, false)
	if err != nil {
		panic(err)
	}

	b.ResetTimer()
	b.Run("verifier", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bw6_672groth16.Verify(proof, &vk, publicWitness)
		}
	})
}

func BenchmarkSerialization(b *testing.B) {
	r1cs, _solution := referenceCircuit()
	fullWitness := bw6_672witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	var pk bw6_672groth16.ProvingKey
	var vk bw6_672groth16.VerifyingKey
	bw6_672groth16.Setup(r1cs.(*cs.R1CS), &pk, &vk)
	proof, err := bw6_672groth16.Prove(r1cs.(*cs.R1CS), &pk, fullWitness, false)
	if err != nil {
		panic(err)
	}

	b.ReportAllocs()

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.ProvingKey binary serialization
	b.Run("pk: binary serialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = pk.WriteTo(&buf)
		}
	})
	b.Run("pk: binary deserialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = pk.WriteTo(&buf)
		var pkReconstructed bw6_672groth16.ProvingKey
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = pkReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = pk.WriteTo(&buf)
	}

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.ProvingKey binary serialization (uncompressed)
	b.Run("pk: binary raw serialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = pk.WriteRawTo(&buf)
		}
	})
	b.Run("pk: binary raw deserialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = pk.WriteRawTo(&buf)
		var pkReconstructed bw6_672groth16.ProvingKey
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = pkReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = pk.WriteRawTo(&buf)
	}

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.ProvingKey binary serialization (cbor)
	b.Run("pk: binary cbor serialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := cbor.NewEncoder(&buf)
			enc.Encode(&pk)
		}
	})
	b.Run("pk: binary cbor deserialization (bw6_672groth16.ProvingKey)", func(b *testing.B) {
		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		enc.Encode(&pk)
		var pkReconstructed bw6_672groth16.ProvingKey
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			dec := cbor.NewDecoder(buf)
			dec.Decode(&pkReconstructed)
		}
	})
	{
		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		enc.Encode(&pk)
	}

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.Proof binary serialization
	b.Run("proof: binary serialization (bw6_672groth16.Proof)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = proof.WriteTo(&buf)
		}
	})
	b.Run("proof: binary deserialization (bw6_672groth16.Proof)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = proof.WriteTo(&buf)
		var proofReconstructed bw6_672groth16.Proof
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = proofReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = proof.WriteTo(&buf)
	}

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.Proof binary serialization (uncompressed)
	b.Run("proof: binary raw serialization (bw6_672groth16.Proof)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = proof.WriteRawTo(&buf)
		}
	})
	b.Run("proof: binary raw deserialization (bw6_672groth16.Proof)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = proof.WriteRawTo(&buf)
		var proofReconstructed bw6_672groth16.Proof
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = proofReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = proof.WriteRawTo(&buf)
	}

	// ---------------------------------------------------------------------------------------------
	// bw6_672groth16.Proof binary serialization (cbor)
	b.Run("proof: binary cbor serialization (bw6_672groth16.Proof)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := cbor.NewEncoder(&buf)
			enc.Encode(&proof)
		}
	})
	b.Run("proof: binary cbor deserialization (bw6_672groth16.Proof)", func(b *testing.B) {
		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		enc.Encode(&proof)
		var proofReconstructed bw6_672groth16.Proof
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			dec := cbor.NewDecoder(buf)
			dec.Decode(&proofReconstructed)
		}
	})
	{
		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		enc.Encode(&proof)
	}

}
