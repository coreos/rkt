package sqlbuilder

// BatchInserter provides a helper that can be used to do massive insertions in
// batches.
type BatchInserter struct {
	inserter *inserter
	size     int
	values   chan []interface{}
	err      error
}

func newBatchInserter(inserter *inserter, size int) *BatchInserter {
	if size < 1 {
		size = 1
	}
	b := &BatchInserter{
		inserter: inserter,
		size:     size,
		values:   make(chan []interface{}, size),
	}
	return b
}

// Values pushes column values to be inserted as part of the batch.
func (b *BatchInserter) Values(values ...interface{}) *BatchInserter {
	b.values <- values
	return b
}

func (b *BatchInserter) nextQuery() *inserter {
	ins := &inserter{}
	*ins = *b.inserter
	i := 0
	for values := range b.values {
		i++
		ins = ins.Values(values...).(*inserter)
		if i == b.size {
			break
		}
	}
	if i == 0 {
		return nil
	}
	return ins
}

// NextResult is useful when using PostgreSQL and Returning(), it dumps the
// next slice of results to dst, which can mean having the IDs of all inserted
// elements in the batch.
func (b *BatchInserter) NextResult(dst interface{}) bool {
	clone := b.nextQuery()
	if clone == nil {
		return false
	}
	b.err = clone.Iterator().All(dst)
	return (b.err == nil)
}

// Done means that no more elements are going to be added.
func (b *BatchInserter) Done() {
	close(b.values)
}

// Wait blocks until the whole batch is executed.
func (b *BatchInserter) Wait() error {
	for {
		q := b.nextQuery()
		if q == nil {
			break
		}
		if _, err := q.Exec(); err != nil {
			b.err = err
			break
		}
	}
	return b.Err()
}

// Err returns any error while executing the batch.
func (b *BatchInserter) Err() error {
	return b.err
}
