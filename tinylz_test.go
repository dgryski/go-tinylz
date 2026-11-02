package tinylz

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/dgryski/go-ddmin"
	"github.com/dgryski/go-tinyfuzz"
)

type roundtripErr string

func (r roundtripErr) Error() string {
	return "roundtrip " + string(r) + " failed"
}

func roundtrip(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	check := func(name string, c []byte) error {
		dn, err := DecodedLen(c)
		if err != nil {
			return err
		}

		d := make([]byte, dn)
		n, err := Decompress(d, c)
		if err != nil {
			return err
		}

		if !bytes.Equal(d[:n], b) {
			return roundtripErr(name)
		}

		return nil
	}

	c := make([]byte, CompressBound(len(b)))

	n, err := CompressBest(c, b)
	if err != nil {
		return err
	}
	if err := check("best", c[:n]); err != nil {
		return err
	}

	n, err = Compress(c, b, make([]uint32, 1024))
	if err != nil {
		return err
	}
	if err := check("fast", c[:n]); err != nil {
		return err
	}

	return nil
}

func TestRoundtrip(t *testing.T) {
	err := tinyfuzz.Fuzz(func(b []byte) bool {
		return roundtrip(b) == nil
	}, nil)
	if err != nil {
		t.Errorf("Error testing roundtrip: %v", err)
	}
}

func FuzzRoundtrip(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		if err := roundtrip(b); err != nil {
			t.Error("fuzz: roundtrip:", err)

			t.Logf("minimizing: %x", b)

			fn := func(b []byte) ddmin.Result {
				got := roundtrip(b)
				if got == nil {
					return ddmin.Pass
				}
				if got == err {
					return ddmin.Fail
				}
				return ddmin.Unresolved
			}
			m := ddmin.Minimize(b, fn)
			t.Logf("minimized: %x", m)
		}

		// Just call decompress to make sure it doesn't panic().
		if n, err := DecodedLen(b); err == nil && n <= 1<<20 {
			Decompress(make([]byte, n), b)
		}
	})
}

func randomInput(rng *rand.Rand, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		if rng.Intn(3) == 0 {
			b[i] = byte(rng.Intn(256))
		} else {
			b[i] = "abcab"[rng.Intn(5)]
		}
	}
	return b
}

func TestRoundtripSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	table := make([]uint32, 256)
	for i := 0; i < 500; i++ {
		b := randomInput(rng, rng.Intn(20000)+1)

		c := make([]byte, CompressBound(len(b)))
		n, err := Compress(c, b, table)
		if err != nil {
			t.Fatalf("case %d: compress: %v", i, err)
		}

		d := make([]byte, len(b))
		n, err = Decompress(d, c[:n])
		if err != nil || n != len(b) || !bytes.Equal(d, b) {
			t.Fatalf("case %d: mismatch, err=%v", i, err)
		}
	}
}

func TestCompressBound(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 200; i++ {
		b := make([]byte, rng.Intn(5000)+1)
		rng.Read(b)

		c := make([]byte, CompressBound(len(b)))
		n, err := Compress(c, b, nil)
		if err != nil || n > len(c) {
			t.Fatalf("len %d: n=%d err=%v", len(b), n, err)
		}

		if n > 1 {
			if _, err := Compress(make([]byte, n-1), b, nil); err != ErrShortBuffer {
				t.Fatalf("len %d: short dst: got %v", len(b), err)
			}
		}
	}
}

func TestTableReuse(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	a := randomInput(rng, 3000)
	b := randomInput(rng, 3000)

	table := make([]uint32, 512)
	ca := make([]byte, CompressBound(len(a)))
	Compress(ca, a, table)

	want := make([]byte, CompressBound(len(b)))
	wn, _ := Compress(want, b, make([]uint32, 512))

	got := make([]byte, CompressBound(len(b)))
	gn, _ := Compress(got, b, table)

	if !bytes.Equal(want[:wn], got[:gn]) {
		t.Fatal("reused table changed the output")
	}
}

