package tinylz

import (
	"io"
)

type Matcher interface {
	findMatch(fileoffs int, buf []uint8, s []uint8) (length int, offs int)
}

const (
	hlog  = 17
	hsize = 1 << hlog
	fnv1a = 0x1000193

	maxMatchLength = 255 + 15
)

type CompressFast []int

func loadUint32(buf []uint8) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}

func (dict *CompressFast) findMatch(fileoffs int, buf []uint8, s []uint8) (length int, offs int) {
	if len(*dict) == 0 {
		*dict = CompressFast(make([]int, hsize))
	}

	if len(s) < 4 {
		return 0, 0
	}

	h := (loadUint32(s) * fnv1a) % (1 << (hlog - 1))

	h *= 2

	m1 := (*dict)[h]
	m2 := (*dict)[h+1]

	(*dict)[h] = (*dict)[h+1]
	(*dict)[h+1] = fileoffs

	l1, o1 := checkMatch(fileoffs, m1, buf, s)
	l2, o2 := checkMatch(fileoffs, m2, buf, s)

	if l1 > l2 {
		return l1, o1
	}

	return l2, o2
}

func checkMatch(fileoffs, offs int, buf, s []byte) (int, int) {
	if offs == 0 || (fileoffs-offs) > 4096 {
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

type CompressBest struct{}

func (*CompressBest) findMatch(fileoffs int, buf []uint8, s []uint8) (length int, offs int) {
	max_length := 0
	max_offs := 0

	s0 := s[0]

	for i, bc := range buf {

		if bc != s0 {
			continue
		}

		j := 1
		for j < maxMatchLength && j < len(s) && i+j < len(buf) && buf[i+j] == s[j] {
			j++
		}

		if j > max_length {
			max_length = j
			max_offs = i
		}
	}

	return max_length, max_offs
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type encoder struct {
	w     io.Writer
	data  []byte
	ctrl  uint8
	ctrln int
}

func (e *encoder) flag(match bool) {
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
	e.w.Write([]byte{e.ctrl})
	e.w.Write(e.data)
	e.data = e.data[0:0]
	e.ctrl = 0
	e.ctrln = 0
}

func (e *encoder) literal(b byte) {
	e.flag(false)
	e.data = append(e.data, b)
	if e.ctrln == 8 {
		e.flush()
	}
}

func (e *encoder) match(l, o int) {
	e.flag(true)

	var lbits int
	if l >= 15 {
		lbits = 0x0f
	} else {
		lbits = l
	}
	e.data = append(e.data, uint8((lbits&0x0f<<4)|((o&0xf00)>>8)))
	e.data = append(e.data, uint8(o&0x00ff))
	if lbits == 0x0f {
		e.data = append(e.data, uint8(l-0x0f))
	}

	if e.ctrln == 8 {
		e.flush()
	}
}

func Compress(buf []byte, w io.Writer, matcher Matcher) {
	if len(buf) == 0 {
		return
	}

	n := len(buf)

	w.Write([]byte{uint8(n & 0xff), uint8(n >> 8 & 0xff), uint8(n >> 16 & 0xff), uint8(n >> 24 & 0xff)})

	e := encoder{w: w, data: make([]byte, 0, 4096)}

	fileoffs := 0

	for fileoffs < n {
		minoffs := max(fileoffs-4096, 0)
		l, o := matcher.findMatch(fileoffs, buf[minoffs:fileoffs], buf[fileoffs:])

		if l > 2 {
			minoffs1 := max(1+fileoffs-4096, 0)
			l1, o1 := matcher.findMatch(fileoffs+1, buf[minoffs1:fileoffs+1], buf[fileoffs+1:])

			if l1 > l {
				// found a better match if we pretend this char didn't match
				e.literal(buf[fileoffs])
				fileoffs++
				l = l1
				o = o1
			}

			e.match(l, o)
		} else {
			l = 1
			e.literal(buf[fileoffs])
		}

		fileoffs += l
	}

	e.flush()
}
