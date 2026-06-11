package net

import (
	"errors"
	"fmt"
	"strings"

	"inet.af/netaddr"
)

const (
	// MinimumSubnetNewBits is the number of additional bits with which to extend the prefix. if given a prefix
	// ending in /24 and a newbits value of 2, the resulting subnet address will have length /26.
	//
	// This is used to ensure that the subnet address is not too small to be used by the cluster. Using a subnet
	// newbits value of 2 ensures that we have at least 4 subnets available for the network.
	MinimumSubnetNewBits = 2

	// DefaultRangePrefixBits is the default prefix length for the subnet. It is used to create smaller subnets.
	// Our assumption is that the subnet /26 (contains 64 ip) is large enough to fit all the containers.
	DefaultRangePrefixBits = 26

	// SingleIPSubnetPrefixBits is used to create a single ip address.
	SingleIPSubnetPrefixBits = 32
)

var ErrSubnetworkTooSmall = errors.New("IP subnetwork is too small")

// Subnet holds a given subnetwork and splits it into separate ranges that can be calculated using `NextRange` function.
type Subnet struct {
	prefix netaddr.IPPrefix
	ipSet  *netaddr.IPSet
}

func ParseSubnet(s string) (*Subnet, error) {
	prefix, err := netaddr.ParseIPPrefix(s)
	if err != nil {
		return nil, err
	}

	// DefaultRangePrefixBits - MinimumSubnetNewBits gives us the minimum prefix bits big enough to fit
	// number of subnets we require to use in tests.
	//
	// When DefaultRangePrefixBits is 26 and MinimumSubnetNewBits is 2, we need to have prefix bits 24 or lower to
	// be able to fit 4 subnets.
	//
	//       notation       addrs/block
	//       --------       -----------
	//       n.n.n.x/26              64
	//       n.n.n.x/25             128
	//       n.n.n.0/24             256
	//       n.n.n.0/23             512
	if DefaultRangePrefixBits-MinimumSubnetNewBits < prefix.Bits() {
		return nil, ErrSubnetworkTooSmall
	}

	builder := netaddr.IPSetBuilder{}
	builder.AddRange(prefix.Range())

	ipSet, err := builder.IPSet()
	if err != nil {
		return nil, err
	}

	subnet := &Subnet{
		prefix: prefix,
		ipSet:  ipSet,
	}

	// Remove the first subnet to avoid clash with the cluster containers
	ok, _ := subnet.NextRange()
	if !ok {
		return nil, fmt.Errorf("cannot remove the first range from subnetwork: %w", ErrSubnetworkTooSmall)
	}

	return subnet, nil
}

func (s *Subnet) IPRange() string {
	return s.prefix.Range().String()
}

// NextRange return new ip range with given bitLen and removes it from subnet ip sets.
func (s *Subnet) NextRange() (bool, string) {
	r, newIPSet, ok := s.ipSet.RemoveFreePrefix(DefaultRangePrefixBits)

	s.ipSet = newIPSet

	return ok && r.IsValid(), r.Range().String()
}

// NextIP returns a single ip and removes it from subnet ip sets.
func (s *Subnet) NextIP() (bool, string) {
	r, newIPSet, ok := s.ipSet.RemoveFreePrefix(SingleIPSubnetPrefixBits)

	s.ipSet = newIPSet

	return ok && r.IsValid(), r.IP().String()
}

func (s *Subnet) String() string {
	ranges := make([]string, 0, len(s.ipSet.Ranges()))
	for _, s := range s.ipSet.Ranges() {
		ranges = append(ranges, s.String())
	}
	return fmt.Sprintf("ip: %s, bits: %d, ranges: %s", s.prefix.IP(), s.prefix.Bits(), strings.Join(ranges, ", "))
}
