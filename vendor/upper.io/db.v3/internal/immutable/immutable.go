package immutable

// Immutable represents an immutable chain that, if passed to FastForward,
// applies Fn() to every element of a chain, the first element of this chain is
// represented by Base().
type Immutable interface {
	// Prev is the previous element on a chain.
	Prev() Immutable
	// Fn a function that is able to modify the passed element.
	Fn(interface{}) error
	// Base is the first element on a chain, there's no previous element before
	// the Base element.
	Base() interface{}
}

// FastForward applies all Fn methods in order on the given new Base.
func FastForward(curr Immutable) (interface{}, error) {
	prev := curr.Prev()
	if prev == nil {
		return curr.Base(), nil
	}
	in, err := FastForward(prev)
	if err != nil {
		return nil, err
	}
	err = curr.Fn(in)
	return in, err
}
