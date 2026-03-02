package response

type HealthResponse struct {
	Status string `json:"status"` // "UP", "DOWN"
	DB     string `json:"db"`     // "UP", "DOWN"
	Redis  string `json:"redis"`  // "UP", "DOWN"
}

type PingResponse struct {
	Message string `json:"message"` // "pong"
}
