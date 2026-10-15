package home

import (
	"context"
	"fmt"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
)

type fleetStore interface {
	FleetPlan(context.Context, string, fleet.Inventory) (fleet.Plan, error)
	FleetReport(context.Context, string, fleet.Result) error
}

func (r *Runtime) FleetPlan(ctx context.Context, nodeID string, in fleet.Inventory) (fleet.Plan, error) {
	store, ok := r.clusterAdapter.(fleetStore)
	if !ok {
		return fleet.Plan{}, fmt.Errorf("fleet store unavailable")
	}
	return store.FleetPlan(ctx, nodeID, in)
}

func (r *Runtime) FleetReport(ctx context.Context, nodeID string, result fleet.Result) error {
	store, ok := r.clusterAdapter.(fleetStore)
	if !ok {
		return fmt.Errorf("fleet store unavailable")
	}
	return store.FleetReport(ctx, nodeID, result)
}
