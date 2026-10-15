package cluster

import (
	"context"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
	"testing"
)

func TestFleetStateUsesSnapshotBackedKVWithoutGeneralRESPAccess(t *testing.T) {
	repo, ctx := newTestKVRepository(t)
	service := fleet.New(repo)
	if errPolicy := service.SetPolicy(ctx, fleet.Policy{IntervalSeconds: 3600}); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	raw, found, errLoad := repo.FleetLoad(ctx)
	if errLoad != nil || !found || len(raw) == 0 {
		t.Fatalf("fleet state not stored: %v %v", found, errLoad)
	}
	if _, _, errRead := repo.KVGet(ctx, fleetStateKey); errRead == nil {
		t.Fatal("general KV could read Home policy")
	}
	if _, errWrite := repo.KVSet(ctx, fleetStateKey, []byte("forged"), 0, KVSetModeAlways); errWrite == nil {
		t.Fatal("general KV could overwrite Home policy")
	}
	if _, errDelete := repo.KVDel(ctx, []string{fleetStateKey}); errDelete == nil {
		t.Fatal("general KV could delete Home policy")
	}
	if errWrite := repo.KVMSet(ctx, map[string][]byte{fleetStateKey: []byte("forged")}); errWrite == nil {
		t.Fatal("general MSET could overwrite Home policy")
	}
	if _, errCAS := repo.KVCompareAndSwap(ctx, fleetStateKey, raw, true, []byte("forged"), 0); errCAS == nil {
		t.Fatal("general CAS could overwrite Home policy")
	}
	newer, foundNew, errNew := repo.FleetLoad(context.Background())
	if errNew != nil || !foundNew || string(newer) != string(raw) {
		t.Fatal("protected state changed")
	}
}
