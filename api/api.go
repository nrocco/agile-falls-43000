package api

import (
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/nrocco/bookmarks/queue"
	"github.com/nrocco/bookmarks/storage"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	log "github.com/sirupsen/logrus"
)

//go:generate go-bindata -pkg api -o bindata.go -prefix ../frontend/dist ../frontend/dist/...

// New returns a new instance of API
func New(store *storage.Store, queue *queue.Queue) *API {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Second))
	r.Use(middleware.Heartbeat("/ping"))

	r.Route("/api", func(r chi.Router) {
		r.Use(authenticator(store))
		r.Mount("/bookmarks", bookmarks{store, queue}.Routes())
		r.Mount("/feeds", feeds{store, queue}.Routes())
		r.Mount("/items", items{store, queue}.Routes())
	})

	r.Get("/*", bindataAssetHandler)

	return &API{r}
}

// API glues together HTTP and the Bookmarks Store
type API struct {
	router chi.Router
}

// ListenAndServe binds the API to the given address and listens for requests
func (api *API) ListenAndServe(address string) error {
	log.Infof("Starting webserver at http://%s", address)
	return http.ListenAndServe(address, api.router)
}

type contextKey string

func (c contextKey) String() string {
	return "bookmarks rest api context key " + string(c)
}

func jsonError(w http.ResponseWriter, err error, code int) {
	jsonResponse(w, code, map[string]string{"message": err.Error()})
}

func jsonResponse(w http.ResponseWriter, code int, object interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(object)
}

func bindataAssetHandler(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(r.URL.Path, "/")
	if file == "" {
		file = "index.html"
	}

	asset, err := Asset(file)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	if mimetype := mime.TypeByExtension(filepath.Ext(file)); mimetype != "" {
		w.Header().Set("Content-Type", mimetype)
	}

	w.WriteHeader(200)
	w.Write(asset)
}
