package response

// RefreshTokenResponse 刷新token响应结构体
type RefreshTokenResponse struct {
	AccessToken          string `json:"access_token"`            // 新的访问令牌
	AccessTokenExpiresAt int64  `json:"access_token_expires_at"` // 访问令牌过期时间戳（毫秒）
	RefreshToken         string `json:"refresh_token"`
}

// AuthResponse token无效的返回方式
type AuthResponse struct {
	Message string `json:"message"`
	Reload  bool   `json:"reload,omitempty"` // 告诉前端刷新数据
}

func (a RefreshTokenResponse) ToResponse(input *RefreshTokenResponse) *RefreshTokenResponse {
	return input
}
func (a *AuthResponse) ToResponse(input *AuthResponse) *AuthResponse {
	return input
}

/*
	{
	  "code": 4010,
	  "data": {
		"message": "Invalid access token",
		"reload": true
	  },
	  "error": "Invalid access token"
	}
*/

/*

  -- 为何为true --
当短token过期时，前端可能仍然缓存着：
	- 用户登录状态
	- 页面数据
	- 路由状态
	- 本地存储的过期token
reload: true 告诉前端需要 完全重置这些状态 。

*/
