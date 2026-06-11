package net

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSubnet(t *testing.T) {
	cases := []struct {
		subnet        string
		expectedIP    string
		expectedRange string
		expectedBits  uint8
		err           error
	}{
		{
			subnet:        "172.21.0.0/16",
			expectedIP:    "172.21.0.0",
			expectedBits:  16,
			expectedRange: "172.21.0.64-172.21.0.127",
		},
		{
			subnet:        "172.21.0.0/22",
			expectedIP:    "172.21.0.0",
			expectedBits:  22,
			expectedRange: "172.21.0.64-172.21.0.127",
		},
		{
			subnet: "172.21.0.0/25",
			err:    ErrSubnetworkTooSmall,
		},
	}

	for _, c := range cases {
		t.Run(c.subnet, func(t *testing.T) {
			s, err := ParseSubnet(c.subnet)
			if c.err != nil {
				assert.ErrorIs(t, err, c.err)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, c.expectedIP, s.prefix.IP().String())
			assert.Equal(t, c.expectedBits, s.prefix.Bits())

			ok, ipRange := s.NextRange()
			assert.True(t, ok)
			assert.Equal(t, c.expectedRange, ipRange)
		})
	}
}