func TestTableSizes(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	b := randomInput(rng, 30000)
	c := make([]byte, CompressBound(len(b)))
	d := make([]byte, len(b))

	for size := 2; size <= 1<<16; size <<= 1 {
		n, err := Compress(c, b, make([]uint32, size))
		if err != nil {
			t.Fatalf("table %d: compress: %v", size, err)
		}

		m, err := Decompress(d, c[:n])
		if err != nil || m != len(b) || !bytes.Equal(d, b) {
			t.Fatalf("table %d: mismatch, err=%v", size, err)
		}
	}
}

func TestBadTable(t *testing.T) {
	b := []byte("hello hello hello hello")
	c := make([]byte, CompressBound(len(b)))

	for _, size := range []int{1, 3, 6, 100, 1000} {
		if _, err := Compress(c, b, make([]uint32, size)); err != ErrBadTable {
			t.Fatalf("table %d: got %v", size, err)
		}
	}

	if _, err := Compress(c, b, []uint32{}); err != ErrBadTable {
		t.Fatalf("empty table: got %v", err)
	}
}

func TestEmpty(t *testing.T) {
	if n, err := Compress(nil, nil, nil); n != 0 || err != nil {
		t.Fatalf("compress: %d %v", n, err)
	}
	if n, err := DecodedLen(nil); n != 0 || err != nil {
		t.Fatalf("decodedlen: %d %v", n, err)
	}
	if n, err := Decompress(nil, nil); n != 0 || err != nil {
		t.Fatalf("decompress: %d %v", n, err)
	}
}

func TestOverlap(t *testing.T) {
	// literal a then a match of length 9 at offset 0
	src := []byte{10, 0, 0, 0, 0x40, 'a', 0x90, 0x00}
	dst := make([]byte, 10)

	n, err := Decompress(dst, src)
	if err != nil || n != 10 || string(dst) != "aaaaaaaaaa" {
		t.Fatal(n, err, string(dst))
	}
}

func TestDecompressErrors(t *testing.T) {
	if _, err := DecodedLen([]byte{1, 2}); err != ErrCorrupt {
		t.Fatalf("short header: %v", err)
	}

	if _, err := Decompress(make([]byte, 1), []byte{10, 0, 0, 0}); err != ErrShortBuffer {
		t.Fatalf("short dst: %v", err)
	}

	if _, err := Decompress(make([]byte, 10), []byte{10, 0, 0, 0}); err != ErrCorrupt {
		t.Fatalf("truncated: %v", err)
	}

	rng := rand.New(rand.NewSource(4))
	dst := make([]byte, 300)
	for i := 0; i < 20000; i++ {
		b := make([]byte, rng.Intn(64))
		rng.Read(b)
		Decompress(dst, b)
	}
}

func TestErrorString(t *testing.T) {
	var err error = ErrCorrupt
	if err.Error() != "tinylz: corrupt stream" {
		t.Fatal(err)
	}
}

func TestCompressBest(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	for i := 0; i < 50; i++ {
		b := randomInput(rng, rng.Intn(10000)+1)

		fast := make([]byte, CompressBound(len(b)))
		fn, _ := Compress(fast, b, make([]uint32, 1<<16))

		best := make([]byte, CompressBound(len(b)))
		bn, err := CompressBest(best, b)
		if err != nil {
			t.Fatal(err)
		}

		if bn > fn {
			t.Errorf("case %d: best %d bytes, fast %d bytes", i, bn, fn)
		}

		d := make([]byte, len(b))
		if n, err := Decompress(d, best[:bn]); err != nil || n != len(b) || !bytes.Equal(d, b) {
			t.Fatalf("case %d: mismatch, err=%v", i, err)
		}

		if bn > 1 {
			if _, err := CompressBest(make([]byte, bn-1), b); err != ErrShortBuffer {
				t.Fatalf("case %d: short dst: got %v", i, err)
			}
		}
	}

	if n, err := CompressBest(nil, nil); n != 0 || err != nil {
		t.Fatalf("empty: %d %v", n, err)
	}
}
