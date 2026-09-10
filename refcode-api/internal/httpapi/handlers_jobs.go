package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"refcode-api/internal/store"
	"refcode-api/internal/store/dbgen"
	"refcode-api/internal/worker"
)

// 排程的間隔下限跟資料庫的 CHECK 對齊（見 00019）。上限擋在 30 天：
// 比這更久的等於關掉，那該用 enabled 表達，不然後台會出現「開著但幾乎不跑」
// 這種看不出意圖的狀態。
const (
	minJobIntervalSeconds = 60
	maxJobIntervalSeconds = 30 * 24 * 60 * 60
)

// handleListJobs 回排程列表。開關與間隔在資料庫，說明文字在程式碼
// （worker.Job.Description），這裡合起來給後台。
//
// 只列程式裡還註冊著的那些：job 被拿掉之後資料庫的列不會自動消失
// （留著是為了保住手動調過的設定，萬一改回來還在），但後台不該看到一支
// 按了不會有反應的排程。
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ListJobs(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}

	registered := map[string]worker.Job{}
	for _, j := range s.jobs {
		registered[j.Name] = j
	}

	type item struct {
		dbgen.ListJobsRow
		Description string `json:"description"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		j, ok := registered[row.Name]
		if !ok {
			continue
		}
		out = append(out, item{ListJobsRow: row, Description: j.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

func (s *Server) handleListJobRuns(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !s.jobRegistered(name) {
		notFound(w, codeJobNotFound, "沒有這支排程")
		return
	}

	limit, _ := paginate(r, 20, 100)
	rows, err := s.store.ListJobRuns(r.Context(), dbgen.ListJobRunsParams{
		JobName:  name,
		RowLimit: limit,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": rows})
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !s.jobRegistered(name) {
		notFound(w, codeJobNotFound, "沒有這支排程")
		return
	}

	var body struct {
		Enabled         *bool  `json:"enabled"`
		IntervalSeconds *int32 `json:"interval_seconds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return
	}
	if body.Enabled == nil && body.IntervalSeconds == nil {
		badRequest(w, codeJobNothingToUpdate, "要改 enabled 或 interval_seconds 其中之一")
		return
	}

	// 兩個欄位分兩支查詢寫，因為改間隔要順便標 interval_overridden，改開關不要。
	// 同一個請求可以兩個都帶，依序套用。
	var row dbgen.ScheduledJob
	var err error

	if body.IntervalSeconds != nil {
		v := *body.IntervalSeconds
		if v < minJobIntervalSeconds || v > maxJobIntervalSeconds {
			badRequest(w, codeJobIntervalInvalid, "間隔要介於 60 秒與 30 天之間")
			return
		}
		row, err = s.store.SetJobInterval(r.Context(), dbgen.SetJobIntervalParams{
			Name:            name,
			IntervalSeconds: v,
		})
		if err != nil {
			internalError(w, r, err)
			return
		}
	}
	if body.Enabled != nil {
		row, err = s.store.SetJobEnabled(r.Context(), dbgen.SetJobEnabledParams{
			Name:    name,
			Enabled: *body.Enabled,
		})
		if err != nil {
			internalError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, row)
}

// handleRunJob 只是留一個記號，真正跑起來是 worker 下一次輪詢時認領到的
// （最多 30 秒）—— HTTP handler 不該等一支爬蟲跑完。
func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !s.jobRegistered(name) {
		notFound(w, codeJobNotFound, "沒有這支排程")
		return
	}

	row, err := s.store.RequestJobRun(r.Context(), name)
	if err != nil {
		// RequestJobRun 帶 enabled 條件，關掉的那些會回 no rows。
		if store.IsNotFound(err) {
			badRequest(w, codeJobDisabled, "這支排程是關閉的，先打開再執行")
			return
		}
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, row)
}

func (s *Server) jobRegistered(name string) bool {
	for _, j := range s.jobs {
		if j.Name == name {
			return true
		}
	}
	return false
}
