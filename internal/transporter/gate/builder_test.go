package gate_test

import (
	"context"
	"testing"
	"time"

	"github.com/devagame/due/v2/cluster"
	"github.com/devagame/due/v2/internal/transporter/gate"
	"github.com/devagame/due/v2/session"
	"github.com/devagame/due/v2/utils/xuuid"
)

func TestBuilder(t *testing.T) {
	builder := gate.NewBuilder(&gate.Options{
		InsID:   xuuid.UUID(),
		InsKind: cluster.Node,
	})

	client, err := builder.Build("127.0.0.1:49899")
	if err != nil {
		t.Fatal(err)
	}

	ip, miss, err := client.GetIP(context.Background(), session.User, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("miss: %v ip: %v", miss, ip)

	ip, miss, err = client.GetIP(context.Background(), session.User, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("miss: %v ip: %v", miss, ip)
}

func TestBuilder_Fault(t *testing.T) {
	builder := gate.NewBuilder(&gate.Options{
		InsID:   xuuid.UUID(),
		InsKind: cluster.Node,
	})

	for i := range 3 {
		if _, err := builder.Build("127.0.0.1:49899"); err != nil {
			t.Log(err)
			time.Sleep(time.Duration(i+1) * time.Second)
		} else {
			t.Log("build success")
		}
	}
}
