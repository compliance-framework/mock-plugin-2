# mock-plugin-2

Mock repo for developing CCF release automation. Not a product.

A real, minimal CCF agent plugin that speaks the **legacy protocol v1**. It is the negative
case for `plugin-probe` ("v1, deprecation candidate") and for the plugin health report
("behind"):

- it implements `runner.Runner` only (`Configure`, `Eval`), with no `Init`, and is served as
  `runner.RunnerGRPCPlugin` under `"runner"`. The agent answers `Init` with
  `codes.Unimplemented` for it;
- it pins `github.com/compliance-framework/agent` v0.8.1, one minor behind the latest, on
  purpose, and `github.com/compliance-framework/mock-agent` at a `main` pseudo-version
  (it implements `mock-agent/runner.Plugin`);
- `Eval` evaluates each policy path against a fixed input through `policy-manager` and sends
  the evidence to the agent.

Don't upgrade it to protocol v2 or bump the agent pin: being behind is its job.

```sh
go test ./...
goreleaser build --snapshot --clean --single-target   # dist/*/plugin
```
