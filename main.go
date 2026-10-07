// Command plugin is mock-plugin-2: a minimal CCF agent plugin that speaks the
// LEGACY protocol v1. It implements runner.Runner (Configure, Eval) and
// deliberately has no Init, so plugin-probe and the plugin health report have
// a v1 plugin to classify. It is a mock for release automation, not a product.
package main

import (
	"context"
	"errors"
	"slices"

	policyManager "github.com/compliance-framework/agent/policy-manager"
	"github.com/compliance-framework/agent/runner"
	"github.com/compliance-framework/agent/runner/proto"
	mockrunner "github.com/compliance-framework/mock-agent/runner"
	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// pluginName is what Name returns and the "plugin" label on every evidence.
const pluginName = "mock-plugin-2"

// MockPlugin is a protocol v1 plugin: it implements runner.Runner only.
type MockPlugin struct {
	logger     hclog.Logger
	policyData map[string]interface{}
}

var (
	_ runner.Runner     = (*MockPlugin)(nil)
	_ mockrunner.Plugin = (*MockPlugin)(nil)
)

// Name implements mock-agent's runner.Plugin.
func (p *MockPlugin) Name() string {
	return pluginName
}

// Configure accepts any config and ignores it. It keeps the policy data,
// which Eval passes to every policy evaluation.
func (p *MockPlugin) Configure(req *proto.ConfigureRequest) (*proto.ConfigureResponse, error) {
	p.policyData = nil
	if policyData := req.GetPolicyData(); policyData != nil {
		p.policyData = policyData.AsMap()
	}
	return &proto.ConfigureResponse{}, nil
}

// fixedInput is the data every policy is evaluated against. It never changes,
// so a policy over it yields the same evidence on every run.
func fixedInput() map[string]interface{} {
	return map[string]interface{}{
		"name":    pluginName,
		"enabled": true,
	}
}

// Eval evaluates each policy path against the fixed input through
// policy-manager and sends the resulting evidence to the agent.
func (p *MockPlugin) Eval(req *proto.EvalRequest, apiHelper runner.ApiHelper) (*proto.EvalResponse, error) {
	ctx := context.Background()

	subjects := []*proto.Subject{
		{
			Type:       proto.SubjectType_SUBJECT_TYPE_COMPONENT,
			Identifier: "mock-components/" + pluginName,
		},
	}
	components := []*proto.Component{
		{
			Identifier:  "mock-components/" + pluginName,
			Type:        "software",
			Title:       "Mock plugin 2",
			Description: "A mock CCF plugin speaking the legacy protocol v1.",
			Purpose:     "Exercise CCF release automation against a v1 plugin.",
		},
	}
	actors := []*proto.OriginActor{
		{
			Title: "Continuous Compliance Framework - mock-plugin-2",
			Type:  "tool",
			Links: []*proto.Link{
				{
					Href: "https://github.com/compliance-framework/mock-plugin-2",
					Rel:  policyManager.Pointer("reference"),
					Text: policyManager.Pointer("mock-plugin-2"),
				},
			},
		},
	}
	activities := []*proto.Activity{
		{
			Title:       "Collect data",
			Description: "Use a fixed input; the mock plugin collects nothing.",
		},
	}

	evidences := make([]*proto.Evidence, 0)
	var evalErr error
	for _, policyPath := range req.GetPolicyPaths() {
		processor := policyManager.NewPolicyProcessor(
			p.logger,
			map[string]string{
				"provider": "mock",
				"type":     "mock-plugin",
				"plugin":   pluginName,
			},
			subjects,
			components,
			nil,
			actors,
			activities,
			p.policyData,
		)
		evidence, err := processor.GenerateResults(ctx, policyPath, fixedInput())
		evidences = slices.Concat(evidences, evidence)
		if err != nil {
			evalErr = errors.Join(evalErr, err)
		}
	}

	if len(evidences) > 0 {
		if err := apiHelper.CreateEvidence(ctx, evidences); err != nil {
			p.logger.Error("Failed to send evidence", "error", err)
			return &proto.EvalResponse{Status: proto.ExecutionStatus_FAILURE}, err
		}
	}

	if evalErr != nil {
		return &proto.EvalResponse{Status: proto.ExecutionStatus_FAILURE}, evalErr
	}
	return &proto.EvalResponse{Status: proto.ExecutionStatus_SUCCESS}, nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		JSONFormat: true,
	})

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: runner.HandshakeConfig,
		Plugins: map[string]goplugin.Plugin{
			// Protocol v1 on purpose: RunnerGRPCPlugin, not RunnerV2GRPCPlugin.
			"runner": &runner.RunnerGRPCPlugin{
				Impl: &MockPlugin{logger: logger},
			},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
