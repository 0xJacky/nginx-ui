#!/bin/bash

# Run Large Scale Benchmark for Nginx Log Indexing
# This script tests indexing and search performance with 100M documents

echo "=== Nginx Log Large Scale Benchmark ==="
echo "Testing indexing and search performance with up to 100M documents"
echo ""

# Set environment variables for optimization
export GOGC=200  # Reduce GC frequency
export GOMAXPROCS=$(nproc)  # Use all CPU cores

# Create temp directory for test data
TEST_DIR="/tmp/nginx_log_benchmark"
mkdir -p $TEST_DIR

echo "Configuration:"
echo "  GOGC: $GOGC"
echo "  GOMAXPROCS: $GOMAXPROCS"
echo "  Test Directory: $TEST_DIR"
echo ""

# Run benchmarks with different configurations
echo "Running benchmarks..."

# 1. Quick test with 1M documents
echo "Test 1: 1M documents (quick validation)"
go test -bench=BenchmarkLargeScaleIndexing/Documents_1000000 \
    -benchtime=1x \
    -benchmem \
    -timeout=30m \
    -run=^$ \
    ./... 2>&1 | tee benchmark_1m.log

# 2. Medium test with 10M documents
echo ""
echo "Test 2: 10M documents (medium scale)"
go test -bench=BenchmarkLargeScaleIndexing/Documents_10000000 \
    -benchtime=1x \
    -benchmem \
    -timeout=1h \
    -cpuprofile=cpu_10m.prof \
    -memprofile=mem_10m.prof \
    -run=^$ \
    ./... 2>&1 | tee benchmark_10m.log

# 3. Large test with 100M documents (only if requested)
if [ "$1" == "--full" ]; then
    echo ""
    echo "Test 3: 100M documents (large scale - this will take several hours)"
    go test -bench=BenchmarkLargeScaleIndexing/Documents_100000000 \
        -benchtime=1x \
        -benchmem \
        -timeout=5h \
        -cpuprofile=cpu_100m.prof \
        -memprofile=mem_100m.prof \
        -blockprofile=block_100m.prof \
        -run=^$ \
        ./... 2>&1 | tee benchmark_100m.log
fi

# Generate performance report
echo ""
echo "=== Performance Summary ==="

if [ -f benchmark_1m.log ]; then
    echo "1M Documents Results:"
    grep -E "(docs/sec|ms/doc|μs/op|qps)" benchmark_1m.log | head -10
fi

if [ -f benchmark_10m.log ]; then
    echo ""
    echo "10M Documents Results:"
    grep -E "(docs/sec|ms/doc|μs/op|qps)" benchmark_10m.log | head -10
fi

if [ -f benchmark_100m.log ]; then
    echo ""
    echo "100M Documents Results:"
    grep -E "(docs/sec|ms/doc|μs/op|qps)" benchmark_100m.log | head -10
fi

# Analyze profiles if they exist
if [ -f cpu_10m.prof ]; then
    echo ""
    echo "=== CPU Profile Analysis (10M) ==="
    go tool pprof -top -cum cpu_10m.prof | head -20
fi

if [ -f mem_10m.prof ]; then
    echo ""
    echo "=== Memory Profile Analysis (10M) ==="
    go tool pprof -top -inuse_space mem_10m.prof | head -20
fi

# Cleanup
echo ""
echo "Cleaning up test data..."
rm -rf $TEST_DIR

echo ""
echo "Benchmark complete!"
echo "Results saved to benchmark_*.log files"
echo "Profiles saved to *.prof files (use 'go tool pprof' to analyze)"

# Print optimization recommendations
echo ""
echo "=== Optimization Recommendations ==="
echo "Based on the benchmark results, consider:"
echo "1. Adjusting batch size based on docs/sec metric"
echo "2. Tuning number of workers based on CPU utilization"
echo "3. Increasing cache size if cache miss rate is high"
echo "4. Using SSD storage for index files"
echo "5. Enabling mmap for better I/O performance"
echo "6. Adjusting GOGC for memory vs CPU trade-off"