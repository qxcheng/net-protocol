package seqnum

// Value represents the value of a sequence number.
type Value uint32

// Size represents the size (length) of a sequence number window.
type Size uint32

// LessThan checks if v is before w, i.e., v < w.
func (v Value) LessThan(w Value) bool {
	return int32(v-w) < 0
}

// LessThanEq returns true if v==w or v is before i.e., v < w.
func (v Value) LessThanEq(w Value) bool {
	if v == w {
		return true
	}
	return v.LessThan(w)
}

// InRange checks if v is in the range [a,b), i.e., a <= v < b.
func (v Value) InRange(a, b Value) bool {
	return v-a < b-a
}

// InWindow checks if v is in the window that starts at 'first' and spans 'size'
// sequence numbers.
func (v Value) InWindow(first Value, size Size) bool {
	return v.InRange(first, first.Add(size))
}

// Overlap checks if the window [a,a+b) overlaps with the window [x, x+y).
func Overlap(a Value, b Size, x Value, y Size) bool {
	return a.LessThan(x.Add(y)) && x.LessThan(a.Add(b))
}

// Add calculates the sequence number following the [v, v+s) window.
func (v Value) Add(s Size) Value {
	return v + Value(s)
}

// Size calculates the size of the window defined by [v, w).
func (v Value) Size(w Value) Size {
	return Size(w - v)
}

// UpdateForward updates v such that it becomes v + s.
func (v *Value) UpdateForward(s Size) {
	*v += Value(s)
}