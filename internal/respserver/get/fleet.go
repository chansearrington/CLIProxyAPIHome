package get

import (
	"context"
	"encoding/json"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
	"github.com/router-for-me/CLIProxyAPIHome/internal/respserver/dispatch"
)

func handleFleetPlan(ctx context.Context, env dispatch.Env, args []string) dispatch.Reply {
	if env.Runtime == nil || len(args) != 3 || env.NodeID == "" || env.ClientCertificateFingerprint == "" {
		return dispatch.Err("fleet plan requires mTLS identity and inventory")
	}
	active, errActive := env.Runtime.PluginSyncNodeActive(ctx, env.NodeID, env.ClientCertificateFingerprint)
	if errActive != nil || !active {
		return dispatch.Err("fleet client certificate is revoked or unknown")
	}
	if len(args[2]) > 4096 {
		return dispatch.Err("fleet inventory too large")
	}
	var inventory fleet.Inventory
	if errDecode := json.Unmarshal([]byte(args[2]), &inventory); errDecode != nil {
		return dispatch.Err("invalid fleet inventory")
	}
	plan, errPlan := env.Runtime.FleetPlan(ctx, env.NodeID, inventory)
	if errPlan != nil {
		return dispatch.Err(errPlan.Error())
	}
	raw, errEncode := json.Marshal(plan)
	if errEncode != nil {
		return dispatch.Err("fleet plan encoding failed")
	}
	return dispatch.SensitiveBulkString(raw)
}
