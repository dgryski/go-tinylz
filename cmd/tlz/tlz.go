package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/dgryski/go-tinylz"
)

const ext = ".tlz"

func main() {
	var optDecompress = flag.Bool("d", false, "decompress")
	var optCpuProfile = flag.String("cpuprofile", "", "profile")
	var optTable = flag.Int("table", 1024, "compression hash table entries, a power of two")
	var optBest = flag.Bool("best", false, "compress better but slower, ignores -table")
	var optForce = flag.Bool("f", false, "overwrite an existing output file")
	flag.Parse()

	if flag.NArg() > 1 {
		log.Fatal("at most one file name is accepted")
	}

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

	in := os.Stdin
	outName := ""
	if name := flag.Arg(0); name != "" {
		var err error
		in, err = os.Open(name)
		if err != nil {
			log.Fatal(err)
		}
		defer in.Close()

		if *optDecompress {
			if !strings.HasSuffix(name, ext) || len(name) == len(ext) {
				log.Fatalf("%s: unknown suffix, expected %s", name, ext)
			}
			outName = strings.TrimSuffix(name, ext)
		} else {
			outName = name + ext
		}
	}

	buf, err := ioutil.ReadAll(in)
	if err != nil {
		log.Fatal("error during read: ", err)
	}

	var result []byte

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
		result = dst[:n]
	} else {
		dst := make([]byte, tinylz.CompressBound(len(buf)))
		var n int
		var cerr error
		if *optBest {
			n, cerr = tinylz.CompressBest(dst, buf)
		} else {
			n, cerr = tinylz.Compress(dst, buf, make([]uint32, *optTable))
		}
		if cerr != nil {
			log.Fatal(cerr)
		}
		result = dst[:n]
	}

	if outName == "" {
		os.Stdout.Write(result)
		return
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if *optForce {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	out, err := os.OpenFile(outName, flags, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := out.Write(result); err != nil {
		out.Close()
		os.Remove(outName)
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		os.Remove(outName)
		log.Fatal(err)
	}
}
