package constraints

type Bytes interface {
	~string | ~[]byte
}
