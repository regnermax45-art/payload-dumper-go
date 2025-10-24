package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func extractPayloadBin(filename string) string {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		log.Fatalf("Not a valid zip archive: %s\n", filename)
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if file.Name == "payload.bin" && file.UncompressedSize64 > 0 {
			zippedFile, err := file.Open()
			if err != nil {
				log.Fatalf("Failed to read zipped file: %s\n", file.Name)
			}

			tempfile, err := os.CreateTemp(os.TempDir(), "payload_*.bin")
			if err != nil {
				log.Fatalf("Failed to create a temp file located at %s\n", tempfile.Name())
			}
			defer tempfile.Close()

			_, err = io.Copy(tempfile, zippedFile)
			if err != nil {
				log.Fatal(err)
			}

			return tempfile.Name()
		}
	}

	return ""
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var (
		list            bool
		partitions      string
		outputDirectory string
		concurrency     int
		// New dual payload options
		portingMode     bool
		portingStrategy string
		partitionMap    string
		generateReport  bool
		detailedReport  bool
	)

	flag.IntVar(&concurrency, "c", 4, "Number of multiple workers to extract (shorthand)")
	flag.IntVar(&concurrency, "concurrency", 4, "Number of multiple workers to extract")
	flag.BoolVar(&list, "l", false, "Show list of partitions in payload.bin (shorthand)")
	flag.BoolVar(&list, "list", false, "Show list of partitions in payload.bin")
	flag.StringVar(&outputDirectory, "o", "", "Set output directory (shorthand)")
	flag.StringVar(&outputDirectory, "output", "", "Set output directory")
	flag.StringVar(&partitions, "p", "", "Dump only selected partitions (comma-separated) (shorthand)")
	flag.StringVar(&partitions, "partitions", "", "Dump only selected partitions (comma-separated)")
	
	// New dual payload flags
	flag.BoolVar(&portingMode, "port", false, "Enable ROM porting mode (requires 2 payload files)")
	flag.StringVar(&portingStrategy, "strategy", "priority", "Porting strategy: priority, size, selective, hybrid")
	flag.StringVar(&partitionMap, "partition-map", "", "Partition mapping for selective strategy (format: partition1:payload1,partition2:payload2)")
	flag.BoolVar(&generateReport, "report", true, "Generate porting report")
	flag.BoolVar(&detailedReport, "detailed-report", false, "Print detailed porting report")
	
	flag.Parse()

	// Check for dual payload mode
	if portingMode {
		if flag.NArg() < 2 {
			fmt.Fprintf(os.Stderr, "Error: ROM porting mode requires 2 payload files\n")
			usage()
		}
		handleDualPayloadMode(flag.Arg(0), flag.Arg(1), outputDirectory, concurrency, 
			portingStrategy, partitionMap, generateReport, detailedReport, list)
		return
	}

	// Original single payload mode
	if flag.NArg() == 0 {
		usage()
	}
	filename := flag.Arg(0)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s\n", filename)
	}

	payloadBin := filename
	if strings.HasSuffix(filename, ".zip") {
		fmt.Println("Please wait while extracting payload.bin from the archive.")
		payloadBin = extractPayloadBin(filename)
		if payloadBin == "" {
			log.Fatal("Failed to extract payload.bin from the archive.")
		} else {
			defer os.Remove(payloadBin)
		}
	}
	fmt.Printf("payload.bin: %s\n", payloadBin)

	payload := NewPayload(payloadBin)
	if err := payload.Open(); err != nil {
		log.Fatal(err)
	}
	payload.Init()

	if list {
		return
	}

	now := time.Now()

	targetDirectory := outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("extracted_%d%02d%02d_%02d%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}
	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		if err := os.Mkdir(targetDirectory, 0o755); err != nil {
			log.Fatal("Failed to create target directory")
		}
	}

	payload.SetConcurrency(concurrency)
	fmt.Printf("Number of workers: %d\n", payload.GetConcurrency())

	if partitions != "" {
		if err := payload.ExtractSelected(targetDirectory, strings.Split(partitions, ",")); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := payload.ExtractAll(targetDirectory); err != nil {
			log.Fatal(err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  Single payload mode: %s [options] [inputfile]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  ROM porting mode:    %s -port [options] [payload1] [payload2]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nROM Porting Strategies:\n")
	fmt.Fprintf(os.Stderr, "  priority   - Payload1 takes precedence (default)\n")
	fmt.Fprintf(os.Stderr, "  size       - Use larger partition when both exist\n")
	fmt.Fprintf(os.Stderr, "  selective  - Use partition mapping (requires -partition-map)\n")
	fmt.Fprintf(os.Stderr, "  hybrid     - Intelligent selection based on partition type\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  %s payload.bin\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -port rom1.bin rom2.bin\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -port -strategy size rom1.bin rom2.bin\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -port -strategy selective -partition-map \"system:payload2,vendor:payload1\" rom1.bin rom2.bin\n", os.Args[0])
	os.Exit(2)
}

// handleDualPayloadMode processes two payload files for ROM porting
func handleDualPayloadMode(payload1Path, payload2Path, outputDirectory string, concurrency int, 
	strategyStr, partitionMapStr string, generateReport, detailedReport, listOnly bool) {
	
	fmt.Println("🚀 ROM Porting Mode Activated")
	fmt.Printf("Payload 1: %s\n", payload1Path)
	fmt.Printf("Payload 2: %s\n", payload2Path)

	// Validate input files
	if _, err := os.Stat(payload1Path); os.IsNotExist(err) {
		log.Fatalf("Payload 1 file does not exist: %s\n", payload1Path)
	}
	if _, err := os.Stat(payload2Path); os.IsNotExist(err) {
		log.Fatalf("Payload 2 file does not exist: %s\n", payload2Path)
	}

	// Process payload files (extract from zip if needed)
	processedPayload1 := processPayloadFile(payload1Path, "payload1")
	processedPayload2 := processPayloadFile(payload2Path, "payload2")
	
	if processedPayload1 != payload1Path {
		defer os.Remove(processedPayload1)
	}
	if processedPayload2 != payload2Path {
		defer os.Remove(processedPayload2)
	}

	// Create and initialize payloads
	payload1 := NewPayload(processedPayload1)
	payload2 := NewPayload(processedPayload2)

	if err := payload1.Open(); err != nil {
		log.Fatalf("Failed to open payload1: %v", err)
	}
	if err := payload2.Open(); err != nil {
		log.Fatalf("Failed to open payload2: %v", err)
	}

	if err := payload1.Init(); err != nil {
		log.Fatalf("Failed to initialize payload1: %v", err)
	}
	if err := payload2.Init(); err != nil {
		log.Fatalf("Failed to initialize payload2: %v", err)
	}

	// Create dual payload processor
	dpp := NewDualPayloadProcessor(payload1, payload2)
	dpp.SetConcurrency(concurrency)

	// Parse and set strategy
	strategy := parsePortingStrategy(strategyStr)
	dpp.SetStrategy(strategy)

	// Parse partition mapping for selective strategy
	if strategy == StrategySelective && partitionMapStr != "" {
		partitionMap := parsePartitionMap(partitionMapStr)
		dpp.SetPartitionMap(partitionMap)
	}

	// Analyze partitions
	if err := dpp.AnalyzePartitions(); err != nil {
		log.Fatalf("Failed to analyze partitions: %v", err)
	}

	// Print porting plan
	dpp.PrintPortingPlan()

	// If list only mode, exit here
	if listOnly {
		return
	}

	// Set up output directory
	now := time.Now()
	targetDirectory := outputDirectory
	if targetDirectory == "" {
		targetDirectory = fmt.Sprintf("ported_%d%02d%02d_%02d%02d%02d", 
			now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
	}

	// Extract ported partitions
	fmt.Printf("\n🔄 Starting ROM porting extraction to: %s\n", targetDirectory)
	if err := dpp.ExtractPortedPartitions(targetDirectory); err != nil {
		log.Fatalf("Failed to extract ported partitions: %v", err)
	}

	// Generate and save report
	if generateReport {
		report := GeneratePortingReport(dpp, payload1Path, payload2Path, targetDirectory)
		
		reportPath := filepath.Join(targetDirectory, "porting_report.json")
		if err := report.SaveToFile(reportPath); err != nil {
			fmt.Printf("Warning: Failed to save porting report: %v\n", err)
		} else {
			fmt.Printf("📊 Porting report saved to: %s\n", reportPath)
		}

		// Print report summary
		report.PrintSummary()
		
		if detailedReport {
			report.PrintDetailedReport()
		}
	}
}

// processPayloadFile handles zip extraction if needed
func processPayloadFile(filename, label string) string {
	if strings.HasSuffix(filename, ".zip") {
		fmt.Printf("Extracting payload.bin from %s archive (%s)...\n", label, filename)
		payloadBin := extractPayloadBin(filename)
		if payloadBin == "" {
			log.Fatalf("Failed to extract payload.bin from %s archive", label)
		}
		return payloadBin
	}
	return filename
}

// parsePortingStrategy converts string to PortingStrategy enum
func parsePortingStrategy(strategyStr string) PortingStrategy {
	switch strings.ToLower(strategyStr) {
	case "priority":
		return StrategyPriority
	case "size":
		return StrategySize
	case "selective":
		return StrategySelective
	case "hybrid":
		return StrategyHybrid
	default:
		fmt.Printf("Warning: Unknown strategy '%s', using 'priority'\n", strategyStr)
		return StrategyPriority
	}
}

// parsePartitionMap parses partition mapping string
func parsePartitionMap(mapStr string) map[string]string {
	partitionMap := make(map[string]string)
	if mapStr == "" {
		return partitionMap
	}

	pairs := strings.Split(mapStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) == 2 {
			partition := strings.TrimSpace(parts[0])
			source := strings.TrimSpace(parts[1])
			if source == "payload1" || source == "payload2" {
				partitionMap[partition] = source
			} else {
				fmt.Printf("Warning: Invalid source '%s' for partition '%s', must be 'payload1' or 'payload2'\n", source, partition)
			}
		} else {
			fmt.Printf("Warning: Invalid partition mapping format: '%s'\n", pair)
		}
	}

	return partitionMap
}
