package pgxprom

import (
	"reflect"
	"time"
)

// ContextKey represents a context key.
type ContextKey struct {
	name string
}

// String returns the context key as a string.
func (k *ContextKey) String() string {
	return k.name
}

// TraceQueryKey represents the context key of the data.
var TraceQueryKey = &ContextKey{
	name: reflect.TypeOf(TraceQueryData{}).PkgPath(),
}

// TraceQueryData represents a query data
type TraceQueryData struct {
	StartedAt time.Time
	SQL       string
	Args      []any
}

// TraceBatchKey represents the context key of the data.
var TraceBatchKey = &ContextKey{
	name: reflect.TypeOf(TraceBatchData{}).PkgPath(),
}

// TraceBatchData represents a batch data
type TraceBatchData struct {
	StartedAt time.Time
}
