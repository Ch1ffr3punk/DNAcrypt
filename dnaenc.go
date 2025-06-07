package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

func encodeBytesToDNA(input io.Reader, output io.Writer, wrapWidth int) error {
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)

	charsWrittenInLine := 0

	var err error

	for {
		var b byte
		b, err = reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading bytes: %w", err)
		}

		for i := 3; i >= 0; i-- {
			bits := (b >> (i * 2)) & 0b11
			var dnaBase byte

			switch bits {
			case 0b00:
				dnaBase = 'A'
			case 0b01:
				dnaBase = 'C'
			case 0b10:
				dnaBase = 'G'
			case 0b11:
				dnaBase = 'T'
			}

			err = writer.WriteByte(dnaBase)
			if err != nil {
				return fmt.Errorf("error writing DNA base: %w", err)
			}
			charsWrittenInLine++

			if wrapWidth > 0 && charsWrittenInLine >= wrapWidth {
				err = writer.WriteByte('\n')
				if err != nil {
					return fmt.Errorf("error writing newline: %w", err)
				}
				charsWrittenInLine = 0
			}
		}
	}

	if wrapWidth > 0 && charsWrittenInLine > 0 {
		err = writer.WriteByte('\n')
		if err != nil {
			return fmt.Errorf("error writing final newline: %w", err)
		}
	}

	return writer.Flush()
}

func decodeDNAToBytes(input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)

	var currentByte byte
	bitsCollected := 0

	var err error

	for {
		var dnaBaseByte byte
		dnaBaseByte, err = reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				if bitsCollected != 0 {
					return fmt.Errorf("odd number of bits at end of DNA sequence. Possible corruption")
				}
				break
			}
			return fmt.Errorf("error reading DNA base: %w", err)
		}

		var bits byte
		switch dnaBaseByte {
		case 'A', 'a':
			bits = 0b00
		case 'C', 'c':
			bits = 0b01
		case 'G', 'g':
			bits = 0b10
		case 'T', 't':
			bits = 0b11
		case '\n', '\r', ' ':
			continue
		default:
			return fmt.Errorf("invalid DNA base found: '%c' (ASCII %d)", dnaBaseByte, dnaBaseByte)
		}

		currentByte = (currentByte << 2) | bits
		bitsCollected += 2

		if bitsCollected == 8 {
			err = writer.WriteByte(currentByte)
			if err != nil {
				return fmt.Errorf("error writing byte: %w", err)
			}
			currentByte = 0
			bitsCollected = 0
		}
	}

	return writer.Flush()
}

func main() {
	decodeFlag := flag.Bool("d", false, "Decodes input from DNA sequence to binary data")
	wrapWidthFlag := flag.Int("w", 64, "Sets the number of characters per line for encoding (0 = no wrapping)")

	flag.Parse()

	if *decodeFlag {
		if err := decodeDNAToBytes(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := encodeBytesToDNA(os.Stdin, os.Stdout, *wrapWidthFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding: %v\n", err)
			os.Exit(1)
		}
	}
}
