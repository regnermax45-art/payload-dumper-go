package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	humanize "github.com/dustin/go-humanize"
)

// PortingReport contains detailed information about the porting process
type PortingReport struct {
	Timestamp       time.Time         `json:"timestamp"`
	Strategy        string            `json:"strategy"`
	Payload1Path    string            `json:"payload1_path"`
	Payload2Path    string            `json:"payload2_path"`
	OutputDirectory string            `json:"output_directory"`
	TotalPartitions int               `json:"total_partitions"`
	Payload1Count   int               `json:"payload1_count"`
	Payload2Count   int               `json:"payload2_count"`
	TotalSize       uint64            `json:"total_size"`
	Partitions      []PartitionReport `json:"partitions"`
	Warnings        []string          `json:"warnings"`
	Errors          []string          `json:"errors"`
}

// PartitionReport contains information about a single ported partition
type PartitionReport struct {
	Name           string `json:"name"`
	Source         string `json:"source"`
	Size           uint64 `json:"size"`
	SizeHuman      string `json:"size_human"`
	OutputFile     string `json:"output_file"`
	ExistsInBoth   bool   `json:"exists_in_both"`
	SizeDifference int64  `json:"size_difference,omitempty"`
	Status         string `json:"status"`
}

// GeneratePortingReport creates a detailed porting report
func GeneratePortingReport(dpp *DualPayloadProcessor, payload1Path, payload2Path, outputDir string) *PortingReport {
	report := &PortingReport{
		Timestamp:       time.Now(),
		Strategy:        dpp.getStrategyName(),
		Payload1Path:    payload1Path,
		Payload2Path:    payload2Path,
		OutputDirectory: outputDir,
		TotalPartitions: len(dpp.portedPartitions),
		Warnings:        []string{},
		Errors:          []string{},
	}

	var totalSize uint64
	payload1Count := 0
	payload2Count := 0

	// Process each ported partition
	for _, partition := range dpp.portedPartitions {
		partReport := PartitionReport{
			Name:         partition.Name,
			Size:         partition.Size,
			SizeHuman:    humanize.Bytes(partition.Size),
			OutputFile:   fmt.Sprintf("%s_ported.img", partition.Name),
			ExistsInBoth: partition.Payload1 != nil && partition.Payload2 != nil,
			Status:       "success",
		}

		switch partition.Source {
		case SourcePayload1:
			partReport.Source = "Payload1"
			payload1Count++
		case SourcePayload2:
			partReport.Source = "Payload2"
			payload2Count++
		case SourceMerged:
			partReport.Source = "Merged"
		}

		// Calculate size difference if partition exists in both payloads
		if partReport.ExistsInBoth {
			size1 := partition.Payload1.GetNewPartitionInfo().GetSize()
			size2 := partition.Payload2.GetNewPartitionInfo().GetSize()
			partReport.SizeDifference = int64(size2) - int64(size1)

			// Add warnings for significant size differences
			if partReport.SizeDifference > 0 {
				if float64(partReport.SizeDifference)/float64(size1) > 0.1 { // >10% increase
					report.Warnings = append(report.Warnings, 
						fmt.Sprintf("Partition %s: Payload2 is %s larger than Payload1", 
							partition.Name, humanize.Bytes(uint64(partReport.SizeDifference))))
				}
			} else if partReport.SizeDifference < 0 {
				if float64(-partReport.SizeDifference)/float64(size2) > 0.1 { // >10% decrease
					report.Warnings = append(report.Warnings, 
						fmt.Sprintf("Partition %s: Payload1 is %s larger than Payload2", 
							partition.Name, humanize.Bytes(uint64(-partReport.SizeDifference))))
				}
			}
		}

		// Check if output file exists and is accessible
		outputPath := filepath.Join(outputDir, partReport.OutputFile)
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			partReport.Status = "failed"
			report.Errors = append(report.Errors, 
				fmt.Sprintf("Output file not found: %s", partReport.OutputFile))
		}

		totalSize += partition.Size
		report.Partitions = append(report.Partitions, partReport)
	}

	report.TotalSize = totalSize
	report.Payload1Count = payload1Count
	report.Payload2Count = payload2Count

	// Add strategy-specific warnings
	switch dpp.Strategy {
	case StrategySize:
		report.Warnings = append(report.Warnings, 
			"Size-based strategy: Larger partitions were preferred, which may not always be optimal")
	case StrategyHybrid:
		report.Warnings = append(report.Warnings, 
			"Hybrid strategy: Critical partitions were selected based on size, others from Payload1")
	}

	return report
}

// SaveReportToFile saves the porting report to a JSON file
func (pr *PortingReport) SaveToFile(filepath string) error {
	data, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %v", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %v", err)
	}

	return nil
}

// PrintSummary prints a human-readable summary of the porting report
func (pr *PortingReport) PrintSummary() {
	fmt.Println("\n=== PORTING REPORT SUMMARY ===")
	fmt.Printf("Timestamp: %s\n", pr.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Strategy: %s\n", pr.Strategy)
	fmt.Printf("Total partitions ported: %d\n", pr.TotalPartitions)
	fmt.Printf("Total size: %s\n", humanize.Bytes(pr.TotalSize))
	fmt.Printf("From Payload1: %d partitions\n", pr.Payload1Count)
	fmt.Printf("From Payload2: %d partitions\n", pr.Payload2Count)

	if len(pr.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d):\n", len(pr.Warnings))
		for _, warning := range pr.Warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}

	if len(pr.Errors) > 0 {
		fmt.Printf("\n❌ Errors (%d):\n", len(pr.Errors))
		for _, error := range pr.Errors {
			fmt.Printf("  • %s\n", error)
		}
	}

	successCount := 0
	for _, partition := range pr.Partitions {
		if partition.Status == "success" {
			successCount++
		}
	}

	fmt.Printf("\n✅ Successfully ported: %d/%d partitions\n", successCount, pr.TotalPartitions)

	if len(pr.Errors) == 0 {
		fmt.Println("\n🎉 ROM porting completed successfully!")
		fmt.Printf("Ported IMG files are available in: %s\n", pr.OutputDirectory)
	} else {
		fmt.Println("\n⚠️  ROM porting completed with errors. Please check the report for details.")
	}
}

// PrintDetailedReport prints a detailed partition-by-partition report
func (pr *PortingReport) PrintDetailedReport() {
	fmt.Println("\n=== DETAILED PORTING REPORT ===")
	
	for _, partition := range pr.Partitions {
		fmt.Printf("\n📦 %s\n", partition.Name)
		fmt.Printf("   Source: %s\n", partition.Source)
		fmt.Printf("   Size: %s\n", partition.SizeHuman)
		fmt.Printf("   Output: %s\n", partition.OutputFile)
		fmt.Printf("   Status: %s\n", partition.Status)
		
		if partition.ExistsInBoth {
			fmt.Printf("   Exists in both payloads: Yes\n")
			if partition.SizeDifference != 0 {
				if partition.SizeDifference > 0 {
					fmt.Printf("   Size difference: +%s (Payload2 larger)\n", 
						humanize.Bytes(uint64(partition.SizeDifference)))
				} else {
					fmt.Printf("   Size difference: -%s (Payload1 larger)\n", 
						humanize.Bytes(uint64(-partition.SizeDifference)))
				}
			} else {
				fmt.Printf("   Size difference: None (identical sizes)\n")
			}
		} else {
			fmt.Printf("   Exists in both payloads: No\n")
		}
	}
}
