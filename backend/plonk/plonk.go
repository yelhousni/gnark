// Copyright 2020 ConsenSys AG
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

// Package plonk implements PLONK Zero Knowledge Proof system.
//
// See also
//
// https://eprint.iacr.org/2019/953
package plonk

import (
	"crypto/rand"
	"io"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/frontend"

	cs_bls12377 "github.com/consensys/gnark/internal/backend/bls12-377/cs"
	cs_bls12381 "github.com/consensys/gnark/internal/backend/bls12-381/cs"
	cs_bls24315 "github.com/consensys/gnark/internal/backend/bls24-315/cs"
	cs_bn254 "github.com/consensys/gnark/internal/backend/bn254/cs"
	cs_bw6633 "github.com/consensys/gnark/internal/backend/bw6-633/cs"
	cs_bw6761 "github.com/consensys/gnark/internal/backend/bw6-761/cs"

	plonk_bls12377 "github.com/consensys/gnark/internal/backend/bls12-377/plonk"
	plonk_bls12381 "github.com/consensys/gnark/internal/backend/bls12-381/plonk"
	plonk_bls24315 "github.com/consensys/gnark/internal/backend/bls24-315/plonk"
	plonk_bn254 "github.com/consensys/gnark/internal/backend/bn254/plonk"
	plonk_bw6633 "github.com/consensys/gnark/internal/backend/bw6-633/plonk"
	plonk_bw6761 "github.com/consensys/gnark/internal/backend/bw6-761/plonk"

	witness_bls12377 "github.com/consensys/gnark/internal/backend/bls12-377/witness"
	witness_bls12381 "github.com/consensys/gnark/internal/backend/bls12-381/witness"
	witness_bls24315 "github.com/consensys/gnark/internal/backend/bls24-315/witness"
	witness_bn254 "github.com/consensys/gnark/internal/backend/bn254/witness"
	witness_bw6633 "github.com/consensys/gnark/internal/backend/bw6-633/witness"
	witness_bw6761 "github.com/consensys/gnark/internal/backend/bw6-761/witness"

	kzg_bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr/kzg"
	kzg_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr/kzg"
	kzg_bls24315 "github.com/consensys/gnark-crypto/ecc/bls24-315/fr/kzg"
	kzg_bn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr/kzg"
	kzg_bw6633 "github.com/consensys/gnark-crypto/ecc/bw6-633/fr/kzg"
	kzg_bw6761 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr/kzg"

	fr_bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	fr_bls24315 "github.com/consensys/gnark-crypto/ecc/bls24-315/fr"
	fr_bn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	fr_bw6633 "github.com/consensys/gnark-crypto/ecc/bw6-633/fr"
	fr_bw6761 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
)

// Proof represents a Plonk proof generated by plonk.Prove
//
// it's underlying implementation is curve specific (see gnark/internal/backend)
type Proof interface {
	io.WriterTo
	io.ReaderFrom
}

// ProvingKey represents a plonk ProvingKey
//
// it's underlying implementation is strongly typed with the curve (see gnark/internal/backend)
type ProvingKey interface {
	io.WriterTo
	io.ReaderFrom
	InitKZG(srs kzg.SRS) error
	VerifyingKey() interface{}
}

// VerifyingKey represents a plonk VerifyingKey
//
// it's underlying implementation is strongly typed with the curve (see gnark/internal/backend)
type VerifyingKey interface {
	io.WriterTo
	io.ReaderFrom
	InitKZG(srs kzg.SRS) error
	NbPublicWitness() int // number of elements expected in the public witness
}

