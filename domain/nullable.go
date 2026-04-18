package domain

type Nullable[T any] struct {
	Value *T
	Set   bool
}

type OpenApiNullable[T any] interface {
	Get() (T, error)
	IsNull() bool
	IsSpecified() bool
}

func MapNullable[T_API any, T_DOMAIN any](
	apiNull OpenApiNullable[T_API],
	transform func(T_API) T_DOMAIN,
) Nullable[T_DOMAIN] {
	if !apiNull.IsSpecified() {
		return Nullable[T_DOMAIN]{Set: false}
	}

	if apiNull.IsNull() {
		return Nullable[T_DOMAIN]{
			Value: nil,
			Set:   true,
		}
	}

	val, _ := apiNull.Get()
	res := transform(val)

	return Nullable[T_DOMAIN]{
		Value: &res,
		Set:   true,
	}
}
