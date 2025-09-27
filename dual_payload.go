package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ssut/payload-dumper-go/chromeos_update_engine"
	"github.com/vbauerster/mpb/v5"
	humanize "github.com/dustin/go-humanize"
)

// PortingStrategy defines how partitions should be ported between payloads
type PortingStrategy int

const (
	StrategyPriority PortingStrategy = iota // Payload1 takes precedence
	StrategySize                            // Use larger partition
	StrategySelective                       // User-defined mapping
	StrategyHybrid                          // Intelligent merging
)

// PartitionSource indicates which payload a partition comes from
type PartitionSource int

const (
	SourcePayload1 PartitionSource = iota
	SourcePayload2
	SourceMerged
)

// PortedPartition represents a partition that has been processed for porting
type PortedPartition struct {
	Name     string
	Source   PartitionSource
	Size     uint64
	Payload1 *chromeos_update_engine.PartitionUpdate
	Payload2 *chromeos_update_engine.PartitionUpdate
	Selected *chromeos_update_engine.PartitionUpdate
}

// DualPayloadProcessor handles processing of two payload files for ROM porting
type DualPayloadProcessor struct {
	Payload1    *Payload
	Payload2    *Payload
	Strategy    PortingStrategy
	PartitionMap map[string]string // For selective strategy: partition -> source
	
	portedPartitions []*PortedPartition
	progress        *mpb.Progress
	concurrency     int
	
	requests chan *portingRequest
	workerWG sync.WaitGroup
}

type portingRequest struct {
	partition       *PortedPartition
	targetDirectory string
}

// NewDualPayloadProcessor creates a new dual payload processor
func NewDualPayloadProcessor(payload1, payload2 *Payload) *DualPayloadProcessor {
	return &DualPayloadProcessor{
		Payload1:     payload1,
		Payload2:     payload2,
		Strategy:     StrategyPriority,
		PartitionMap: make(map[string]string),
		concurrency:  4,
	}
}

// SetStrategy sets the porting strategy
func (dpp *DualPayloadProcessor) SetStrategy(strategy PortingStrategy) {
	dpp.Strategy = strategy
}

// SetPartitionMap sets the partition mapping for selective strategy
func (dpp *DualPayloadProcessor) SetPartitionMap(partitionMap map[string]string) {
	dpp.PartitionMap = partitionMap
}

// SetConcurrency sets the number of worker threads
func (dpp *DualPayloadProcessor) SetConcurrency(concurrency int) {
	dpp.concurrency = concurrency
}

