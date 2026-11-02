package tinylz

// Error is the type of the errors returned by this package.
type Error int

const (
	ErrCorrupt Error = iota + 1
	ErrShortBuffer
	ErrTooLarge
	ErrBadTable
)

func (e Error) Error() string {
	switch e {
	case ErrCorrupt:
		return "tinylz: corrupt stream"
	case ErrShortBuffer:
		return "tinylz: short buffer"
	case ErrTooLarge:
		return "tinylz: input too large"
	case ErrBadTable:
		return "tinylz: table length must be a power of two"
	}
	return "tinylz: unknown error"
}

// DecodedLen returns the number of bytes src decompresses to.
func DecodedLen(src []byte) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}

	if len(src) < 4 {
		return 0, ErrCorrupt
	}

	n := int(uint32(src[0]) | uint32(src[1])<<8 | uint32(src[2])<<16 | uint32(src[3])<<24)
	if n < 0 {
		return 0, ErrTooLarge
	}

	return n, nil
}

// Decompress writes the decompressed src to dst and returns the bytes written.
// dst must hold DecodedLen(src) bytes and must not overlap src.
func Decompress(dst, src []byte) (int, error) {
	n, err := DecodedLen(src)
	if err != nil {
		return 0, err
	}

	if n > len(dst) {
		return 0, ErrShortBuffer
	}

	dst = dst[:n]

	s, d := 4, 0

	for d < n {
		if s >= len(src) {
			return 0, ErrCorrupt
		}

		ctrlbits := src[s]
		s++

		for ctrln := 0; ctrln < 8 && d < n; ctrln++ {
			if ctrlbits&0x80 == 0x80 {
				if s+2 > len(src) {
					return 0, ErrCorrupt
				}

				l := int(src[s] >> 4)
				o := int(src[s]&0x0f)<<8 | int(src[s+1])
				s += 2

				if l == 0x0f {
					if s >= len(src) {
						return 0, ErrCorrupt
					}
					l += int(src[s])
					s++
				}

				p := o
				if d > window {
					p += d - window
				}

				if p >= d || l > n-d {
					return 0, ErrCorrupt
				}

				if p+l <= d {
					copy(dst[d:d+l], dst[p:p+l])
				} else {
					// overlapping match, copy one byte at a time
					for j := 0; j < l; j++ {
						dst[d+j] = dst[p+j]
					}
				}
				d += l
			} else {
				if s >= len(src) {
					return 0, ErrCorrupt
				}

				dst[d] = src[s]
				s++
				d++
			}

			ctrlbits <<= 1
		}
	}

	return n, nil
}
