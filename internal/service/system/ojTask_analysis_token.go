package system

import (
	"errors"
	"strings"
	"time"

	"personal_assistant/global"

	"github.com/golang-jwt/jwt/v4"
)

const defaultOJTaskAnalysisTokenTTL = 24 * time.Hour

type ojTaskAnalysisTokenClaims struct {
	Platform   string `json:"platform"`
	Title      string `json:"title"`
	QuestionID uint   `json:"question_id"`
	jwt.RegisteredClaims
}

type ojTaskAnalysisTokenCodec struct {
	secret []byte
	ttl    time.Duration
}

func newOJTaskAnalysisTokenCodec() *ojTaskAnalysisTokenCodec {
	secret := ""
	if global.Config != nil {
		secret = strings.TrimSpace(global.Config.JWT.AccessTokenSecret)
	}
	return &ojTaskAnalysisTokenCodec{
		secret: []byte(secret),
		ttl:    defaultOJTaskAnalysisTokenTTL,
	}
}

func (c *ojTaskAnalysisTokenCodec) Encode(platform, title string, questionID uint) (string, error) {
	if c == nil || len(c.secret) == 0 {
		return "", errors.New("analysis token codec not initialized")
	}
	if strings.TrimSpace(platform) == "" || strings.TrimSpace(title) == "" || questionID == 0 {
		return "", errors.New("invalid analysis token payload")
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &ojTaskAnalysisTokenClaims{
		Platform:   strings.TrimSpace(platform),
		Title:      strings.TrimSpace(title),
		QuestionID: questionID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.ttl)),
		},
	})
	return token.SignedString(c.secret)
}

func (c *ojTaskAnalysisTokenCodec) Decode(token string) (*ojTaskAnalysisTokenClaims, error) {
	if c == nil || len(c.secret) == 0 {
		return nil, errors.New("analysis token codec not initialized")
	}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), &ojTaskAnalysisTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return c.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*ojTaskAnalysisTokenClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid analysis token")
	}
	if strings.TrimSpace(claims.Platform) == "" || strings.TrimSpace(claims.Title) == "" || claims.QuestionID == 0 {
		return nil, errors.New("invalid analysis token claims")
	}
	return claims, nil
}
