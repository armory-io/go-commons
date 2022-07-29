package logging

type Logger interface {
	Debug(msg string, tags ...Tag)
	Info(msg string, tags ...Tag)
	Warn(msg string, tags ...Tag)
	Error(msg string, tags ...Tag)
	Fatal(msg string, tags ...Tag)
}

type Tag interface {
	Key() string
	Value() interface{}
}
