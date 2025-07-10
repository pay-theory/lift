package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-org/your-repo/pkg/security" // Replace with actual import path
)

func main() {
	// Parse command line flags
	basePath := flag.String("path", ".", "Base path to scan")
	outputFile := flag.String("output", "", "Output file for results (optional)")
	verboseMode := flag.Bool("verbose", false, "Verbose output mode")
	flag.Parse()

	// Resolve absolute path
	absPath, err := filepath.Abs(*basePath)
	if err != nil {
		log.Fatalf("Error resolving path %s: %v", *basePath, err)
	}

	fmt.Printf("Scanning for STIBs (Security Threats In Backend) in %s\n", absPath)

	// Create and run scanner
	scanner := security.NewStibScanner(absPath)
	err = scanner.Scan()
	if err != nil {
		log.Fatalf("Error scanning for security issues: %v", err)
	}

	// Get results
	results := scanner.GetResults()

	// Print summary
	fmt.Printf("\nScan completed. Found %d potential security issues.\n", len(results))
	
	// Print results if verbose mode is enabled
	if *verboseMode {
		scanner.PrintResults()
	}

	// Write results to file if output file is specified
	if *outputFile != "" {
		writeResultsToFile(results, *outputFile)
	}

	// Print summary by type
	printSummaryByType(results)
}

func printSummaryByType(results []security.Stib) {
	typeCount := make(map[security.StibType]int)
	severityCount := make(map[string]int)
	
	for _, result := range results {
		typeCount[result.Type]++
		severityCount[result.Severity]++
	}

	fmt.Println("\nSummary by Type:")
	for typ, count := range typeCount {
		fmt.Printf("- %s: %d\n", typ, count)
	}

	fmt.Println("\nSummary by Severity:")
	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		if count, ok := severityCount[severity]; ok {
			fmt.Printf("- %s: %d\n", severity, count)
		}
	}
}

func writeResultsToFile(results []security.Stib, outputFile string) {
	file, err := os.Create(outputFile)
	if err != nil {
		log.Printf("Error creating output file: %v", err)
		return
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("# STIB Scan Results\n\n"))
	file.WriteString(fmt.Sprintf("Found %d potential security issues\n\n", len(results)))

	for i, stib := range results {
		file.WriteString(fmt.Sprintf("## %d. [%s] %s\n", i+1, stib.Severity, stib.Description))
		file.WriteString(fmt.Sprintf("- **File**: %s:%d\n", stib.FilePath, stib.LineNumber))
		file.WriteString(fmt.Sprintf("- **Type**: %s\n", stib.Type))
		file.WriteString(fmt.Sprintf("- **Code**: `%s`\n\n", strings.Replace(stib.Code, "`", "\\`", -1)))
	}

	fmt.Printf("Results written to %s\n", outputFile)
}