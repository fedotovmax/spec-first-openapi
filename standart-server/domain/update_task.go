package domain

type UpdateTask struct {
	Title  Nullable[string]
	Email  string
	Status *string
}
