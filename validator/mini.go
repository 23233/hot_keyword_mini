package validator

type WeCodeReq struct {
	Code string `json:"code" form:"code" validate:"required,max=64"`
}
