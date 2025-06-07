package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/google/go-tpm/legacy/tpm2"
)

func byteToDNA(b byte) string {
	var dnaSequence strings.Builder
	for i := 3; i >= 0; i-- {
		twoBits := (b >> (i * 2)) & 0x03
		switch twoBits {
		case 0:
			dnaSequence.WriteString("A")
		case 1:
			dnaSequence.WriteString("T")
		case 2:
			dnaSequence.WriteString("C")
		case 3:
			dnaSequence.WriteString("G")
		}
	}
	return dnaSequence.String()
}

func dnaToByte(dna string) (byte, error) {
	if len(dna) != 4 {
		return 0, fmt.Errorf("DNA sequence must be 4 bases long, but is %d", len(dna))
	}
	var b byte
	for _, r := range dna {
		var val byte
		switch r {
		case 'A', 'a':
			val = 0
		case 'T', 't':
			val = 1
		case 'C', 'c':
			val = 2
		case 'G', 'g':
			val = 3
		default:
			return 0, fmt.Errorf("invalid DNA base: %c", r)
		}
		b = (b << 2) | val
	}
	return b, nil
}

func textToDNA(text string) string {
	var dnaBuilder strings.Builder
	for _, char := range []byte(text) {
		dnaBuilder.WriteString(byteToDNA(char))
	}
	return dnaBuilder.String()
}

func dnaToText(dna string) (string, error) {
	if len(dna)%4 != 0 {
		return "", fmt.Errorf("DNA sequence length must be a multiple of 4")
	}
	var textBuilder strings.Builder
	for i := 0; i < len(dna); i += 4 {
		dnaSegment := dna[i : i+4]
		b, err := dnaToByte(dnaSegment)
		if err != nil {
			return "", err
		}
		textBuilder.WriteByte(b)
	}
	return textBuilder.String(), nil
}

func dnaBaseToBinary(base rune) (byte, error) {
	switch base {
	case 'A', 'a':
		return 0, nil
	case 'T', 't':
		return 1, nil
	case 'C', 'c':
		return 2, nil
	case 'G', 'g':
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid DNA base: %c", base)
	}
}

func binaryToDNABase(val byte) (rune, error) {
	switch val & 0x03 {
	case 0:
		return 'A', nil
	case 1:
		return 'T', nil
	case 2:
		return 'C', nil
	case 3:
		return 'G', nil
	default:
		return '?', fmt.Errorf("invalid binary value for DNA base: %d", val)
	}
}

func xorDNAStrings(dna1, dna2 string) (string, error) {
	if len(dna1) != len(dna2) {
		return "", fmt.Errorf("DNA strings must have the same length")
	}

	var resultBuilder strings.Builder
	for i := 0; i < len(dna1); i++ {
		b1, err := dnaBaseToBinary(rune(dna1[i]))
		if err != nil {
			return "", fmt.Errorf("error in DNA string 1 at index %d: %w", i, err)
		}
		b2, err := dnaBaseToBinary(rune(dna2[i]))
		if err != nil {
			return "", fmt.Errorf("error in DNA string 2 at index %d: %w", i, err)
		}

		xorResult := b1 ^ b2
		resBase, err := binaryToDNABase(xorResult)
		if err != nil {
			return "", fmt.Errorf("error converting XOR result to base: %w", err)
		}
		resultBuilder.WriteString(string(resBase))
	}
	return resultBuilder.String(), nil
}

