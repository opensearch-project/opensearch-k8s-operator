package responses

type EnableBalanceRoutingMode int64

const (
	all EnableBalanceRoutingMode = iota
	primaries
	replicas
	none
)

func (s EnableBalanceRoutingMode) String() string {
	switch s {
	case all:
		return "all"
	case primaries:
		return "primaries"
	case replicas:
		return "replicas"
	case none:
		return "none"
	}
	return "all"
}
