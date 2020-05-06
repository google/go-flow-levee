package core

func Sanitize(args ...interface{}) []interface{} {
	return args
}

func SanitizeSource(s Source) Source {
	return Source{ID: s.ID}
}
