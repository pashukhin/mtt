package cli

import (
	"testing"

	"github.com/pashukhin/mtt/internal/core"
)

func TestToSelfUpdateJSONCheckOnly(t *testing.T) {
	p := core.Plan{Current: "v0.8.0", Latest: "v0.9.0", State: core.UpdateAvailable, Via: core.ViaAsset, AssetName: "mtt_v0.9.0_linux_amd64"}
	j := toSelfUpdateJSON(p, core.Result{}, false, nil)
	// check-only: nothing fetched/installed -> asset/path empty (D8)
	if !j.UpdateAvailable || j.Updated || j.Via != "asset" || j.Latest != "v0.9.0" || j.Asset != "" || j.Path != "" {
		t.Fatalf("check-only json: %+v", j)
	}
	// applied
	r := core.Result{Tag: "v0.9.0", Via: core.ViaAsset, Path: "/usr/local/bin/mtt"}
	j = toSelfUpdateJSON(p, r, true, nil)
	if !j.Updated || j.Path != "/usr/local/bin/mtt" || j.Asset != "mtt_v0.9.0_linux_amd64" {
		t.Fatalf("applied json: %+v", j)
	}
	// via none carries the reason
	pn := core.Plan{Current: "dev", Latest: "v0.9.0", State: core.UpdateAvailable, Via: core.ViaNone, Reason: "no asset"}
	j = toSelfUpdateJSON(pn, core.Result{}, false, nil)
	if j.Via != "none" || j.Reason == "" {
		t.Fatalf("via none json: %+v", j)
	}
}
