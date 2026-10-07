package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compliance-framework/agent/runner"
	"github.com/compliance-framework/agent/runner/proto"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// fakeApiHelper records what the plugin sends to the agent.
type fakeApiHelper struct {
	evidence []*proto.Evidence
}

func (f *fakeApiHelper) CreateEvidence(_ context.Context, evidence []*proto.Evidence) error {
	f.evidence = append(f.evidence, evidence...)
	return nil
}

func (f *fakeApiHelper) UpsertRiskTemplates(context.Context, string, []*proto.RiskTemplate) error {
	return nil
}

func (f *fakeApiHelper) UpsertSubjectTemplates(context.Context, []*proto.SubjectTemplate) error {
	return nil
}

func newTestPlugin() *MockPlugin {
	return &MockPlugin{logger: hclog.NewNullLogger()}
}

func TestName(t *testing.T) {
	if got := newTestPlugin().Name(); got != "mock-plugin-2" {
		t.Fatalf("Name() = %q, want %q", got, "mock-plugin-2")
	}
}

// The plugin must stay on protocol v1: a Runner, never a RunnerV2.
func TestIsProtocolV1(t *testing.T) {
	var impl interface{} = newTestPlugin()
	if _, ok := impl.(runner.Runner); !ok {
		t.Fatal("MockPlugin does not implement runner.Runner")
	}
	if _, ok := impl.(runner.RunnerV2); ok {
		t.Fatal("MockPlugin implements runner.RunnerV2; it must stay a protocol v1 plugin")
	}
}

func TestConfigureAcceptsAnyConfig(t *testing.T) {
	p := newTestPlugin()
	cfgs := []map[string]string{
		nil,
		{},
		{"anything": "goes", "token": ""},
	}
	for _, cfg := range cfgs {
		if _, err := p.Configure(&proto.ConfigureRequest{Config: cfg}); err != nil {
			t.Fatalf("Configure(%v) returned error: %v", cfg, err)
		}
	}
}

func TestEvalProducesOneEvidence(t *testing.T) {
	p := newTestPlugin()
	if _, err := p.Configure(&proto.ConfigureRequest{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	api := &fakeApiHelper{}
	resp, err := p.Eval(&proto.EvalRequest{PolicyPaths: []string{"testdata/policies"}}, api)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if resp.GetStatus() != proto.ExecutionStatus_SUCCESS {
		t.Fatalf("Eval status = %v, want SUCCESS", resp.GetStatus())
	}
	if len(api.evidence) != 1 {
		t.Fatalf("got %d evidence, want 1", len(api.evidence))
	}

	ev := api.evidence[0]
	if ev.GetTitle() != "Mock plugin is enabled" {
		t.Errorf("evidence title = %q", ev.GetTitle())
	}
	if ev.GetStatus().GetState() != proto.EvidenceStatusState_EVIDENCE_STATUS_STATE_SATISFIED {
		t.Errorf("evidence state = %v, want SATISFIED", ev.GetStatus().GetState())
	}
	if ev.GetLabels()["plugin"] != "mock-plugin-2" {
		t.Errorf("evidence plugin label = %q", ev.GetLabels()["plugin"])
	}
	if ev.GetPolicyEvaluation().GetPolicyPath() != "testdata/policies" {
		t.Errorf("policy evaluation path = %q", ev.GetPolicyEvaluation().GetPolicyPath())
	}
}

func TestEvalPassesPolicyDataFromConfigure(t *testing.T) {
	policyData, err := structpb.NewStruct(map[string]interface{}{"threshold": 3})
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	p := newTestPlugin()
	if _, err := p.Configure(&proto.ConfigureRequest{PolicyData: policyData}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	api := &fakeApiHelper{}
	if _, err := p.Eval(&proto.EvalRequest{PolicyPaths: []string{"testdata/policies"}}, api); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(api.evidence) != 1 {
		t.Fatalf("got %d evidence, want 1", len(api.evidence))
	}

	var got map[string]interface{}
	if err := json.Unmarshal(api.evidence[0].GetPolicyEvaluation().GetPolicyData(), &got); err != nil {
		t.Fatalf("decode policy data %q: %v", api.evidence[0].GetPolicyEvaluation().GetPolicyData(), err)
	}
	if got["threshold"] != float64(3) {
		t.Fatalf("policy data = %v, want threshold 3", got)
	}
}

func TestEvalBadPolicyPathFails(t *testing.T) {
	p := newTestPlugin()
	api := &fakeApiHelper{}
	resp, err := p.Eval(&proto.EvalRequest{PolicyPaths: []string{"testdata/does-not-exist"}}, api)
	if err == nil {
		t.Fatal("Eval with a missing policy path returned no error")
	}
	if resp.GetStatus() != proto.ExecutionStatus_FAILURE {
		t.Fatalf("Eval status = %v, want FAILURE", resp.GetStatus())
	}
	if len(api.evidence) != 0 {
		t.Fatalf("got %d evidence, want 0", len(api.evidence))
	}
}

// Served the way main serves it, Init over go-plugin gRPC must be Unimplemented:
// that is how the agent and plugin-probe recognise a protocol v1 plugin.
func TestServedInitIsUnimplemented(t *testing.T) {
	client, _ := goplugin.TestPluginGRPCConn(t, true, map[string]goplugin.Plugin{
		"runner": &runner.RunnerGRPCPlugin{Impl: newTestPlugin()},
	})
	t.Cleanup(func() { _ = client.Close() })

	raw, err := client.Dispense("runner")
	if err != nil {
		t.Fatalf("Dispense: %v", err)
	}
	v2, ok := raw.(runner.RunnerV2)
	if !ok {
		t.Fatalf("dispensed client %T does not expose Init", raw)
	}

	_, err = v2.Init(&proto.InitRequest{}, &fakeApiHelper{})
	if got := status.Code(err); got != codes.Unimplemented {
		t.Fatalf("Init returned code %v (err %v), want Unimplemented", got, err)
	}
}
