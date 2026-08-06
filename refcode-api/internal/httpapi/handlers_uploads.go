package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"

	"refcode-api/internal/store/dbgen"
)

// Logo、分類圖都是小圖示，5MB 綽綽有餘，順便擋掉誤傳大檔案。
const maxUploadImageSize = 5 << 20

// 大頭照給得比後台小：來源是任何登入的使用者，而且 app 端上傳前就縮到 512px 了，
// 超過這個大小多半是繞過前端直接打 API。
const maxAvatarSize = 2 << 20

// readImageUpload 把 multipart 的檔案取出來並驗過型別，三支上傳共用同一套規則。
// 回傳的 file 由呼叫端負責 Close。
func (s *Server) readImageUpload(w http.ResponseWriter, r *http.Request, maxSize int64) (multipart.File, bool) {
	if !s.images.Enabled() {
		internalError(w, r, errors.New("cloudinary 未設定"))
		return nil, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		badRequest(w, codeImageInvalid, fmt.Sprintf("檔案太大（上限 %dMB）或格式錯誤", maxSize>>20))
		return nil, false
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, codeImageInvalid, "缺少檔案")
		return nil, false
	}

	if ct := header.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		file.Close()
		badRequest(w, codeImageInvalid, "只能上傳圖片檔案")
		return nil, false
	}
	return file, true
}

func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	// 存放位置分開，方便之後直接去 Cloudinary 後台找特定用途的圖。
	// 先看 folder 再讀檔，省得白傳一份上來才發現參數錯。
	folder := r.FormValue("folder")
	if folder != "merchants" && folder != "categories" {
		badRequest(w, codeInvalidRequest, "folder 只能是 merchants 或 categories")
		return
	}

	file, ok := s.readImageUpload(w, r, maxUploadImageSize)
	if !ok {
		return
	}
	defer file.Close()

	// 服務商 logo 與分類圖沒有「換掉就該刪舊的」的問題（後台可能同一張圖用在多處），
	// public_id 這裡用不到。
	url, _, err := s.images.Upload(r.Context(), file, folder)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// handleUploadAvatar 上傳大頭照並直接寫回 users.avatar_url，回整份使用者資料。
//
// 不讓前端「上傳完自己再打 PATCH /me」：那支是整份覆寫，中間隔一個 round trip，
// 同時在別的裝置改了顯示名稱就會被舊值蓋掉。
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	file, ok := s.readImageUpload(w, r, maxAvatarSize)
	if !ok {
		return
	}
	defer file.Close()

	// 先把舊的 public_id 記下來，等新的存進去之後再刪 —— 反過來的話中間出錯，
	// 使用者的大頭照會變成指向已刪圖片的破圖。
	before, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	userID := before.ID

	url, publicID, err := s.images.Upload(ctx, file, "avatars")
	if err != nil {
		internalError(w, r, err)
		return
	}

	user, err := s.store.UpdateUserAvatar(ctx, dbgen.UpdateUserAvatarParams{
		ID:             userID,
		AvatarUrl:      &url,
		AvatarPublicID: &publicID,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	// 換掉的舊圖要刪，不然每換一次就在 Cloudinary 上留一張公開網址的臉。
	// 刪失敗不影響這次上傳的結果，記一筆就好 —— 使用者那邊已經換好了。
	if before.AvatarPublicID != nil {
		if err := s.images.Destroy(ctx, *before.AvatarPublicID); err != nil {
			slog.Warn("舊大頭照刪不掉", "user_id", userID, "public_id", *before.AvatarPublicID, "err", err)
		}
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}
