package tabwriter

type Writer interface {
	Render() error

	Append(args ...interface{})

	AppendAndFlush(args ...interface{}) error

	SetHeader(header []string)

	Write(buf []byte) (n int, err error)

	Reset()
}
