package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// Counts holds the line, word, and byte counts.
type Counts struct {
	Lines int64
	Words int64
	Bytes int64
}

// Flags holds the boolean flags indicating which counts to display.
type Flags struct {
	ShowLines bool
	ShowWords bool
	ShowBytes bool
}

const (
	// Define a large buffer size for efficient reading.
	// 64KB is often a good balance. Adjust based on profiling if needed.
	bufferSize = 64 * 1024
)

// count performs the counting operation on the given reader.
// It's optimized by reading in large chunks and processing the buffer.
func count(reader io.Reader) (Counts, error) {
	var counts Counts
	// Use bufio.Reader with a specified large buffer size for performance.
	br := bufio.NewReaderSize(reader, bufferSize)
	buf := make([]byte, bufferSize) // Reusable buffer for Read calls

	inWord := false // State machine: are we currently inside a word?

	for {
		// Read a chunk from the buffered reader into our local buffer.
		// This minimizes the number of underlying system calls.
		n, err := br.Read(buf)

		// Always count bytes read, even if there's an error (like EOF)
		counts.Bytes += int64(n)

		// Process the chunk that was just read
		for i := 0; i < n; i++ {
			char := buf[i]

			// Count lines (efficiently check for newline)
			if char == '\n' {
				counts.Lines++
			}

			// Count words using a state machine
			// Consider any Unicode space character as a separator.
			// Cast byte to rune for unicode.IsSpace
			isSpace := unicode.IsSpace(rune(char))
			if isSpace {
				inWord = false
			} else {
				// If we were not in a word before, and current char is not space,
				// it marks the beginning of a new word.
				if !inWord {
					counts.Words++
					inWord = true
				}
			}
		}

		// Handle read errors
		if err != nil {
			if err == io.EOF {
				break // End of file reached, exit loop normally
			}
			// An actual read error occurred
			return counts, fmt.Errorf("error reading input: %w", err)
		}
	}

	return counts, nil
}

// formatOutput formats the counts according to the selected flags for printing.
// It mimics the right-aligned output of standard wc.
func formatOutput(counts Counts, flags Flags, filename string) string {
	var parts []string
	// Use a consistent width for alignment (e.g., 8 characters)
	const width = 8

	if flags.ShowLines {
		parts = append(parts, fmt.Sprintf("%*d", width, counts.Lines))
	}
	if flags.ShowWords {
		parts = append(parts, fmt.Sprintf("%*d", width, counts.Words))
	}
	if flags.ShowBytes {
		parts = append(parts, fmt.Sprintf("%*d", width, counts.Bytes))
	}

	// Add filename if provided
	if filename != "" {
		// Add a space separator before the filename
		parts = append(parts, " "+filename)
	}

	return strings.Join(parts, "")
}

func main() {
	// --- 1. Define and Parse Command Line Flags ---
	var flags Flags
	flag.BoolVar(&flags.ShowLines, "l", false, "print the newline counts")
	flag.BoolVar(&flags.ShowWords, "w", false, "print the word counts")
	flag.BoolVar(&flags.ShowBytes, "c", false, "print the byte counts")
	// Note: Standard wc also has -m for character count, which is different from -c for bytes
	// if the input contains multi-byte characters. We are implementing -c (bytes).

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-clw] [file ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Print newline, word, and byte counts for each FILE, and a total line if\n")
		fmt.Fprintf(os.Stderr, "more than one FILE is specified. With no FILE, or when FILE is -, read standard input.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// If no specific count flag is provided, default to showing all three
	if !flags.ShowLines && !flags.ShowWords && !flags.ShowBytes {
		flags.ShowLines = true
		flags.ShowWords = true
		flags.ShowBytes = true
	}

	// --- 2. Determine Input Source(s) ---
	filenames := flag.Args()
	var totalCounts Counts
	var filesProcessed int
	var errorsOccurred bool

	// --- 3. Process Input ---
	if len(filenames) == 0 {
		// Read from standard input
		counts, err := count(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
			os.Exit(1)
		}
		fmt.Println(formatOutput(counts, flags, "")) // No filename for stdin
		filesProcessed = 1                           // Consider stdin as one "file" processed
		totalCounts = counts                         // For consistency, although total isn't printed for single stdin
	} else {
		// Process each file provided as argument
		for _, filename := range filenames {
			var currentReader io.Reader
			var file *os.File
			var err error

			// Handle "-" as stdin explicitly
			if filename == "-" {
				currentReader = os.Stdin
				filename = "" // Use empty string to signify stdin for output formatting
			} else {
				file, err = os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], filename, err)
					errorsOccurred = true
					continue // Skip to the next file
				}
				// Ensure file is closed even if counting fails partially
				defer file.Close()
				currentReader = file
			}

			counts, err := count(currentReader)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], filename, err)
				errorsOccurred = true
				// If it was a file, close it now as the deferred close won't run if we continue
				if file != nil {
					file.Close()
				}
				continue // Skip to the next file
			}

			// Close the file manually if it was opened (deferred close handles the happy path)
			// No need to explicitly close here if using defer correctly.
			// if file != nil {
			//     file.Close() // Already deferred
			// }

			// Print counts for the current file
			fmt.Println(formatOutput(counts, flags, filename))

			// Add to totals
			totalCounts.Lines += counts.Lines
			totalCounts.Words += counts.Words
			totalCounts.Bytes += counts.Bytes
			filesProcessed++
		}

		// --- 4. Print Total (if multiple files were processed) ---
		if filesProcessed > 1 {
			fmt.Println(formatOutput(totalCounts, flags, "total"))
		}
	}

	// Exit with non-zero status if any errors occurred during file processing
	if errorsOccurred {
		os.Exit(1)
	}
}
