package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
)

var dnaGrayscaleMap = map[rune]uint8{
	'A': 0,
	'T': 64,
	'C': 128,
	'G': 192,
}

type Config struct {
	BlockSize      int
	BlocksPerRow   int
	RowsPerBlock   int
	DecodeMode     bool
	InputFile      string
	OutputFile     string
	UseStdio       bool
	PaddingChar    string
	TransparentPad bool
}

func parseFlags() Config {
	var cfg Config

	flag.IntVar(&cfg.BlockSize, "b", 10, "Block size in pixels")
	flag.IntVar(&cfg.BlocksPerRow, "cols", 40, "Number of blocks per row")
	flag.IntVar(&cfg.RowsPerBlock, "rows", 1, "Number of rows per DNA block")
	flag.BoolVar(&cfg.DecodeMode, "d", false, "Decode mode (PNG to DNA)")
	flag.StringVar(&cfg.InputFile, "i", "", "Input file (default: stdin)")
	flag.StringVar(&cfg.OutputFile, "o", "", "Output file (default: stdout)")
	flag.BoolVar(&cfg.UseStdio, "stdio", false, "Force stdin/stdout usage")
	flag.StringVar(&cfg.PaddingChar, "pad", "N", "Character for padding (default: N)")
	flag.BoolVar(&cfg.TransparentPad, "transpad", true, "Use transparent padding")

	flag.Parse()

	if cfg.InputFile != "" || cfg.OutputFile != "" {
		cfg.UseStdio = false
	}

	return cfg
}

func findClosestDNABaseGrayscale(intensity uint8, tolerance uint8) (rune, error) {
	minDiff := 256
	var closestBase rune

	for base, mappedIntensity := range dnaGrayscaleMap {
		diff := 0
		if intensity > mappedIntensity {
			diff = int(intensity - mappedIntensity)
		} else {
			diff = int(mappedIntensity - intensity)
		}

		if diff < minDiff {
			minDiff = diff
			closestBase = base
		}
	}

	if uint8(minDiff) > tolerance {
		return '?', fmt.Errorf("grayscale intensity (%d) is too far from any known DNA base intensity (minDiff: %d)", intensity, minDiff)
	}

	return closestBase, nil
}

func dnaToPNGGrayscale(dnaSequence string, writer io.Writer, cfg Config) error {
	if cfg.BlockSize <= 0 {
		return fmt.Errorf("block size must be positive")
	}
	if cfg.BlocksPerRow <= 0 {
		return fmt.Errorf("blocks per row must be positive")
	}
	if cfg.RowsPerBlock <= 0 {
		return fmt.Errorf("rows per block must be positive")
	}

	// Create RGBA image for transparency support
	basesPerBlock := cfg.BlocksPerRow * cfg.RowsPerBlock
	numBlocks := (len(dnaSequence) + basesPerBlock - 1) / basesPerBlock
	imgWidth := cfg.BlocksPerRow * cfg.BlockSize
	imgHeight := numBlocks * cfg.RowsPerBlock * cfg.BlockSize

	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	// Fill with transparent background if enabled
	if cfg.TransparentPad {
		for y := 0; y < imgHeight; y++ {
			for x := 0; x < imgWidth; x++ {
				img.Set(x, y, color.Transparent)
			}
		}
	}

	for block := 0; block < numBlocks; block++ {
		start := block * basesPerBlock
		end := start + basesPerBlock
		if end > len(dnaSequence) {
			end = len(dnaSequence)
		}
		blockSeq := dnaSequence[start:end]

		for i, base := range blockSeq {
			col := i % cfg.BlocksPerRow
			row := (i / cfg.BlocksPerRow) + (block * cfg.RowsPerBlock)

			x := col * cfg.BlockSize
			y := row * cfg.BlockSize

			grayVal := dnaGrayscaleMap[base]
			c := color.RGBA{R: grayVal, G: grayVal, B: grayVal, A: 255}

			for dx := 0; dx < cfg.BlockSize; dx++ {
				for dy := 0; dy < cfg.BlockSize; dy++ {
					img.Set(x+dx, y+dy, c)
				}
			}
		}
	}

	return png.Encode(writer, img)
}

func pngToDNAGrayscale(reader io.Reader, writer io.Writer, cfg Config) error {
	if cfg.BlockSize <= 0 {
		return fmt.Errorf("block size must be positive")
	}

	img, err := png.Decode(reader)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	imgWidth := bounds.Max.X
	imgHeight := bounds.Max.Y

	if imgWidth%cfg.BlockSize != 0 || imgHeight%cfg.BlockSize != 0 {
		return fmt.Errorf("image dimensions (%dx%d) are not a multiple of block size (%d)", imgWidth, imgHeight, cfg.BlockSize)
	}

	blocksPerRow := imgWidth / cfg.BlockSize
	rowsOfBlocks := imgHeight / cfg.BlockSize

	var dnaBuilder strings.Builder
	tolerance := uint8(20)
	paddingChar := rune(cfg.PaddingChar[0])

	for blockRow := 0; blockRow < rowsOfBlocks; blockRow += cfg.RowsPerBlock {
		for col := 0; col < blocksPerRow; col++ {
			for rowInBlock := 0; rowInBlock < cfg.RowsPerBlock; rowInBlock++ {
				if blockRow+rowInBlock >= rowsOfBlocks {
					break
				}

				pixelX := col * cfg.BlockSize
				pixelY := (blockRow + rowInBlock) * cfg.BlockSize

				// Handle transparency
				r, g, b, a := img.At(pixelX, pixelY).RGBA()
				if a < 0xFFFF/2 { // If pixel is semi-transparent
					dnaBuilder.WriteRune(paddingChar)
					continue
				}

				intensity := uint8((r + g + b) / 3 / 257)
				base, err := findClosestDNABaseGrayscale(intensity, tolerance)
				if err != nil {
					return fmt.Errorf("error decoding pixel at (%d,%d): %w", pixelX, pixelY, err)
				}
				dnaBuilder.WriteRune(base)
			}
		}
	}

	_, err = writer.Write([]byte(dnaBuilder.String()))
	return err
}

func validateDNA(sequence string, paddingChar rune) bool {
	for _, c := range sequence {
		if c != 'A' && c != 'T' && c != 'C' && c != 'G' && c != paddingChar {
			return false
		}
	}
	return true
}

func getReader(cfg Config) (io.Reader, error) {
	if cfg.InputFile == "" || cfg.UseStdio {
		return os.Stdin, nil
	}
	return os.Open(cfg.InputFile)
}

func getWriter(cfg Config) (io.Writer, error) {
	if cfg.OutputFile == "" || cfg.UseStdio {
		return os.Stdout, nil
	}
	return os.Create(cfg.OutputFile)
}

func main() {
	cfg := parseFlags()

	reader, err := getReader(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input: %v\n", err)
		os.Exit(1)
	}

	writer, err := getWriter(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening output: %v\n", err)
		os.Exit(1)
	}

	paddingChar := rune(cfg.PaddingChar[0])

	if cfg.DecodeMode {
		err := pngToDNAGrayscale(reader, writer, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		data, err := io.ReadAll(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			os.Exit(1)
		}

		sequence := strings.ToUpper(strings.TrimSpace(string(data)))
		if !validateDNA(sequence, paddingChar) {
			fmt.Fprintf(os.Stderr, "Error: Invalid DNA sequence. Only A, T, C, G and %c are allowed.\n", paddingChar)
			os.Exit(1)
		}

		err = dnaToPNGGrayscale(sequence, writer, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
