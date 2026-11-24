package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ctxUserID      = "user_id"
	ctxIsModerator = "is_moderator"
)

// bearerToken — токен из заголовка Authorization: Bearer <token>; ok=false, если заголовка нет
func bearerToken(ctx *gin.Context) (token string, present bool, ok bool) {
	header := ctx.GetHeader("Authorization")
	if header == "" {
		return "", false, false
	}
	token, found := strings.CutPrefix(header, "Bearer ")
	token = strings.TrimSpace(token)
	return token, true, found && token != ""
}

// OptionalAuth — гость проходит без заголовка; валидный токен кладёт user_id и is_moderator в контекст;
// невалидный, просроченный или отозванный токен -> 401
func (h *Handler) OptionalAuth(ctx *gin.Context) {
	token, present, ok := bearerToken(ctx)
	if !present {
		ctx.Next()
		return
	}
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claims, err := h.JWT.ParseToken(token)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	blacklisted, err := h.Blacklist.IsBlacklisted(ctx.Request.Context(), token)
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if blacklisted {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx.Set(ctxUserID, claims.UserID)
	ctx.Set(ctxIsModerator, claims.IsModerator)
	ctx.Next()
}

// CORS — разрешает запросы с другого источника: нативное приложение Tauri обращается к API
// напрямую по IP компьютера в локальной сети, а не через прокси фронтенда
func CORS(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Origin")
	ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	if ctx.Request.Method == http.MethodOptions {
		ctx.AbortWithStatus(http.StatusNoContent)
		return
	}
	ctx.Next()
}

// RequireAuth — 401, если в контексте нет пользователя (ставится после OptionalAuth)
func (h *Handler) RequireAuth(ctx *gin.Context) {
	if _, ok := currentUserID(ctx); !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ctx.Next()
}

// RequireModerator — 401 без пользователя, 403 для не-модератора
func (h *Handler) RequireModerator(ctx *gin.Context) {
	if _, ok := currentUserID(ctx); !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !isModerator(ctx) {
		ctx.AbortWithStatus(http.StatusForbidden)
		return
	}
	ctx.Next()
}

func currentUserID(ctx *gin.Context) (uint, bool) {
	v, ok := ctx.Get(ctxUserID)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok
}

func isModerator(ctx *gin.Context) bool {
	return ctx.GetBool(ctxIsModerator)
}

// isOwner — для гостя всегда false
func isOwner(ctx *gin.Context, creatorID uint) bool {
	userID, ok := currentUserID(ctx)
	return ok && userID == creatorID
}

// ownerFlag — признак 0/1 для JSON: 1, если услугу создал текущий пользователь; для гостя всегда 0
func ownerFlag(ctx *gin.Context, creatorID uint) int {
	if isOwner(ctx, creatorID) {
		return 1
	}
	return 0
}
