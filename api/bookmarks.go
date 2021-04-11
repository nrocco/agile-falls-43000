package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nrocco/bookmarks/storage"
)

var (
	contextKeyBookmark = contextKey("bookmark")
)

type bookmarks struct {
	store *storage.Store
}

func (api bookmarks) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", api.list)
	r.Post("/", api.create)
	r.Get("/save", api.save)
	r.Route("/{id}", func(r chi.Router) {
		r.Use(api.middleware)
		r.Get("/", api.get)
		r.Patch("/", api.update)
		r.Delete("/", api.delete)
	})

	return r
}

func (api *bookmarks) list(w http.ResponseWriter, r *http.Request) {
	bookmarks, totalCount := api.store.BookmarkList(r.Context(), &storage.BookmarkListOptions{
		Search: r.URL.Query().Get("q"),
		Tags:   strings.Split(r.URL.Query().Get("tags"), ","),
		Limit:  asInt(r.URL.Query().Get("_limit"), 50),
		Offset: asInt(r.URL.Query().Get("_offset"), 0),
	})

	w.Header().Set("X-Pagination-Total", strconv.Itoa(totalCount))

	jsonResponse(w, 200, bookmarks)
}

func (api *bookmarks) create(w http.ResponseWriter, r *http.Request) {
	var bookmark storage.Bookmark

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&bookmark); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if err := bookmark.Fetch(r.Context()); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	if err := api.store.BookmarkPersist(r.Context(), &bookmark); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, 200, &bookmark)
}

func (api *bookmarks) save(w http.ResponseWriter, r *http.Request) {
	bookmark := storage.Bookmark{
		URL:  r.URL.Query().Get("url"),
		Tags: storage.Tags{"read-it-later"},
	}

	if err := bookmark.Fetch(r.Context()); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	if err := api.store.BookmarkPersist(r.Context(), &bookmark); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	http.Redirect(w, r, bookmark.URL, 302)
}

func (api *bookmarks) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bookmark := storage.Bookmark{ID: chi.URLParam(r, "id")}

		if err := api.store.BookmarkGet(r.Context(), &bookmark); err != nil {
			jsonError(w, "Bookmark Not Found", 404)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyBookmark, &bookmark)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *bookmarks) get(w http.ResponseWriter, r *http.Request) {
	bookmark := r.Context().Value(contextKeyBookmark).(*storage.Bookmark)

	jsonResponse(w, 200, bookmark)
}

func (api *bookmarks) update(w http.ResponseWriter, r *http.Request) {
	bookmark := r.Context().Value(contextKeyBookmark).(*storage.Bookmark)

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(bookmark); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if err := api.store.BookmarkPersist(r.Context(), bookmark); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, 200, bookmark)
}

func (api *bookmarks) delete(w http.ResponseWriter, r *http.Request) {
	bookmark := r.Context().Value(contextKeyBookmark).(*storage.Bookmark)

	if err := api.store.BookmarkDelete(r.Context(), bookmark); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, 204, nil)
}
