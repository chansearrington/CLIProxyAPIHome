package get

import (
	"context"
	"github.com/router-for-me/CLIProxyAPIHome/internal/respserver/dispatch"
	"testing"
)

func TestFleetPlanRequiresUnrevokedMTLSIdentity(t *testing.T) {
	rt := pluginSyncTestRuntime(t)
	for _, env := range []dispatch.Env{{Runtime: rt}, {Runtime: rt, NodeID: "fixture", ClientCertificateFingerprint: "fingerprint"}} {
		reply := handleFleetPlan(context.Background(), env, []string{"GET", "fleet-plan", `{}`})
		if reply.Kind != dispatch.ReplyKindRedisError {
			t.Fatal("unverified node received a fleet plan")
		}
	}
}
