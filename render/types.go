package render

// RenderInput represents the input data for template rendering
type RenderInput struct {
	// Template is the Mako template content
	Template string `json:"template"`
	// Context contains the variables to be used in template rendering
	Context map[string]interface{} `json:"context"`
}

// RenderOutput represents the output of template rendering
type RenderOutput struct {
	// Result is the rendered content
	Result string
	// Error contains error message if rendering failed
	Error string
}

// ProcessContext represents the context data structure for process template rendering
// This matches the structure from get_process_context in the original Python code
type ProcessContext struct {
	Scope           string                 `json:"Scope"`
	FuncID          string                 `json:"FuncID"`
	InstID          int                    `json:"InstID"`
	InstID0         int                    `json:"InstID0"`
	LocalInstID     int                    `json:"LocalInstID"`
	LocalInstID0    int                    `json:"LocalInstID0"`
	BkSetName       string                 `json:"bk_set_name"`
	BkModuleName    string                 `json:"bk_module_name"`
	BkHostInnerIP   string                 `json:"bk_host_innerip"`
	BkCloudID       int                    `json:"bk_cloud_id"`
	BkProcessID     int                    `json:"bk_process_id"`
	BkProcessName   string                 `json:"bk_process_name"`
	FuncName        string                 `json:"FuncName"`
	ProcName        string                 `json:"ProcName"`
	WorkPath        string                 `json:"WorkPath"`
	GlobalVariables map[string]interface{} `json:"global_variables"`
}
