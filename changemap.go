package godemirror

// mapSet and composeSets are @codemirror/state 6.7.6's functions of the same
// names, which always build a ChangeSet: a section's text is carried along
// even where CodeMirror maps a ChangeDesc, which has none.

// mapSet is a rebased to apply after b, both starting from one document.
// before puts a's insertions first where both insert at one place.
func mapSet(a, b ChangeSet, before bool) ChangeSet {
	var sections []int
	var insert []string
	ia, ib := newSectionIter(a), newSectionIter(b)
	// inserted is the index of the section of a whose text is already in,
	// for a change of a that is taken in piece by piece.
	for inserted := -1; ; {
		switch {
		case ia.done() && ib.n > 0 || ib.done() && ia.n > 0:
			panic(errChangeLength)
		case ia.ins == -1 && ib.ins == -1:
			n := min(ia.n, ib.n)
			sections = addSection(sections, n, -1, false)
			ia.forward(n)
			ib.forward(n)
		case ib.ins >= 0 && (ia.ins < 0 || inserted == ia.i || ia.off == 0 && (ib.n < ia.n || ib.n == ia.n && !before)):
			// A change of b comes before a's next one (by start, then
			// length, then before): skip it, and the changes of a it covers.
			n := ib.n
			sections = addSection(sections, ib.ins, -1, false)
			for n > 0 {
				piece := min(ia.n, n)
				if ia.ins >= 0 && inserted < ia.i && ia.n <= piece {
					sections = addSection(sections, 0, ia.ins, false)
					insert = addInsert(insert, sections, ia.text())
					inserted = ia.i
				}
				ia.forward(piece)
				n -= piece
			}
			ib.next()
		case ia.ins >= 0:
			// The part of a's change up to b's next change that is not a
			// deletion, if they overlap.
			n, left := 0, ia.n
			for left > 0 {
				if ib.ins == -1 {
					piece := min(left, ib.n)
					n += piece
					left -= piece
					ib.forward(piece)
				} else if ib.ins == 0 && ib.n < left {
					left -= ib.n
					ib.next()
				} else {
					break
				}
			}
			ins := 0
			if inserted < ia.i {
				ins = ia.ins
			}
			sections = addSection(sections, n, ins, false)
			if inserted < ia.i {
				insert = addInsert(insert, sections, ia.text())
			}
			inserted = ia.i
			ia.forward(ia.n - left)
		case ia.done() && ib.done():
			return ChangeSet{sections, insert}
		default:
			panic(errChangeLength)
		}
	}
}

// composeSets is a followed by b, which starts from the document a produces.
func composeSets(a, b ChangeSet) ChangeSet {
	var sections []int
	var insert []string
	ia, ib := newSectionIter(a), newSectionIter(b)
	for open := false; ; {
		switch {
		case ia.done() && ib.done():
			return ChangeSet{sections, insert}
		case ia.ins == 0: // a deletion in a
			sections = addSection(sections, ia.n, 0, open)
			ia.next()
		case ib.n == 0 && !ib.done(): // an insertion in b
			sections = addSection(sections, 0, ib.ins, open)
			insert = addInsert(insert, sections, ib.text())
			ib.next()
		case ia.done() || ib.done():
			panic(errChangeLength)
		default:
			n, before := min(ia.len2(), ib.n), len(sections)
			switch {
			case ia.ins == -1:
				insB := ib.ins
				if insB != -1 && ib.off > 0 {
					insB = 0
				}
				sections = addSection(sections, n, insB, open)
				if insB > 0 {
					insert = addInsert(insert, sections, ib.text())
				}
			case ib.ins == -1:
				sections = addSection(sections, ia.untouched(), n, open)
				insert = addInsert(insert, sections, ia.textBit(n))
			default:
				insB := ib.ins
				if ib.off > 0 {
					insB = 0
				}
				sections = addSection(sections, ia.untouched(), insB, open)
				if ib.off == 0 {
					insert = addInsert(insert, sections, ib.text())
				}
			}
			open = (ia.ins > n || ib.ins >= 0 && ib.n > n) && (open || len(sections) > before)
			ia.forward2(n)
			ib.forward(n)
		}
	}
}

// sectionIter walks a set's sections, part of one at a time: n and ins are
// what is left of the current one, off how much of it is behind, and i the
// index just past it. ins is -2 once done.
type sectionIter struct {
	set    ChangeSet
	i      int
	n, off int
	ins    int
}

func newSectionIter(set ChangeSet) *sectionIter {
	it := &sectionIter{set: set}
	it.next()
	return it
}

func (it *sectionIter) next() {
	if s := it.set.sections; it.i < len(s) {
		it.n, it.ins = s[it.i], s[it.i+1]
		it.i += 2
	} else {
		it.n, it.ins = 0, -2
	}
	it.off = 0
}

func (it *sectionIter) done() bool { return it.ins == -2 }

// len2 is what is left of the section on its new side.
func (it *sectionIter) len2() int {
	if it.ins < 0 {
		return it.n
	}
	return it.ins
}

// untouched is the section's old length if none of it is behind, else 0:
// only its first piece counts what it replaced.
func (it *sectionIter) untouched() int {
	if it.off > 0 {
		return 0
	}
	return it.n
}

func (it *sectionIter) text() string { return it.set.insertedAt(it.i - 2) }

func (it *sectionIter) textBit(n int) string {
	t := it.text()
	if t == "" {
		return ""
	}
	return t[it.off : it.off+n]
}

func (it *sectionIter) forward(n int) {
	if n == it.n {
		it.next()
		return
	}
	it.n -= n
	it.off += n
}

func (it *sectionIter) forward2(n int) {
	switch {
	case it.ins == -1:
		it.forward(n)
	case n == it.ins:
		it.next()
	default:
		it.ins -= n
		it.off += n
	}
}
