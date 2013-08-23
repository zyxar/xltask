package ed2k

import (
	"code.google.com/p/go.crypto/md4"
	"hash"
)

const (
	chunk = 9728000
)

type digest struct {
	inner hash.Hash
	total hash.Hash
	round int
	s     int
}

func (d *digest) Reset() {
	d.inner.Reset()
	d.total.Reset()
	d.round = 0
	d.s = 0
}

func New() hash.Hash {
	d := new(digest)
	d.inner = md4.New()
	d.total = md4.New()
	return d
}

func (d *digest) Size() int {
	return 16
}

func (d *digest) BlockSize() int {
	return chunk
}

func (d *digest) next() {
	d.round++
	d.s = 0
	d.inner.Reset()
}

func (d *digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	if nn+d.s < chunk {
		d.inner.Write(p)
		d.s += nn
		return
	}
	l := chunk - d.s
	d.inner.Write(p[:l])
	d.total.Write(d.inner.Sum(nil))
	d.next()
	left := nn - l
	min := chunk
	for left > 0 {
		if left < chunk {
			min = left
		}
		d.inner.Write(p[l : l+min])
		if min == chunk {
			d.total.Write(d.inner.Sum(nil))
			d.next()
		} else {
			d.s = min
		}
		left -= min
	}
	return
}

func (d *digest) Sum(in []byte) []byte {
	dd := *d
	if dd.round == 0 {
		return dd.inner.Sum(in)
	}
	if dd.s > 0 {
		dd.total.Write(dd.inner.Sum(nil))
	}
	return dd.total.Sum(in)
}
