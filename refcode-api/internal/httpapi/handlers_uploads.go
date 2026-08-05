package httpapi

import (
	"errors"
	"net/http"
	"strings"
)

// Logo、分類圖都是小圖示，5MB 綽綽有餘，順便擋掉誤傳大檔案。
const maxUploadImageSize = 5 << 20

func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	if !s.images.Enabled() {
		internalError(w, r, errors.New("cloudinary 未設定"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadImageSize)
	if err := r.ParseMultipartForm(maxUploadImageSize); err != nil {
		badRequest(w, codeImageInvalid, "檔案太大（上限 5MB）或格式錯誤")
		return
	}

	// 存放位置分開，方便之後直接去 Cloudinary 後台找特定用途的圖。
	folder := r.FormValue("folder")
	if folder != "merchants" && folder != "categories" {
		badRequest(w, codeInvalidRequest, "folder 只能是 merchants 或 categories")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, codeImageInvalid, "缺少檔案")
		return
	}
	defer file.Close()

	if ct := header.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		badRequest(w, codeImageInvalid, "只能上傳圖片檔案")
		return
	}

	url, err := s.images.Upload(r.Context(), file, folder)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}
