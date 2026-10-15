package cluster

import (
	"context"
	"errors"

	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
	"gorm.io/gorm"
)

const fleetStateKey = "__home:fleet:state:v1"

// FleetLoad reads Home-only state. General RESP KV operations cannot address this namespace.
func (r *Repository) FleetLoad(ctx context.Context) ([]byte, bool, error) {
	db, errDB := r.database()
	if errDB != nil {
		return nil, false, errDB
	}
	var row KVRecord
	errFind := db.WithContext(contextOrBackground(ctx)).Where("key = ?", fleetStateKey).First(&row).Error
	if errors.Is(errFind, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if errFind != nil {
		return nil, false, errFind
	}
	return cloneKVBytes(row.Value), true, nil
}

func (r *Repository) FleetSwap(ctx context.Context, old []byte, exists bool, next []byte) (bool, error) {
	db, errDB := r.database()
	if errDB != nil {
		return false, errDB
	}
	return compareAndSwapKV(ctx, db, fleetStateKey, old, exists, next, 0)
}

func (a *RuntimeAdapter) FleetPlan(ctx context.Context, nodeID string, in fleet.Inventory) (fleet.Plan, error) {
	return fleet.New(a.repo).Plan(ctx, nodeID, in)
}

func (a *RuntimeAdapter) FleetReport(ctx context.Context, nodeID string, result fleet.Result) error {
	return fleet.New(a.repo).Report(ctx, nodeID, result)
}
