package compliance_framework.mock_plugin_enabled

title := "Mock plugin is enabled"

description := "Passes when the mock plugin's fixed input says it is enabled."

violation contains {"id": "mock_plugin_disabled", "title": "Mock plugin is disabled"} if {
	not input.enabled
}
