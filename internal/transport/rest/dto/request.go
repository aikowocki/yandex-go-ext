package dto

// ErrorResponse описывает ошибку REST API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	MaxSize int64  `json:"max_size,omitempty"`
}

// PaginationResponse содержит страницу результатов и параметры пагинации.
type PaginationResponse struct {
	Items  any `json:"items"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
