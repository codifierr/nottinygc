package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func buildTags() string {
	mode := strings.ToLower(os.Getenv("RE2_TEST_MODE"))
	exhaustive := os.Getenv("RE2_TEST_EXHAUSTIVE") == "1"

	var tags []string
	if mode == "cgo" {
		tags = append(tags, "re2_cgo")
	}
	if exhaustive {
		tags = append(tags, "re2_test_exhaustive")
	}

	return strings.Join(tags, ",")
}

// Test runs unit tests - by default, it uses wazero; set RE2_TEST_MODE=cgo or RE2_TEST_MODE=tinygo to use either, or
// RE2_TEST_EXHAUSTIVE=1 to enable exhaustive tests that may take a long time.
func Test() error {
	mode := strings.ToLower(os.Getenv("RE2_TEST_MODE"))

	if mode != "tinygo" {
		return sh.RunV("go", "test", "-v", "-timeout=20m", "-tags", buildTags(), "./...")
	}

	return sh.RunV("tinygo", "test", "-target=wasi", "-v", "-tags", buildTags(), "./...")
}

func Format() error {
	if err := sh.RunV("go", "run", fmt.Sprintf("mvdan.cc/gofumpt@%s", gofumptVersion), "-l", "-w", "."); err != nil {
		return err
	}
	if err := sh.RunV("go", "run", fmt.Sprintf("github.com/rinchsan/gosimports/cmd/gosimports@%s", gosImportsVer), "-w",
		"-local", "github.com/wasilibs/go-re2",
		"."); err != nil {
		return nil
	}
	return nil
}

func Lint() error {
	return sh.RunV("go", "run", fmt.Sprintf("github.com/golangci/golangci-lint/cmd/golangci-lint@%s", golangCILintVer), "run", "--build-tags", buildTags())
}

// Check runs lint and tests.
func Check() {
	mg.SerialDeps(Lint, Test)
}

// UpdateLibs updates the precompiled wasm libraries.
func UpdateLibs() error {
	libs := []string{"bdwgc", "mimalloc"}
	for _, lib := range libs {
		if err := sh.RunV("docker", "build", "-t", "ghcr.io/wasilibs/nottinygc/buildtools-"+lib, filepath.Join("buildtools", lib)); err != nil {
			return err
		}
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := sh.RunV("docker", "run", "-it", "--rm", "-v", fmt.Sprintf("%s:/out", filepath.Join(wd, "wasm")), "ghcr.io/wasilibs/nottinygc/buildtools-"+lib); err != nil {
			return err
		}
	}
	return nil
}

// Bench runs benchmarks in the default configuration for a Go app, using wazero.
func Bench() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeWazero)...)
}

// BenchCGO runs benchmarks with re2 accessed using cgo. A C++ toolchain and libre2 must be installed to run.
func BenchCGO() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeCGO)...)
}

// BenchSTDLib runs benchmarks using the regexp library in the standard library for comparison.
func BenchSTDLib() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeSTDLib)...)
}

// BenchAll runs all benchmark types and outputs with benchstat. A C++ toolchain and libre2 must be installed to run.
func BenchAll() error {
	if err := os.MkdirAll("build", 0o755); err != nil {
		return err
	}

	fmt.Println("Executing wazero benchmarks")
	wazero, err := sh.Output("go", benchArgs("./...", 5, benchModeWazero)...)
	if err != nil {
		fmt.Printf("Error running wazero benchmarks:\n%s", wazero)
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "bench.txt"), []byte(wazero), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing cgo benchmarks")
	cgo, err := sh.Output("go", benchArgs("./...", 5, benchModeCGO)...)
	if err != nil {
		fmt.Printf("Error running cgo benchmarks:\n%s", cgo)
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "bench_cgo.txt"), []byte(cgo), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing stdlib benchmarks")
	stdlib, err := sh.Output("go", benchArgs("./...", 5, benchModeSTDLib)...)
	if err != nil {
		fmt.Printf("Error running stdlib benchmarks:\n%s", stdlib)
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "bench_stdlib.txt"), []byte(stdlib), 0o644); err != nil {
		return err
	}

	return sh.RunV("go", "run", fmt.Sprintf("golang.org/x/perf/cmd/benchstat@%s", benchstatVer),
		"build/bench_stdlib.txt", "build/bench.txt", "build/bench_cgo.txt")
}

// WAFBench runs benchmarks in the default configuration for a Go app, using wazero.
func WAFBench() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeWazero)...)
}

// WAFBenchCGO runs benchmarks with re2 accessed using cgo. A C++ toolchain and libre2 must be installed to run.
func WAFBenchCGO() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeCGO)...)
}

// WAFBenchSTDLib runs benchmarks using the regexp library in the standard library for comparison.
func WAFBenchSTDLib() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeSTDLib)...)
}

// WAFBenchAll runs all benchmark types and outputs with benchstat. A C++ toolchain and libre2 must be installed to run.
func WAFBenchAll() error {
	if err := os.MkdirAll("build", 0o755); err != nil {
		return err
	}

	fmt.Println("Executing wazero benchmarks")
	wazero, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeWazero)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench.txt"), []byte(wazero), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing cgo benchmarks")
	cgo, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeCGO)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench_cgo.txt"), []byte(cgo), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing stdlib benchmarks")
	stdlib, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeSTDLib)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench_stdlib.txt"), []byte(stdlib), 0o644); err != nil {
		return err
	}

	return sh.RunV("go", "run", fmt.Sprintf("golang.org/x/perf/cmd/benchstat@%s", benchstatVer),
		"build/wafbench_stdlib.txt", "build/wafbench.txt", "build/wafbench_cgo.txt")
}

var Default = Test

type benchMode int

const (
	benchModeWazero benchMode = iota
	benchModeCGO
	benchModeSTDLib
)

func benchArgs(pkg string, count int, mode benchMode) []string {
	args := []string{"test", "-bench=.", "-run=^$", "-v", "-timeout=60m"}
	if count > 0 {
		args = append(args, fmt.Sprintf("-count=%d", count))
	}
	switch mode {
	case benchModeCGO:
		args = append(args, "-tags=re2_cgo")
	case benchModeSTDLib:
		args = append(args, "-tags=re2_bench_stdlib")
	}
	args = append(args, pkg)

	return args
}
