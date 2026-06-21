package taxbandits

type AuthResponse struct {
	StatusCode    int    `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
	AccessToken   string `json:"AccessToken"`
}

type FilingResponse struct {
	StatusCode   int         `json:"StatusCode"`
	StatusName   string      `json:"StatusName"`
	SubmissionId string      `json:"SubmissionId"`
	FormRecords  FormRecords `json:"FormRecords,omitempty"`
	Errors       []ApiError  `json:"Errors,omitempty"`
}

type FormRecords struct {
	ErrorRecords []ApiError `json:"ErrorRecords,omitempty"`
}

type ApiError struct {
	Id      string `json:"Id"`
	Name    string `json:"Name"`
	Message string `json:"Message"`
}
