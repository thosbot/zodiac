package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fhs/gompd/mpd"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/cors"
)

const BaseURL = "127.0.0.1:3000"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// FIXME: This should examine the request and only allow cross-origin over
	//        localhost port 3000.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	// XXX: Not sure that CORS is actually working -- we're currently just
	//      overriding via CheckOrigin.
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost", "http://localhost:3000",
			"http://127.0.0.1", "http://127.0.0.1:3000",
			"http://10.0.0.2", "http://10.0.0.2:3000",
		},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost},
		AllowCredentials: true,
	})

	r := mux.NewRouter()
	r.HandleFunc("/dashboard", dashboard).Methods(http.MethodGet)
	r.HandleFunc("/now-playing", nowPlaying).Methods(http.MethodGet)
	r.HandleFunc("/ws", playerWs).Methods(http.MethodGet)

	r.HandleFunc("/player/{action}", playerAction).Methods(http.MethodPost)

	r.HandleFunc("/list/{type}", list).Methods(http.MethodGet)
	r.HandleFunc("/find/albums", findAlbums).Methods(http.MethodGet)
	r.HandleFunc("/find/songs", findSongs).Methods(http.MethodGet)

	r.HandleFunc("/playlist/add", addLoc).Methods(http.MethodPost)
	r.HandleFunc("/playlist/clear", clearPlaylist).Methods(http.MethodPost)
	r.HandleFunc("/playlist/play/{pos}", playPos).Methods(http.MethodPost)
	r.HandleFunc("/playlist/delete/{pos}", deletePos).Methods(http.MethodPost)

	r.HandleFunc("/volume/{vol}", setVol).Methods(http.MethodPost)

	srv := &http.Server{
		Addr:         BaseURL,
		WriteTimeout: time.Second * 15, // Set timeouts to avoid Slowloris attacks
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      c.Handler(r), // Pass our instance of gorilla/mux into CORS
	}
	srv.ListenAndServe()
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp, err := getStatus()
	if err != nil {
		log.Println(errors.Wrap(err, "dashboard resp"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

type Status struct {
	Status      mpd.Attrs
	CurrentSong mpd.Attrs
	Queue       []mpd.Attrs
}

func getStatus() (*Status, error) {
	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		return nil, err
	}
	defer mpdconn.Close()

	status, err := mpdconn.Status()
	if err != nil {
		return nil, err
	}
	resp := Status{
		Status: status,
	}

	if status["state"] == "play" || status["state"] == "pause" {
		song, err := mpdconn.CurrentSong()
		if err != nil {
			return nil, err
		}
		resp.CurrentSong = song
	}

	q, err := mpdconn.PlaylistInfo(-1, -1)
	if err != nil {
		return nil, err
	}
	resp.Queue = q

	return &resp, nil
}

func nowPlaying(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	status, err := mpdconn.Status()
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd status"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type NowPlaying struct {
		CurrentSong mpd.Attrs
	}
	resp := NowPlaying{}

	if status["state"] == "play" || status["state"] == "pause" {
		song, err := mpdconn.CurrentSong()
		if err != nil {
			log.Println(errors.Wrapf(err, "mpd currentsong"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.CurrentSong = song
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func playerWs(w http.ResponseWriter, r *http.Request) {
	wsconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(errors.Wrap(err, "ws upgrade"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer wsconn.Close()

	mpdwatcher, err := mpd.NewWatcher("tcp", ":6600", "")
	if err != nil {
		log.Println(errors.Wrap(err, "mpd watcher"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdwatcher.Close()

	// Send the current status
	err = writeStatusToWs(wsconn)
	if err != nil {
		log.Println(errors.Wrap(err, "zodiac writestatus"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Report on MPD events
	for _ = range mpdwatcher.Event {
		// log.Println("Changed subsystem:", subsys)
		err := writeStatusToWs(wsconn)
		if err != nil {
			log.Println(errors.Wrap(err, "zodiac writestatus"))
			w.WriteHeader(http.StatusInternalServerError)
			break
		}
	}
	log.Println("Closing socket")
}

func writeStatusToWs(wsconn *websocket.Conn) error {
	resp, err := getStatus()
	if err != nil {
		return err
	}

	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	if err = wsconn.WriteMessage(1, b); err != nil {
		return err
	}

	return nil
}

func playerAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	action := vars["action"]

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	switch action {
	case "play":
		err = mpdconn.Play(-1)
	case "pause":
		s, err := mpdconn.Status()
		if err != nil {
			log.Println(errors.Wrapf(err, "mpd status"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Flip current pause state
		if s["state"] == "pause" {
			err = mpdconn.Pause(false)
		} else if s["state"] == "play" {
			err = mpdconn.Pause(true)
		}
	case "stop":
		err = mpdconn.Stop()
	case "next":
		err = mpdconn.Next()
	case "previous":
		err = mpdconn.Previous()
	}
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd %s", action))
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO: Check vars type
	vars := mux.Vars(r)

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	list, err := mpdconn.List(vars["type"])
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd list"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := make(map[string][]string)
	resp["List"] = list

	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func findAlbums(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Build list criteria from query params
	criteria := []string{}
	if artist := r.FormValue("artist"); artist != "" {
		criteria = append(criteria, "artist", artist)
	}
	if genre := r.FormValue("genre"); genre != "" {
		criteria = append(criteria, "genre", genre)
	}

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	type Album struct {
		Title  string
		Artist string
		Date   string
	}
	type Resp struct {
		Albums []Album
	}
	resp := Resp{}

	// Find returns a list of songs
	list, err := mpdconn.Find(criteria...)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd list"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Iterate over the song list and pull out the distinct albums
	found := map[string]bool{}
	for _, s := range list {
		// Skip to the next song if we've already caught the album details.
		if found[s["Album"]] {
			continue
		}
		a := Album{
			Title:  s["Album"],
			Artist: s["Artist"],
			Date:   s["Date"],
		}
		resp.Albums = append(resp.Albums, a)
		found[s["Album"]] = true
	}

	// Send response
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func findSongs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	album := r.FormValue("album")
	// FIXME: Will prob need album or album artist, but what to do with various
	//        artists?

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	type Resp struct {
		Title  string
		Artist string
		Dir    string
		Songs  []mpd.Attrs
	}

	list, err := mpdconn.Find("album", album)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd list"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get album info from the first song
	dir := strings.Split(list[0]["file"], "/")
	resp := Resp{
		Title:  list[0]["Album"],
		Artist: list[0]["AlbumArtist"],
		Dir:    fmt.Sprintf("%s/%s", dir[0], dir[1]),
		Songs:  list,
	}
	if resp.Artist == "" {
		resp.Artist = list[0]["Artist"]
	}

	// Send response
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func addLoc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	loc := r.FormValue("loc")
	err = mpdconn.Add(loc)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd add %s", loc))
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func clearPlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	err = mpdconn.Clear()
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd playlist clear"))
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func playPos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	pos, err := strconv.Atoi(vars["pos"])
	if err != nil {
		log.Println(errors.Wrapf(err, "invalid pos"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	err = mpdconn.Play(pos)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd play"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func deletePos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	pos, err := strconv.Atoi(vars["pos"])
	if err != nil {
		log.Println(errors.Wrapf(err, "invalid pos"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	err = mpdconn.Delete(pos, -1)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd delete"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

func setVol(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	// Get volume param
	vars := mux.Vars(r)
	vol, err := strconv.Atoi(vars["vol"])
	if err != nil {
		log.Println(errors.Wrapf(err, "invalid volume: '%s'", vars["vol"]))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Keep the volume in the allowable range (0-100)
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}

	// Set volume
	err = mpdconn.SetVolume(vol)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd volume"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write out an OK JSON response
	resp := make(map[string]string)
	resp["status"] = "OK"
	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}
