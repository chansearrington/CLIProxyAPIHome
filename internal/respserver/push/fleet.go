package push

import (
	"context"
	"encoding/json"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
	"github.com/router-for-me/CLIProxyAPIHome/internal/respserver/dispatch"
)

func handleFleetStatus(ctx context.Context, env dispatch.Env, args []string) dispatch.Reply {
	if env.Runtime == nil || len(args) != 3 || env.NodeID == "" || env.ClientCertificateFingerprint == "" {
		return dispatch.Err("fleet status requires mTLS identity")
	}
	active, errActive := env.Runtime.PluginSyncNodeActive(ctx, env.NodeID, env.ClientCertificateFingerprint)
	if errActive != nil || !active {
		return dispatch.Err("fleet client certificate is revoked or unknown")
	}
	if len(args[2]) > 8192 {
		return dispatch.Err("fleet result too large")
	}
	var result fleet.Result
	if errDecode := json.Unmarshal([]byte(args[2]), &result); errDecode != nil {
		return dispatch.Err("invalid fleet result")
	}
	if errReport := env.Runtime.FleetReport(ctx, env.NodeID, result); errReport != nil {
		return dispatch.Err(errReport.Error())
	}
	return dispatch.Integer(1)
}