// NewSRS uses ccs nb variables and nb constraints to initialize a kzg srs
// note that this method is here for convenience only: in production, a SRS generated through MPC should be used.
func NewSRS(ccs frontend.CompiledConstraintSystem) (kzg.SRS, error) {

	nbConstraints := ccs.GetNbConstraints()
	internal, secret, public := ccs.GetNbVariables()
	nbVariables := internal + secret + public
	kzgSize := uint64(nbVariables)
	if nbConstraints > nbVariables {
		kzgSize = uint64(nbConstraints)
	}
	kzgSize = ecc.NextPowerOfTwo(kzgSize) + 3

	switch ccs.(type) {
	case *cs_bn254.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bn254.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bn254.NewSRS(kzgSize, alpha)
	case *cs_bls12381.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bls12381.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bls12381.NewSRS(kzgSize, alpha)
	case *cs_bls12377.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bls12377.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bls12377.NewSRS(kzgSize, alpha)
	case *cs_bw6761.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bw6761.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bw6761.NewSRS(kzgSize, alpha)
	case *cs_bw6633.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bw6633.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bw6633.NewSRS(kzgSize, alpha)
	case *cs_bls24315.SparseR1CS:
		alpha, err := rand.Int(rand.Reader, fr_bls24315.Modulus())
		if err != nil {
			return nil, err
		}
		return kzg_bls24315.NewSRS(kzgSize, alpha)
	default:
		panic("unrecognized R1CS curve type")
	}

}

// Setup prepares the public data associated to a circuit + public inputs.
func Setup(ccs frontend.CompiledConstraintSystem, kzgSRS kzg.SRS) (ProvingKey, VerifyingKey, error) {

	switch tccs := ccs.(type) {
	case *cs_bn254.SparseR1CS:
		return plonk_bn254.Setup(tccs, kzgSRS.(*kzg_bn254.SRS))
	case *cs_bls12381.SparseR1CS:
		return plonk_bls12381.Setup(tccs, kzgSRS.(*kzg_bls12381.SRS))
	case *cs_bls12377.SparseR1CS:
		return plonk_bls12377.Setup(tccs, kzgSRS.(*kzg_bls12377.SRS))
	case *cs_bw6761.SparseR1CS:
		return plonk_bw6761.Setup(tccs, kzgSRS.(*kzg_bw6761.SRS))
	case *cs_bls24315.SparseR1CS:
		return plonk_bls24315.Setup(tccs, kzgSRS.(*kzg_bls24315.SRS))
	case *cs_bw6633.SparseR1CS:
		return plonk_bw6633.Setup(tccs, kzgSRS.(*kzg_bw6633.SRS))
	default:
		panic("unrecognized R1CS curve type")
	}
}

// Prove generates PLONK proof from a circuit, associated preprocessed public data, and the witness
func Prove(ccs frontend.CompiledConstraintSystem, pk ProvingKey, fullWitness frontend.Circuit) (Proof, error) {

	switch tccs := ccs.(type) {
	case *cs_bn254.SparseR1CS:
		w := witness_bn254.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bn254.Prove(tccs, pk.(*plonk_bn254.ProvingKey), w)

	case *cs_bls12381.SparseR1CS:
		w := witness_bls12381.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bls12381.Prove(tccs, pk.(*plonk_bls12381.ProvingKey), w)

	case *cs_bls12377.SparseR1CS:
		w := witness_bls12377.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bls12377.Prove(tccs, pk.(*plonk_bls12377.ProvingKey), w)

	case *cs_bw6761.SparseR1CS:
		w := witness_bw6761.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bw6761.Prove(tccs, pk.(*plonk_bw6761.ProvingKey), w)

	case *cs_bls24315.SparseR1CS:
		w := witness_bls24315.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bls24315.Prove(tccs, pk.(*plonk_bls24315.ProvingKey), w)

	case *cs_bw6633.SparseR1CS:
		w := witness_bw6633.Witness{}
		if err := w.FromFullAssignment(fullWitness); err != nil {
			return nil, err
		}
		return plonk_bw6633.Prove(tccs, pk.(*plonk_bw6633.ProvingKey), w)

	default:
		panic("unrecognized R1CS curve type")
	}
}

