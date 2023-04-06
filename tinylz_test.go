package tinylz

import (
	"bytes"
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

	check := func(name string, buf *bytes.Buffer, b []byte) error {
		c := buf.Bytes()
		d := make([]byte, DecompressedLength(c))
		d, err := Decompress(c, d[:0])
		if err != nil {
			return err
		}

		if !bytes.Equal(d, b) {
			return roundtripErr(name)
		}

		return nil
	}

	var buf bytes.Buffer
	Compress(b, &buf, &CompressBest{})
	if err := check("best", &buf, b); err != nil {
		return err
	}

	buf.Reset()
	Compress(b, &buf, &CompressFast{})
	if err := check("fast", &buf, b); err != nil {
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
		//Decompress(b, nil)
	})
}
