package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"

	"github.com/dgryski/go-tinylz"
)

func main() {
	var optDecompress = flag.Bool("d", false, "decompress")
	var optCpuProfile = flag.String("cpuprofile", "", "profile")
	var optFast = flag.Bool("fast", false, "compress faster")
	var optBest = flag.Bool("best", false, "compress better")
	flag.Parse()

	if *optFast && *optBest {
		log.Fatal("only one of -fast / -best allowed")
	}

	// default
	var matcher tinylz.Matcher = &tinylz.CompressBest{}

	switch {
	case *optFast:
		matcher = &tinylz.CompressFast{}
	case *optBest:
		matcher = &tinylz.CompressBest{}
	}

	if *optCpuProfile != "" {
		f, err := os.Create(*optCpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	buf, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("error during read: ", err)
	}

	if *optDecompress {
		dst := make([]byte, tinylz.DecompressedLength(buf))
		tinylz.Decompress(buf, dst[:0])
		os.Stdout.Write(dst)
	} else {
		stdout := bufio.NewWriter(os.Stdout)
		tinylz.Compress(buf, stdout, matcher)
		stdout.Flush()
	}
}
