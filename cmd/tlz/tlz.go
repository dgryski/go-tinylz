package main

import (
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
	var optTable = flag.Int("table", 1024, "compression hash table entries, a power of two")
	flag.Parse()

	if *optTable < 0 {
		log.Fatal("-table must not be negative")
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
		n, derr := tinylz.DecodedLen(buf)
		if derr != nil {
			log.Fatal(derr)
		}
		dst := make([]byte, n)
		n, derr = tinylz.Decompress(dst, buf)
		if derr != nil {
			log.Fatal(derr)
		}
		os.Stdout.Write(dst[:n])
	} else {
		dst := make([]byte, tinylz.CompressBound(len(buf)))
		n, cerr := tinylz.Compress(dst, buf, make([]uint32, *optTable))
		if cerr != nil {
			log.Fatal(cerr)
		}
		os.Stdout.Write(dst[:n])
	}
}
