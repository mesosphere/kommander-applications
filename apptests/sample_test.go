package apptests

import (
	"context"
	"log"
	"testing"

	"github.com/d2iq-labs/cluster-mechanics/kindcluster"
)

func TestXXX(t *testing.T) {
	kc, err := kindcluster.NewCluster()
	if err != nil {
		log.Fatal(err)
	}

	err = kc.CreateCluster(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}
