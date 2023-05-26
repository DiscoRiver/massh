package ssh

var (
	// Valid Network types
	NetworkTCP  = TCPNetwork{Name: "tcp"}
	NetworkTCP4 = TCP4Network{Name: "tcp4"}
	NetworkTCP6 = TCP6Network{Name: "tcp6"}
)

type Network interface {
	GetName() string
}

type TCPNetwork struct {
	Name string
}

func (n *TCPNetwork) GetName() string {
	return n.Name
}

type TCP4Network struct {
	Name string
}

func (n *TCP4Network) GetName() string {
	return n.Name
}

type TCP6Network struct {
	Name string
}

func (n *TCP6Network) GetName() string {
	return n.Name
}
