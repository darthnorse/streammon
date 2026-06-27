package server

import (
	"encoding/csv"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"streammon/internal/store"
)

var allowedLibrarySortColumns = map[string]string{
	"added_at":    "li.added_at",
	"title":       "li.title",
	"last_played": "last_played_at",
	"plays":       "plays",
	"total_time":  "watched_ms",
	"viewers":     "unique_viewers",
	"size":        "li.file_size",
}

func (s *Server) libraryQueryFromRequest(r *http.Request) (store.LibraryItemQuery, bool) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		return store.LibraryItemQuery{}, false
	}
	libraryID := chi.URLParam(r, "libraryID")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	q := store.LibraryItemQuery{
		ServerID: serverID, LibraryID: libraryID,
		Page: page, PerPage: perPage,
		SortColumn: allowedLibrarySortColumns[r.URL.Query().Get("sort_by")],
		SortOrder:  r.URL.Query().Get("sort_order"),
	}

	switch r.URL.Query().Get("filter") {
	case "played":
		q.Filter = "played"
	case "unplayed":
		q.Filter = "unplayed"
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > maxSearchLength {
		search = search[:maxSearchLength]
	}
	q.Search = search
	return q, true
}

func (s *Server) handleListLibraryItems(w http.ResponseWriter, r *http.Request) {
	q, ok := s.libraryQueryFromRequest(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	if r.URL.Query().Get("format") == "csv" {
		s.writeLibraryItemsCSV(w, r, q)
		return
	}

	result, err := s.store.ListLibraryItemDetails(r.Context(), q)
	if err != nil {
		log.Printf("list library items (server %d, library %q): %v", q.ServerID, q.LibraryID, err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeLibraryItemsCSV(w http.ResponseWriter, r *http.Request, q store.LibraryItemQuery) {
	q.PerPage = 0 // all rows
	result, err := s.store.ListLibraryItemDetails(r.Context(), q)
	if err != nil {
		log.Printf("library items CSV (server %d, library %q): %v", q.ServerID, q.LibraryID, err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="library-items.csv"`)

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Title", "Year", "Type", "Added At", "Last Played", "Plays",
		"Total Hours", "Unique Viewers", "Last Viewer", "Episodes Watched", "Episode Count",
		"File Size", "Resolution", "TMDB Status", "Flagged", "Protected"})
	for _, it := range result.Items {
		lastPlayed := ""
		if it.LastPlayedAt != nil {
			lastPlayed = it.LastPlayedAt.Format("2006-01-02 15:04:05")
		}
		_ = cw.Write([]string{
			csvSafe(it.Title), strconv.Itoa(it.Year), string(it.MediaType),
			it.AddedAt.Format("2006-01-02 15:04:05"), lastPlayed, strconv.Itoa(it.Plays),
			strconv.FormatFloat(it.TotalHours, 'f', 2, 64), strconv.Itoa(it.UniqueViewers),
			csvSafe(it.LastViewer), strconv.Itoa(it.EpisodesWatched), strconv.Itoa(it.EpisodeCount),
			strconv.FormatInt(it.FileSize, 10), csvSafe(it.VideoResolution), csvSafe(it.TMDBStatus),
			strconv.FormatBool(it.FlaggedForDeletion), strconv.FormatBool(it.Protected),
		})
	}
	cw.Flush()
}

func (s *Server) handleLibraryItemSummary(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "serverID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}
	summary, err := s.store.GetLibrarySummary(r.Context(), serverID, chi.URLParam(r, "libraryID"))
	if err != nil {
		log.Printf("library summary (server %d, library %q): %v", serverID, chi.URLParam(r, "libraryID"), err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// csvSafe neutralizes spreadsheet formula injection: a cell beginning with a
// formula trigger (= + - @) or a leading control char is prefixed with a single
// quote so Excel/Sheets/LibreOffice treat the value as text.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