func printUsage() {
	fmt.Println("Usage: dnacrypt <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  encrypt <plaintext_file> <key_file> <output_file>")
	fmt.Println("  decrypt <ciphertext_file> <key_file> <output_file>")
	fmt.Println("  genkey <length> <output_file>")
	fmt.Println("    Generates a random DNA key of the specified length (in bases)\n    using TPM 2.0 and saves it to <output_file>.")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "encrypt":
		if len(os.Args) != 5 {
			fmt.Println("Error: incorrect number of arguments for 'encrypt'.")
			printUsage()
			os.Exit(1)
		}
		plaintextFile := os.Args[2]
		keyFile := os.Args[3]
		outputFile := os.Args[4]

		plaintextBytes, err := ioutil.ReadFile(plaintextFile)
		if err != nil {
			log.Fatalf("Error reading plaintext: %v", err)
		}
		plaintext := string(plaintextBytes)

		dnaPlaintext := textToDNA(plaintext)
		fmt.Printf("Plaintext as DNA (first 20 bases): %s...\n", dnaPlaintext[:min(len(dnaPlaintext), 20)])

		keyDNABytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			log.Fatalf("Error reading key: %v", err)
		}
		keyDNA := strings.TrimSpace(string(keyDNABytes))

		if len(keyDNA) < len(dnaPlaintext) {
			log.Fatalf("Error: key is too short. Needs %d bases, has %d bases.", len(dnaPlaintext), len(keyDNA))
		}
		keyDNA = keyDNA[:len(dnaPlaintext)]

		ciphertextDNA, err := xorDNAStrings(dnaPlaintext, keyDNA)
		if err != nil {
			log.Fatalf("Error during encryption: %v", err)
		}

		err = ioutil.WriteFile(outputFile, []byte(ciphertextDNA), 0644)
		if err != nil {
			log.Fatalf("Error saving ciphertext: %v", err)
		}
		fmt.Printf("Encryption complete. Ciphertext DNA saved to '%s'.\n", outputFile)

	case "decrypt":
		if len(os.Args) != 5 {
			fmt.Println("Error: incorrect number of arguments for 'decrypt'.")
			printUsage()
			os.Exit(1)
		}
		ciphertextFile := os.Args[2]
		keyFile := os.Args[3]
		outputFile := os.Args[4]

		ciphertextDNABytes, err := ioutil.ReadFile(ciphertextFile)
		if err != nil {
			log.Fatalf("Error reading ciphertext: %v", err)
		}
		ciphertextDNA := strings.TrimSpace(string(ciphertextDNABytes))

		keyDNABytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			log.Fatalf("Error reading key: %v", err)
		}
		keyDNA := strings.TrimSpace(string(keyDNABytes))

		if len(keyDNA) != len(ciphertextDNA) {
			log.Fatalf("Error: key length (%d) and ciphertext length (%d) do not match.", len(keyDNA), len(ciphertextDNA))
		}

		decryptedDNAPlaintext, err := xorDNAStrings(ciphertextDNA, keyDNA)
		if err != nil {
			log.Fatalf("Error during decryption: %v", err)
		}

		decryptedText, err := dnaToText(decryptedDNAPlaintext)
		if err != nil {
			log.Fatalf("Error decoding DNA to text: %v", err)
		}

		err = ioutil.WriteFile(outputFile, []byte(decryptedText), 0644)
		if err != nil {
			log.Fatalf("Error saving decrypted text: %v", err)
		}
		fmt.Printf("Decryption complete. Decrypted text saved to '%s'.\n", outputFile)

	case "genkey":
		if len(os.Args) != 4 {
			fmt.Println("Error: incorrect number of arguments for 'genkey'.")
			printUsage()
			os.Exit(1)
		}
		keyLengthStr := os.Args[2]
		outputFile := os.Args[3]

		var keyLength int
		_, err := fmt.Sscanf(keyLengthStr, "%d", &keyLength)
		if err != nil || keyLength <= 0 {
			log.Fatalf("Invalid key length: %s. Must be a positive integer.", keyLengthStr)
		}
		
		rwc, err := tpm2.OpenTPM()
		if err != nil {
			rwc, err = tpm2.OpenTPM()
			if err != nil {
				log.Fatalf("Error opening TPM: %v. Ensure TPM module is loaded and accessible.", err)
			}
		}
		defer rwc.Close()

		var keyBuilder strings.Builder
		remaining := keyLength
		
		for remaining > 0 {
			bytesNeeded := (remaining + 3) / 4
			if bytesNeeded > 1024 {
				bytesNeeded = 1024
			}
			
			tpmRandomBytes, err := tpm2.GetRandom(rwc, uint16(bytesNeeded))
			if err != nil {
				log.Fatalf("Error getting random bytes from TPM: %v", err)
			}
			
			for _, b := range tpmRandomBytes {
				keyBuilder.WriteString(byteToDNA(b))
			}
			
			generated := keyBuilder.Len()
			if generated >= keyLength {
				break
			}
			remaining = keyLength - generated
		}

		keyDNA := keyBuilder.String()[:keyLength]

		err = ioutil.WriteFile(outputFile, []byte(keyDNA), 0644)
		if err != nil {
			log.Fatalf("Error saving key: %v", err)
		}
		fmt.Printf("Random DNA key of length %d generated using TPM 2.0 and saved to '%s'.\n", keyLength, outputFile)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}