// AnalyzePartitions compares partitions between both payloads
func (dpp *DualPayloadProcessor) AnalyzePartitions() error {
	if !dpp.Payload1.initialized || !dpp.Payload2.initialized {
		return fmt.Errorf("both payloads must be initialized before analysis")
	}

	// Create maps for quick lookup
	payload1Partitions := make(map[string]*chromeos_update_engine.PartitionUpdate)
	payload2Partitions := make(map[string]*chromeos_update_engine.PartitionUpdate)

	for _, partition := range dpp.Payload1.deltaArchiveManifest.Partitions {
		payload1Partitions[partition.GetPartitionName()] = partition
	}

	for _, partition := range dpp.Payload2.deltaArchiveManifest.Partitions {
		payload2Partitions[partition.GetPartitionName()] = partition
	}

	// Get all unique partition names
	allPartitions := make(map[string]bool)
	for name := range payload1Partitions {
		allPartitions[name] = true
	}
	for name := range payload2Partitions {
		allPartitions[name] = true
	}

	// Analyze each partition
	for name := range allPartitions {
		ported := &PortedPartition{
			Name:     name,
			Payload1: payload1Partitions[name],
			Payload2: payload2Partitions[name],
		}

		// Determine which partition to use based on strategy
		switch dpp.Strategy {
		case StrategyPriority:
			if ported.Payload1 != nil {
				ported.Selected = ported.Payload1
				ported.Source = SourcePayload1
			} else {
				ported.Selected = ported.Payload2
				ported.Source = SourcePayload2
			}

		case StrategySize:
			if ported.Payload1 != nil && ported.Payload2 != nil {
				size1 := ported.Payload1.GetNewPartitionInfo().GetSize()
				size2 := ported.Payload2.GetNewPartitionInfo().GetSize()
				if size1 >= size2 {
					ported.Selected = ported.Payload1
					ported.Source = SourcePayload1
				} else {
					ported.Selected = ported.Payload2
					ported.Source = SourcePayload2
				}
			} else if ported.Payload1 != nil {
				ported.Selected = ported.Payload1
				ported.Source = SourcePayload1
			} else {
				ported.Selected = ported.Payload2
				ported.Source = SourcePayload2
			}

		case StrategySelective:
			if sourcePayload, exists := dpp.PartitionMap[name]; exists {
				if sourcePayload == "payload1" && ported.Payload1 != nil {
					ported.Selected = ported.Payload1
					ported.Source = SourcePayload1
				} else if sourcePayload == "payload2" && ported.Payload2 != nil {
					ported.Selected = ported.Payload2
					ported.Source = SourcePayload2
				} else {
					// Fallback to priority strategy
					if ported.Payload1 != nil {
						ported.Selected = ported.Payload1
						ported.Source = SourcePayload1
					} else {
						ported.Selected = ported.Payload2
						ported.Source = SourcePayload2
					}
				}
			} else {
				// Default to priority strategy for unmapped partitions
				if ported.Payload1 != nil {
					ported.Selected = ported.Payload1
					ported.Source = SourcePayload1
				} else {
					ported.Selected = ported.Payload2
					ported.Source = SourcePayload2
				}
			}

		case StrategyHybrid:
			// For hybrid strategy, we'll implement intelligent selection
			// For now, use priority with some intelligence
			if ported.Payload1 != nil && ported.Payload2 != nil {
				// Prefer newer/larger partitions for system-critical partitions
				criticalPartitions := []string{"system", "vendor", "boot", "recovery"}
				isCritical := false
				for _, critical := range criticalPartitions {
					if strings.Contains(strings.ToLower(name), critical) {
						isCritical = true
						break
					}
				}

				if isCritical {
					size1 := ported.Payload1.GetNewPartitionInfo().GetSize()
					size2 := ported.Payload2.GetNewPartitionInfo().GetSize()
					if size2 > size1 {
						ported.Selected = ported.Payload2
						ported.Source = SourcePayload2
					} else {
						ported.Selected = ported.Payload1
						ported.Source = SourcePayload1
					}
				} else {
					ported.Selected = ported.Payload1
					ported.Source = SourcePayload1
				}
			} else if ported.Payload1 != nil {
				ported.Selected = ported.Payload1
				ported.Source = SourcePayload1
			} else {
				ported.Selected = ported.Payload2
				ported.Source = SourcePayload2
			}
		}

		if ported.Selected != nil {
			ported.Size = ported.Selected.GetNewPartitionInfo().GetSize()
			dpp.portedPartitions = append(dpp.portedPartitions, ported)
		}
	}

	// Sort partitions by name for consistent output
	sort.Slice(dpp.portedPartitions, func(i, j int) bool {
		return dpp.portedPartitions[i].Name < dpp.portedPartitions[j].Name
	})

	return nil
}

