package ssh

type network int

const (
	tcp network = iota
	tcp4
	tcp6
)

var networkType = map[network]string{
	tcp:  "tcp",
	tcp4: "tcp4",
	tcp6: "tcp6",
}

func (n network) String() string {
	return networkType[n]
}
