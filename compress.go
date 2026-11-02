package tinylz

const (
	defaultTableSize = 1 << 10
	fnv1a            = 0x1000193

	window         = 4096
	maxMatchLength = 255 + 15
)

// CompressBound returns the largest number of bytes Compress can write for an
// input of n bytes.
func CompressBound(n int) int {
	return 4 + n + (n+7)/8
}

func loadUint32(buf []uint8) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}

func findMatch(table []uint32, fileoffs int, buf []uint8, s []uint8) (length int, offs int) {
	if len(s) < 4 {
		return 0, 0
	}

	// the mask clears the low bit so h starts a pair of entries
	mask := uint32(len(table) - 2)

	h := ((loadUint32(s) * fnv1a) << 1) & mask

	m1 := table[h]
	m2 := table[h+1]

	table[h] = table[h+1]
	table[h+1] = uint32(fileoffs)

	l1, o1 := checkMatch(fileoffs, int(m1), buf, s)
	l2, o2 := checkMatch(fileoffs, int(m2), buf, s)

	if l1 > l2 {
		return l1, o1
	}

	return l2, o2
}

func checkMatch(fileoffs, offs int, buf, s []byte) (int, int) {
	if offs == 0 || (fileoffs-offs) > window {
		return 0, 0
	}

	/*
	         bufoffs
	   buf = [ . . . . . . o . . . . . . . . . . . . . . . ] fileoffs
	*/

	bufoffs := fileoffs - len(buf)
	o := offs - bufoffs

	match := buf[o:]

	var length int
	for length < maxMatchLength && length < len(s) && length < len(match) && match[length] == s[length] {
		length++
	}

	return length, o
}

func findBest(buf, s []uint8) (length int, offs int) {
	if len(s) == 0 {
		return 0, 0
	}

	s0 := s[0]

	for i, bc := range buf {
		if bc != s0 {
			continue
		}

		j := 1
		for j < maxMatchLength && j < len(s) && i+j < len(buf) && buf[i+j] == s[j] {
			j++
		}

		if j > length {
			length = j
			offs = i
		}
	}

	return length, offs
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type encoder struct {
	dst     []byte
	n       int
	ctrlpos int
	ctrl    uint8
	ctrln   int
	full    bool
}

func (e *encoder) put(b byte) {
	if e.n >= len(e.dst) {
		e.full = true
		return
	}
	e.dst[e.n] = b
	e.n++
}

func (e *encoder) flag(match bool) {
	if e.ctrln == 0 {
		e.ctrlpos = e.n
		e.put(0)
	}
	e.ctrl <<= 1
	if match {
		e.ctrl |= 1
	}
	e.ctrln++
}

func (e *encoder) flush() {
	if e.ctrln == 0 {
		return
	}
	e.ctrl <<= uint(8 - e.ctrln)
	if !e.full {
		e.dst[e.ctrlpos] = e.ctrl
	}
	e.ctrl = 0
	e.ctrln = 0
}

func (e *encoder) literal(b byte) {
	e.flag(false)
	e.put(b)
	if e.ctrln == 8 {
		e.flush()
	}
}

func (e *encoder) match(l, o int) {
	e.flag(true)

	lbits := l
	if l >= 15 {
		lbits = 0x0f
	}
	e.put(uint8((lbits & 0x0f << 4) | ((o & 0xf00) >> 8)))
	e.put(uint8(o & 0x00ff))
	if lbits == 0x0f {
		e.put(uint8(l - 0x0f))
	}

	if e.ctrln == 8 {
		e.flush()
	}
}

// Compress writes the compressed src to dst and returns the bytes written.
// table must be nil or have a power of two length of at least 2.
func Compress(dst, src []byte, table []uint32) (int, error) {
	if table == nil {
		table = make([]uint32, defaultTableSize)
	}

	if len(table) < 2 || len(table)&(len(table)-1) != 0 || uint64(len(table)) > 1<<31 {
		return 0, ErrBadTable
	}

	for i := range table {
		table[i] = 0
	}

	return compress(dst, src, func(fileoffs int, buf, s []byte) (int, int) {
		return findMatch(table, fileoffs, buf, s)
	})
}

// CompressBest searches the whole window for the longest match at every position.
// It takes no table and is much slower than Compress.
func CompressBest(dst, src []byte) (int, error) {
	return compress(dst, src, func(_ int, buf, s []byte) (int, int) {
		return findBest(buf, s)
	})
}

func compress(dst, src []byte, find func(fileoffs int, buf, s []byte) (int, int)) (int, error) {
	n := len(src)
	if n == 0 {
		return 0, nil
	}

	if uint64(n) > 1<<32-1 {
		return 0, ErrTooLarge
	}

	e := encoder{dst: dst}

	e.put(uint8(n & 0xff))
	e.put(uint8(n >> 8 & 0xff))
	e.put(uint8(n >> 16 & 0xff))
	e.put(uint8(n >> 24 & 0xff))

	fileoffs := 0

	for fileoffs < n && !e.full {
		minoffs := max(fileoffs-window, 0)
		l, o := find(fileoffs, src[minoffs:fileoffs], src[fileoffs:])

		if l > 2 {
			minoffs1 := max(1+fileoffs-window, 0)
			l1, o1 := find(fileoffs+1, src[minoffs1:fileoffs+1], src[fileoffs+1:])

			if l1 > l {
				// found a better match if we pretend this char didn't match
				e.literal(src[fileoffs])
				fileoffs++
				l = l1
				o = o1
			}

			e.match(l, o)
		} else {
			l = 1
			e.literal(src[fileoffs])
		}

		fileoffs += l
	}

	e.flush()

	if e.full {
		return 0, ErrShortBuffer
	}

	return e.n, nil
}
