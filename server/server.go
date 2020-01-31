package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/gorilla/mux"
)

const staticDir = "../static"

func main() {
	port := flag.Int("p", 8080, "port to listen at")
	help := flag.Bool("h", false, "show this help")

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	log.Printf("Listening on http://localhost:%d/", *port)

	artists = make(map[string]Artist)
	moves = make(map[string][]Move)

	r := mux.NewRouter()
	apiR := r.PathPrefix("/api/").Subrouter()
	apiR.HandleFunc("/artists", CreateArtistsHandler).Methods(http.MethodPost)
	apiR.HandleFunc("/artists", ArtistsHandler).Methods(http.MethodGet)
	apiR.HandleFunc("/artists/{artistID}/moves", CreateMoveHandler).Methods(http.MethodPost)
	apiR.HandleFunc("/artists/{artistID}/moves", MovesHandler).Methods(http.MethodGet)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(staticDir)))

	http.Handle("/", r)
	// http.Handle("/", )

	if _, err := os.Stat(gzPath("/client.js")); err == nil {
		http.HandleFunc("/client.js", gzHandler)
	}
	if _, err := os.Stat(gzPath("/client.js.map")); err == nil {
		http.HandleFunc("/client.js.map", gzHandler)
	}

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*port), nil))
}

type Artist struct {
	ID   string
	Name string
}

type Move struct {
	ID          string
	Description string

	retreived bool
}

var (
	artistsCount int32
	moveCount    int32
	artists      map[string]Artist
	moves        map[string][]Move
)

func ArtistsHandler(w http.ResponseWriter, r *http.Request) {

	var aa []Artist
	for _, a := range artists {
		aa = append(aa, a)
	}

	w.Header().Add("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(aa)
}

func CreateArtistsHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var artist Artist
	err := decoder.Decode(&artist)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := "artist" + strconv.Itoa(int(atomic.AddInt32(&artistsCount, 1)))

	artist.ID = id
	artists[id] = artist
	moves[id] = append(moves[id], Move{Description: "say " + artist.Name})

	w.Header().Add("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(artist)
}

func MovesHandler(w http.ResponseWriter, r *http.Request) {
	artistID := mux.Vars(r)["artistID"]

	w.Header().Add("Content-Type", "application/json")
	readyMoves, ok := moves[artistID]
	if !ok {
		readyMoves = []Move{}
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(readyMoves)

	delete(moves, artistID)
}

func CreateMoveHandler(w http.ResponseWriter, r *http.Request) {
	artistID := mux.Vars(r)["artistID"]
	_, ok := artists[artistID]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	moveID := "move" + strconv.Itoa(int(atomic.AddInt32(&moveCount, 1)))

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var move Move
	err := decoder.Decode(&move)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	move.ID = moveID
	moves[artistID] = append(moves[artistID], move)

	w.Header().Add("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(move)
}

func gzPath(path string) string {
	return staticDir + path + ".gz"
}

func gzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Content-Encoding", "gzip")
	http.ServeFile(w, r, gzPath(r.URL.Path))
	return
}
