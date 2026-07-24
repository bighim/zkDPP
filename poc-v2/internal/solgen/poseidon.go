package solgen

import (
	"fmt"
	"io"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	poseidon2 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr/poseidon2"
)

func WritePoseidon2BLS12381(writer io.Writer) error {
	params := poseidon2.GetDefaultParameters()
	if params.Width != 2 || params.NbFullRounds != 6 || params.NbPartialRounds != 50 {
		return fmt.Errorf("unexpected Poseidon2 parameters: width=%d full=%d partial=%d", params.Width, params.NbFullRounds, params.NbPartialRounds)
	}
	fprintf := func(format string, args ...any) {
		_, _ = fmt.Fprintf(writer, format, args...)
	}
	fprintf("// SPDX-License-Identifier: Apache-2.0\n")
	fprintf("// Code generated from gnark-crypto v0.20.1 BLS12-381 Poseidon2 parameters. DO NOT EDIT.\n")
	fprintf("pragma solidity ^0.8.30;\n\n")
	fprintf("contract Poseidon2BLS12381 {\n")
	fprintf("    uint256 internal constant MODULUS = 0x%s;\n\n", fr.Modulus().Text(16))
	fprintf("    function _pow5(uint256 value) private pure returns (uint256) {\n")
	fprintf("        uint256 squared = mulmod(value, value, MODULUS);\n")
	fprintf("        uint256 fourth = mulmod(squared, squared, MODULUS);\n")
	fprintf("        return mulmod(fourth, value, MODULUS);\n")
	fprintf("    }\n\n")
	fprintf("    function compress(uint256 left, uint256 right) external pure returns (uint256) {\n")
	fprintf("        require(left < MODULUS && right < MODULUS, \"non-canonical field element\");\n")
	fprintf("        uint256 x0 = left;\n")
	fprintf("        uint256 x1 = right;\n")
	writeExternalMatrix(fprintf)
	fullHalf := params.NbFullRounds / 2
	for round := 0; round < fullHalf; round++ {
		writeFullRound(fprintf, params.RoundKeys[round])
	}
	for round := fullHalf; round < fullHalf+params.NbPartialRounds; round++ {
		writePartialRound(fprintf, params.RoundKeys[round][0])
	}
	for round := fullHalf + params.NbPartialRounds; round < params.NbFullRounds+params.NbPartialRounds; round++ {
		writeFullRound(fprintf, params.RoundKeys[round])
	}
	fprintf("        return addmod(x1, right, MODULUS);\n")
	fprintf("    }\n")
	fprintf("}\n")
	return nil
}

func hexElement(value fr.Element) string {
	var asBigInt big.Int
	value.BigInt(&asBigInt)
	return fmt.Sprintf("0x%064x", &asBigInt)
}

func writeExternalMatrix(fprintf func(string, ...any)) {
	fprintf("        {\n")
	fprintf("            uint256 sum = addmod(x0, x1, MODULUS);\n")
	fprintf("            x0 = addmod(sum, x0, MODULUS);\n")
	fprintf("            x1 = addmod(sum, x1, MODULUS);\n")
	fprintf("        }\n")
}

func writeFullRound(fprintf func(string, ...any), keys []fr.Element) {
	fprintf("        x0 = addmod(x0, %s, MODULUS);\n", hexElement(keys[0]))
	fprintf("        x1 = addmod(x1, %s, MODULUS);\n", hexElement(keys[1]))
	fprintf("        x0 = _pow5(x0);\n")
	fprintf("        x1 = _pow5(x1);\n")
	writeExternalMatrix(fprintf)
}

func writePartialRound(fprintf func(string, ...any), key fr.Element) {
	fprintf("        x0 = addmod(x0, %s, MODULUS);\n", hexElement(key))
	fprintf("        x0 = _pow5(x0);\n")
	fprintf("        {\n")
	fprintf("            uint256 sum = addmod(x0, x1, MODULUS);\n")
	fprintf("            uint256 next0 = addmod(x0, sum, MODULUS);\n")
	fprintf("            uint256 next1 = addmod(addmod(x1, x1, MODULUS), sum, MODULUS);\n")
	fprintf("            x0 = next0;\n")
	fprintf("            x1 = next1;\n")
	fprintf("        }\n")
}
