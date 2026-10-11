package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/l-donovan/flacdoc/batch"
	"github.com/l-donovan/flacdoc/flac/frame"
	"github.com/l-donovan/flacdoc/flac/metadata"

	"github.com/go-mmap/mmap"
)

var (
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	verbose     = flag.Bool("verbose", false, "verbose output")
	input       = flag.String("input", "", "input directory")
	workerCount = flag.Int("workers", 8, "worker count")
)

func main() {
	flag.Parse()

	if input != nil && strings.TrimSpace(*input) == "" {
		panic("you gotta set -input, bud")
	}

	if workerCount != nil && *workerCount < 1 {
		panic("come on, you need at least one -worker")
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)

		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	executor := batch.NewExecutor[string, error](*workerCount)

	executor.StartWorkers(func(path string) error {
		fp, err := mmap.Open(path)

		if err != nil {
			return err
		}

		defer func() { _ = fp.Close() }()

		fileMetadata, err := metadata.Parse(fp)

		if err != nil {
			return err
		}

		if len(fileMetadata) == 0 {
			return fmt.Errorf("file %s contained no metadata entries", path)
		}

		streaminfo, ok := fileMetadata[0].(metadata.StreamInfo)

		if !ok {
			return fmt.Errorf("first metadata entry for file %s was not streaminfo", path)
		}

		frameScanner := frame.NewScanner(fp, streaminfo)

		if err := frameScanner.Init(); err != nil {
			return err
		}

		for {
			if err := frameScanner.Scan(); err != nil {
				if streaminfo.SampleCount != frameScanner.SampleCount {
					return fmt.Errorf("Expected %d samples but found %d: %w", streaminfo.SampleCount, frameScanner.SampleCount, err)
				}

				fmt.Printf("Found all %d samples for %s\n", streaminfo.SampleCount, filepath.Base(path))

				return nil
			}
		}
	})

	filepath.WalkDir(*input, func(path string, d fs.DirEntry, err error) error {
		if d.Type().IsRegular() && filepath.Ext(path) == ".flac" {
			executor.AddJobs(path)
		}

		return nil
	})

	if err := errors.Join(executor.Wait()...); err != nil {
		panic(err)
	}
}