// Verify verifies a PLONK proof, from the proof, preprocessed public data, and public witness.
func Verify(proof Proof, vk VerifyingKey, publicWitness frontend.Circuit) error {

	switch _proof := proof.(type) {

	case *plonk_bn254.Proof:
		w := witness_bn254.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bn254.Verify(_proof, vk.(*plonk_bn254.VerifyingKey), w)

	case *plonk_bls12381.Proof:
		w := witness_bls12381.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bls12381.Verify(_proof, vk.(*plonk_bls12381.VerifyingKey), w)

	case *plonk_bls12377.Proof:
		w := witness_bls12377.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bls12377.Verify(_proof, vk.(*plonk_bls12377.VerifyingKey), w)

	case *plonk_bw6761.Proof:
		w := witness_bw6761.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bw6761.Verify(_proof, vk.(*plonk_bw6761.VerifyingKey), w)

	case *plonk_bls24315.Proof:
		w := witness_bls24315.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bls24315.Verify(_proof, vk.(*plonk_bls24315.VerifyingKey), w)

	case *plonk_bw6633.Proof:
		w := witness_bw6633.Witness{}
		if err := w.FromPublicAssignment(publicWitness); err != nil {
			return err
		}
		return plonk_bw6633.Verify(_proof, vk.(*plonk_bw6633.VerifyingKey), w)

	default:
		panic("unrecognized proof type")
	}
}

// NewCS instantiate a concrete curved-typed SparseR1CS and return a CompiledConstraintSystem interface
// This method exists for (de)serialization purposes
func NewCS(curveID ecc.ID) frontend.CompiledConstraintSystem {
	var r1cs frontend.CompiledConstraintSystem
	switch curveID {
	case ecc.BN254:
		r1cs = &cs_bn254.SparseR1CS{}
	case ecc.BLS12_377:
		r1cs = &cs_bls12377.SparseR1CS{}
	case ecc.BLS12_381:
		r1cs = &cs_bls12381.SparseR1CS{}
	case ecc.BW6_761:
		r1cs = &cs_bw6761.SparseR1CS{}
	case ecc.BW6_633:
		r1cs = &cs_bw6633.SparseR1CS{}
	case ecc.BLS24_315:
		r1cs = &cs_bls24315.SparseR1CS{}
	default:
		panic("not implemented")
	}
	return r1cs
}

// NewProvingKey instantiates a curve-typed ProvingKey and returns an interface
// This function exists for serialization purposes
func NewProvingKey(curveID ecc.ID) ProvingKey {
	var pk ProvingKey
	switch curveID {
	case ecc.BN254:
		pk = &plonk_bn254.ProvingKey{}
	case ecc.BLS12_377:
		pk = &plonk_bls12377.ProvingKey{}
	case ecc.BLS12_381:
		pk = &plonk_bls12381.ProvingKey{}
	case ecc.BW6_761:
		pk = &plonk_bw6761.ProvingKey{}
	case ecc.BW6_633:
		pk = &plonk_bw6633.ProvingKey{}
	case ecc.BLS24_315:
		pk = &plonk_bls24315.ProvingKey{}
	default:
		panic("not implemented")
	}

	return pk
}

// NewProof instantiates a curve-typed ProvingKey and returns an interface
// This function exists for serialization purposes
func NewProof(curveID ecc.ID) Proof {
	var proof Proof
	switch curveID {
	case ecc.BN254:
		proof = &plonk_bn254.Proof{}
	case ecc.BLS12_377:
		proof = &plonk_bls12377.Proof{}
	case ecc.BLS12_381:
		proof = &plonk_bls12381.Proof{}
	case ecc.BW6_761:
		proof = &plonk_bw6761.Proof{}
	case ecc.BW6_633:
		proof = &plonk_bw6633.Proof{}
	case ecc.BLS24_315:
		proof = &plonk_bls24315.Proof{}
	default:
		panic("not implemented")
	}

	return proof
}

