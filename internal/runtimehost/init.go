package runtimehost

type Initializer[T any] struct {
	Base   func() T
	Config []func(T)
	Start  func(T)
}

func Init[T any](i Initializer[T]) T {
	var zero T
	if i.Base == nil {
		return zero
	}
	v := i.Base()
	for _, fn := range i.Config {
		if fn != nil {
			fn(v)
		}
	}
	if i.Start != nil {
		i.Start(v)
	}
	return v
}
