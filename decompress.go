package tinylz

import (
	"errors"
)

func DecompressedLength(buf []byte) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}

var errCorrupt = errors.New("tinylz: corrupt stream")

func Decompress(src, dst []byte) ([]byte, error) {
	n := len(src)

	if n == 0 {
		return nil, nil
	}

	if n < 4 {
		return nil, errCorrupt
	}

	length := DecompressedLength(src)
	fileoffs := 4

	var ctrlbits uint8

	for fileoffs < n {
		ctrlbits = src[fileoffs]
		fileoffs++

		for ctrln := 0; ctrln < 8; ctrln++ {
			if ctrlbits&0x80 == 0x80 {
				if fileoffs+2 > len(src) {
					return nil, errCorrupt
				}
				l := (int(src[fileoffs]&0xf0) >> 4) | 0
				o := (int(src[fileoffs]&0x0f) << 8) | int(src[fileoffs+1])
				if l == 0x0f {
					if fileoffs+3 > len(src) {
						return nil, errCorrupt
					}
					l += int(src[fileoffs+2])
					fileoffs++
				}
				minoffs := max(len(dst)-4096, 0)
				if minoffs+o >= len(dst) || minoffs+o+l > len(dst) {
					return nil, errCorrupt
				}
				dst = append(dst, dst[minoffs+o:minoffs+o+l]...)
				fileoffs += 2
			} else {
				if fileoffs >= len(src) {
					return nil, errCorrupt
				}
				dst = append(dst, src[fileoffs])
				fileoffs++
			}
			ctrlbits <<= 1
			if uint32(len(dst)) >= length {
				break
			}
		}
	}

	return dst, nil
}