// NewVerifyingKey instantiates a curve-typed VerifyingKey and returns an interface
// This function exists for serialization purposes
func NewVerifyingKey(curveID ecc.ID) VerifyingKey {
	var vk VerifyingKey
	switch curveID {
	case ecc.BN254:
		vk = &plonk_bn254.VerifyingKey{}
	case ecc.BLS12_377:
		vk = &plonk_bls12377.VerifyingKey{}
	case ecc.BLS12_381:
		vk = &plonk_bls12381.VerifyingKey{}
	case ecc.BW6_761:
		vk = &plonk_bw6761.VerifyingKey{}
	case ecc.BW6_633:
		vk = &plonk_bw6633.VerifyingKey{}
	case ecc.BLS24_315:
		vk = &plonk_bls24315.VerifyingKey{}
	default:
		panic("not implemented")
	}

	return vk
}

// ReadAndProve generates PLONK proof from a circuit, associated proving key, and the full witness
func ReadAndProve(ccs frontend.CompiledConstraintSystem, pk ProvingKey, witness io.Reader) (Proof, error) {

	_, nbSecret, nbPublic := ccs.GetNbVariables()
	expectedSize := (nbSecret + nbPublic)

	switch tccs := ccs.(type) {
	case *cs_bn254.SparseR1CS:
		_pk := pk.(*plonk_bn254.ProvingKey)
		w := witness_bn254.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bn254.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	case *cs_bls12381.SparseR1CS:
		_pk := pk.(*plonk_bls12381.ProvingKey)
		w := witness_bls12381.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bls12381.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	case *cs_bls12377.SparseR1CS:
		_pk := pk.(*plonk_bls12377.ProvingKey)
		w := witness_bls12377.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bls12377.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	case *cs_bw6633.SparseR1CS:
		_pk := pk.(*plonk_bw6633.ProvingKey)
		w := witness_bw6633.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bw6633.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	case *cs_bw6761.SparseR1CS:
		_pk := pk.(*plonk_bw6761.ProvingKey)
		w := witness_bw6761.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bw6761.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	case *cs_bls24315.SparseR1CS:
		_pk := pk.(*plonk_bls24315.ProvingKey)
		w := witness_bls24315.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return nil, err
		}
		proof, err := plonk_bls24315.Prove(tccs, _pk, w)
		if err != nil {
			return proof, err
		}
		return proof, nil

	default:
		panic("unrecognized R1CS curve type")
	}
}

// ReadAndVerify verifies a PLONK proof from a circuit, associated proving key, and the full witness
func ReadAndVerify(proof Proof, vk VerifyingKey, witness io.Reader) error {

	expectedSize := vk.NbPublicWitness()

	switch _proof := proof.(type) {
	case *plonk_bn254.Proof:
		_vk := vk.(*plonk_bn254.VerifyingKey)
		w := witness_bn254.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bn254.Verify(_proof, _vk, w)

	case *plonk_bls12381.Proof:
		_vk := vk.(*plonk_bls12381.VerifyingKey)
		w := witness_bls12381.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bls12381.Verify(_proof, _vk, w)

	case *plonk_bls12377.Proof:
		_vk := vk.(*plonk_bls12377.VerifyingKey)
		w := witness_bls12377.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bls12377.Verify(_proof, _vk, w)
	case *plonk_bw6633.Proof:
		_vk := vk.(*plonk_bw6633.VerifyingKey)
		w := witness_bw6633.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bw6633.Verify(_proof, _vk, w)


	case *plonk_bw6761.Proof:
		_vk := vk.(*plonk_bw6761.VerifyingKey)
		w := witness_bw6761.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bw6761.Verify(_proof, _vk, w)

	case *plonk_bls24315.Proof:
		_vk := vk.(*plonk_bls24315.VerifyingKey)
		w := witness_bls24315.Witness{}
		if _, err := w.LimitReadFrom(witness, expectedSize); err != nil {
			return err
		}
		return plonk_bls24315.Verify(_proof, _vk, w)

	default:
		panic("unrecognized R1CS curve type")
	}
}
