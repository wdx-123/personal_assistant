package response

import "personal_assistant/internal/model/entity"

// LoginResponse 登录成功返回结构体
type LoginResponse struct {
    User                   entity.User `json:"user"`
    AccessToken            string      `json:"access_token"`
    AccessTokenExpiresAt   int64       `json:"access_token_expires_at"`
    RefreshToken           string      `json:"refresh_token"`
}

func (l LoginResponse) ToResponse(input *LoginResponse) *LoginResponse {
    return input
}
