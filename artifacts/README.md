# Payload Dumper Go - Dual Payload ROM Porting Edition

## 🚀 Enhanced Android OTA Payload Dumper with ROM Porting Support

This enhanced version of payload-dumper-go adds powerful dual payload ROM porting capabilities while maintaining all original functionality.

### 📦 What's Included

- `payload-dumper-go` - Main executable (Linux x64)
- `payload-dumper-go-linux-amd64` - Linux x64 binary
- This README with usage instructions

### ✨ New Features

**🔄 Dual Payload ROM Porting**
- Process two payload.bin files simultaneously
- Intelligent partition comparison and selection
- Multiple porting strategies for different use cases
- Generate fully ported IMG files ready for flashing

**📋 Porting Strategies**
- **Priority**: Payload1 takes precedence (default)
- **Size**: Use larger partition when both exist
- **Selective**: Custom partition mapping via CLI
- **Hybrid**: Intelligent selection based on partition type

**📊 Comprehensive Reporting**
- Detailed JSON porting reports
- Console progress tracking with real-time updates
- Partition size comparison and conflict warnings
- Success/failure status for each partition

### 🛠️ Usage

#### Single Payload Mode (Original)
```bash
# Extract all partitions from a single payload
./payload-dumper-go payload.bin

# Extract specific partitions
./payload-dumper-go -p system,vendor,boot payload.bin

# Custom output directory
./payload-dumper-go -o ./extracted payload.bin
```

#### ROM Porting Mode (NEW)
```bash
# Basic ROM porting (Payload1 takes precedence)
./payload-dumper-go -port rom1.bin rom2.bin

# Use size-based strategy (larger partitions preferred)
./payload-dumper-go -port -strategy size rom1.bin rom2.bin

# Selective porting with custom partition mapping
./payload-dumper-go -port -strategy selective -partition-map "system:payload2,vendor:payload1,boot:payload2" rom1.bin rom2.bin

# Hybrid strategy with intelligent selection
./payload-dumper-go -port -strategy hybrid rom1.bin rom2.bin

# List partitions without extracting
./payload-dumper-go -port -list rom1.bin rom2.bin

# Custom output directory with detailed report
./payload-dumper-go -port -output ./ported_rom -detailed-report rom1.bin rom2.bin
```

### 📁 Output Structure

ROM porting mode generates:
```
ported_20241227_143022/
├── system_ported.img      # Ported system partition
├── vendor_ported.img      # Ported vendor partition
├── boot_ported.img        # Ported boot partition
├── recovery_ported.img    # Ported recovery partition
├── ...                    # All other partitions
└── porting_report.json    # Detailed porting analysis
```

### 🎯 Use Cases

- **Custom ROM Development**: Merge features from different ROMs
- **Partition Porting**: Port specific partitions between devices
- **ROM Analysis**: Compare partition differences between versions
- **Hybrid ROM Creation**: Combine best features from multiple sources

### 🔧 Command Line Options

```
Usage:
  Single payload mode: ./payload-dumper-go [options] [inputfile]
  ROM porting mode:    ./payload-dumper-go -port [options] [payload1] [payload2]

Options:
  -c int
        Number of multiple workers to extract (shorthand) (default 4)
  -concurrency int
        Number of multiple workers to extract (default 4)
  -detailed-report
        Print detailed porting report
  -l    Show list of partitions in payload.bin (shorthand)
  -list
        Show list of partitions in payload.bin
  -o string
        Set output directory (shorthand)
  -output string
        Set output directory
  -p string
        Dump only selected partitions (comma-separated) (shorthand)
  -partition-map string
        Partition mapping for selective strategy (format: partition1:payload1,partition2:payload2)
  -partitions string
        Dump only selected partitions (comma-separated)
  -port
        Enable ROM porting mode (requires 2 payload files)
  -report
        Generate porting report (default true)
  -strategy string
        Porting strategy: priority, size, selective, hybrid (default "priority")
```

### 📋 Requirements

- Linux x64 system
- `xz` utility installed (for XZ decompression)
- Sufficient disk space for extracted partitions

### 🚨 Important Notes

- **Backup First**: Always backup your original payload files before porting
- **Test Thoroughly**: Test ported images in a safe environment before flashing
- **Compatibility**: Ensure payload files are compatible with your target device
- **Permissions**: The tool may require appropriate permissions to create output files

### 🔍 Porting Report

The tool generates detailed JSON reports containing:
- Partition source mapping (which payload each partition came from)
- Size comparisons and differences
- Warnings for significant size changes
- Success/failure status for each partition
- Timestamp and strategy information

### 🎉 Examples

**Port system partition from ROM2, keep everything else from ROM1:**
```bash
./payload-dumper-go -port -strategy selective -partition-map "system:payload2" stock_rom.bin custom_rom.bin
```

**Use larger partitions when available:**
```bash
./payload-dumper-go -port -strategy size old_version.bin new_version.bin
```

**Intelligent hybrid porting for critical partitions:**
```bash
./payload-dumper-go -port -strategy hybrid base_rom.bin donor_rom.bin
```

### 📞 Support

This enhanced version maintains full backward compatibility with the original payload-dumper-go while adding powerful ROM porting capabilities.

For issues or questions about the ROM porting features, please refer to the project repository.

---

**Built with ❤️ for the Android development community**
