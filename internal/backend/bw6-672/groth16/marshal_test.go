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

package groth16

import (
	curve "github.com/consensys/gnark-crypto/ecc/bw6-672"

	"github.com/consensys/gnark-crypto/ecc/bw6-672/fr/fft"

	"bytes"
	"math/big"
	"reflect"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"

	"testing"
)

func TestProofSerialization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 1000

	properties := gopter.NewProperties(parameters)

	properties.Property("Proof -> writer -> reader -> Proof should stay constant", prop.ForAll(
		func(ar, krs curve.G1Affine, bs curve.G2Affine) bool {
			var proof, pCompressed, pRaw Proof

			// create a random proof
			proof.Ar = ar
			proof.Krs = krs
			proof.Bs = bs

			var bufCompressed bytes.Buffer
			written, err := proof.WriteTo(&bufCompressed)
			if err != nil {
				return false
			}

			read, err := pCompressed.ReadFrom(&bufCompressed)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			var bufRaw bytes.Buffer
			written, err = proof.WriteRawTo(&bufRaw)
			if err != nil {
				return false
			}

			read, err = pRaw.ReadFrom(&bufRaw)
			if err != nil {
				return false
			}

			if read != written {
				return false
			}

			return reflect.DeepEqual(&proof, &pCompressed) && reflect.DeepEqual(&proof, &pRaw)
		},
		GenG1(),
		GenG1(),
		GenG2(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestVerifyingKeySerialization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10

	properties := gopter.NewProperties(parameters)

	properties.Property("VerifyingKey -> writer -> reader -> VerifyingKey should stay constant", prop.ForAll(
		func(p1 curve.G1Affine, p2 curve.G2Affine) bool {
			var vk, vkCompressed, vkRaw VerifyingKey

			// create a random vk
			nbWires := 6

			vk.G1.Alpha = p1
			vk.G1.Beta = p1
			vk.G1.Delta = p1

			vk.G2.Gamma = p2
			vk.G2.Beta = p2
			vk.G2.Delta = p2

			var err error
			vk.e, err = curve.Pair([]curve.G1Affine{vk.G1.Alpha}, []curve.G2Affine{vk.G2.Beta})
			if err != nil {
				t.Fatal(err)
				return false
			}
			vk.G2.deltaNeg.Neg(&vk.G2.Delta)
			vk.G2.gammaNeg.Neg(&vk.G2.Gamma)

			vk.G1.K = make([]curve.G1Affine, nbWires)
			for i := 0; i < nbWires; i++ {
				vk.G1.K[i] = p1
			}

			var bufCompressed bytes.Buffer
			written, err := vk.WriteTo(&bufCompressed)
			if err != nil {
				t.Log(err)
				return false
			}

			read, err := vkCompressed.ReadFrom(&bufCompressed)
			if err != nil {
				t.Log(err)
				return false
			}

			if read != written {
				t.Log("read != written")
				return false
			}

			var bufRaw bytes.Buffer
			written, err = vk.WriteRawTo(&bufRaw)
			if err != nil {
				t.Log(err)
				return false
			}

			read, err = vkRaw.ReadFrom(&bufRaw)
			if err != nil {
				t.Log(err)
				return false
			}

			if read != written {
				t.Log("read raw != written")
				return false
			}

			return reflect.DeepEqual(&vk, &vkCompressed) && reflect.DeepEqual(&vk, &vkRaw)
		},
		GenG1(),
		GenG2(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProvingKeySerialization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 10

	properties := gopter.NewProperties(parameters)

	properties.Property("ProvingKey -> writer -> reader -> ProvingKey should stay constant", prop.ForAll(
		func(p1 curve.G1Affine, p2 curve.G2Affine) bool {
			var pk, pkCompressed, pkRaw ProvingKey

			// create a random pk
			domain := fft.NewDomain(8, 1, true)
			pk.Domain = *domain

			nbWires := 6
			nbPrivateWires := 4

			// allocate our slices
			pk.G1.A = make([]curve.G1Affine, nbWires)
			pk.G1.B = make([]curve.G1Affine, nbWires)
			pk.G1.K = make([]curve.G1Affine, nbPrivateWires)
			pk.G1.Z = make([]curve.G1Affine, pk.Domain.Cardinality)
			pk.G2.B = make([]curve.G2Affine, nbWires)

			pk.G1.Alpha = p1
			pk.G2.Beta = p2
			pk.G1.K[1] = p1
			pk.G1.B[0] = p1
			pk.G2.B[0] = p2

			var bufCompressed bytes.Buffer
			written, err := pk.WriteTo(&bufCompressed)
			if err != nil {
				t.Log(err)
				return false
			}

			read, err := pkCompressed.ReadFrom(&bufCompressed)
			if err != nil {
				t.Log(err)
				return false
			}

			if read != written {
				t.Log("read != written")
				return false
			}

			var bufRaw bytes.Buffer
			written, err = pk.WriteRawTo(&bufRaw)
			if err != nil {
				t.Log(err)
				return false
			}

			read, err = pkRaw.ReadFrom(&bufRaw)
			if err != nil {
				t.Log(err)
				return false
			}

			if read != written {
				t.Log("read raw != written")
				return false
			}

			return reflect.DeepEqual(&pk, &pkCompressed) && reflect.DeepEqual(&pk, &pkRaw)
		},
		GenG1(),
		GenG2(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func GenG1() gopter.Gen {
	_, _, g1GenAff, _ := curve.Generators()
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		var scalar big.Int
		scalar.SetUint64(genParams.NextUint64())

		var g1 curve.G1Affine
		g1.ScalarMultiplication(&g1GenAff, &scalar)

		genResult := gopter.NewGenResult(g1, gopter.NoShrinker)
		return genResult
	}
}

func GenG2() gopter.Gen {
	_, _, _, g2GenAff := curve.Generators()
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		var scalar big.Int
		scalar.SetUint64(genParams.NextUint64())

		var g2 curve.G2Affine
		g2.ScalarMultiplication(&g2GenAff, &scalar)

		genResult := gopter.NewGenResult(g2, gopter.NoShrinker)
		return genResult
	}
}
