package core

// Source will be configured to be detected as a source struct, with Source.Data as the source field.
type Source struct {
	Data string
	ID   int
}

func (s Source) GetID() int {
	return s.ID
}

func (s Source) GetData() string {
	return s.Data
}

// Innocuous will _not_ be configured to be a source, even though underlying types are equal.
type Innocuous struct {
	Data string
	ID   int
}

func (i Innocuous) GetID() int {
	return i.ID
}

func (i Innocuous) GetData() string {
	return i.Data
}