// PrintPortingPlan displays the porting plan to the user
func (dpp *DualPayloadProcessor) PrintPortingPlan() {
	fmt.Println("\n=== ROM PORTING PLAN ===")
	fmt.Printf("Strategy: %s\n", dpp.getStrategyName())
	fmt.Printf("Total partitions to port: %d\n\n", len(dpp.portedPartitions))

	payload1Count := 0
	payload2Count := 0
	
	fmt.Println("Partitions to be ported:")
	for _, partition := range dpp.portedPartitions {
		sourceStr := ""
		switch partition.Source {
		case SourcePayload1:
			sourceStr = "Payload1"
			payload1Count++
		case SourcePayload2:
			sourceStr = "Payload2"
			payload2Count++
		case SourceMerged:
			sourceStr = "Merged"
		}
		
		fmt.Printf("  %s (%s) <- %s\n", 
			partition.Name, 
			humanize.Bytes(partition.Size), 
			sourceStr)
	}
	
	fmt.Printf("\nSource distribution:\n")
	fmt.Printf("  From Payload1: %d partitions\n", payload1Count)
	fmt.Printf("  From Payload2: %d partitions\n", payload2Count)
	fmt.Println()
}

// getStrategyName returns a human-readable strategy name
func (dpp *DualPayloadProcessor) getStrategyName() string {
	switch dpp.Strategy {
	case StrategyPriority:
		return "Priority (Payload1 preferred)"
	case StrategySize:
		return "Size-based (Larger partition preferred)"
	case StrategySelective:
		return "Selective (User-defined mapping)"
	case StrategyHybrid:
		return "Hybrid (Intelligent selection)"
	default:
		return "Unknown"
	}
}

// ExtractPortedPartitions extracts all ported partitions to the target directory
func (dpp *DualPayloadProcessor) ExtractPortedPartitions(targetDirectory string) error {
	if len(dpp.portedPartitions) == 0 {
		return fmt.Errorf("no partitions analyzed yet, call AnalyzePartitions first")
	}

	// Create target directory if it doesn't exist
	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDirectory, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %v", err)
		}
	}

	dpp.progress = mpb.New()
	dpp.requests = make(chan *portingRequest, 100)
	dpp.spawnPortingWorkers(dpp.concurrency)

	// Queue all partitions for extraction
	for _, partition := range dpp.portedPartitions {
		dpp.workerWG.Add(1)
		dpp.requests <- &portingRequest{
			partition:       partition,
			targetDirectory: targetDirectory,
		}
	}

	// Wait for all extractions to complete
	dpp.workerWG.Wait()
	close(dpp.requests)
	dpp.progress.Wait()

	return nil
}

// spawnPortingWorkers creates worker goroutines for porting operations
func (dpp *DualPayloadProcessor) spawnPortingWorkers(n int) {
	for i := 0; i < n; i++ {
		go dpp.portingWorker()
	}
}

// portingWorker processes porting requests
func (dpp *DualPayloadProcessor) portingWorker() {
	for req := range dpp.requests {
		partition := req.partition
		targetDirectory := req.targetDirectory

		// Determine which payload to extract from
		var sourcePayload *Payload
		var sourcePartition *chromeos_update_engine.PartitionUpdate

		switch partition.Source {
		case SourcePayload1:
			sourcePayload = dpp.Payload1
			sourcePartition = partition.Payload1
		case SourcePayload2:
			sourcePayload = dpp.Payload2
			sourcePartition = partition.Payload2
		default:
			fmt.Printf("Error: Unknown source for partition %s\n", partition.Name)
			dpp.workerWG.Done()
			continue
		}

		// Create output file with ported suffix
		filename := fmt.Sprintf("%s_ported.img", partition.Name)
		filepath := filepath.Join(targetDirectory, filename)
		
		file, err := os.OpenFile(filepath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			fmt.Printf("Error creating file %s: %v\n", filepath, err)
			dpp.workerWG.Done()
			continue
		}

		// Extract the partition
		if err := sourcePayload.Extract(sourcePartition, file); err != nil {
			fmt.Printf("Error extracting partition %s: %v\n", partition.Name, err)
		}

		file.Close()
		dpp.workerWG.Done()
	}
}

// GetPortedPartitions returns the list of ported partitions
func (dpp *DualPayloadProcessor) GetPortedPartitions() []*PortedPartition {
	return dpp.portedPartitions
}
