package ssh

type Network int

const (
	TCP Network = iota
	TCP4
	TCP6
)

var networkType = map[Network]string{
	TCP:  "tcp",
	TCP4: "tcp4",
	TCP6: "tcp6",
}

func (n Network) String() string {
	return networkType[n]
}
