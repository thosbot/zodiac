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

	r.HandleFunc("/list/albums", listAlbums).Methods(http.MethodGet)
	r.HandleFunc("/list/{type}", list).Methods(http.MethodGet)

	r.HandleFunc("/find/albums", findAlbums).Methods(http.MethodGet)
	r.HandleFunc("/find/songs", findSongs).Methods(http.MethodGet)

	r.HandleFunc("/playlists", listPlaylists).Methods(http.MethodGet)
	r.HandleFunc("/playlist/load/{name}", loadPlaylist).Methods(http.MethodPost)
	r.HandleFunc("/playlist/save", savePlaylist).Methods(http.MethodPost)
	r.HandleFunc("/playlist/clear", clearPlaylist).Methods(http.MethodPost)

	r.HandleFunc("/playlist/add", addLoc).Methods(http.MethodPost)
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
	Playlist    []mpd.Attrs
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
	resp.Playlist = q

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

	err = wsconn.WriteMessage(1, b)
	if err != nil {
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
	returnOK(w)
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
	for _, li := range list {
		rec := strings.Split(li, ": ")
		resp["List"] = append(resp["List"], rec[1])
	}

	b, err := json.Marshal(resp)
	if err != nil {
		log.Println(errors.Wrap(err, "json marshal"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, string(b))
}

type Album struct {
	Title  string
	Artist string
	Date   string
}

func listAlbums(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	list, err := mpdconn.List("album", "group", "albumartist", "group", "date")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd list"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type Resp struct {
		Albums []Album
	}
	resp := Resp{}

	// Iterate over the slice of "key: values" returned. Each album will begin
	// with the "Album" key.
	album := Album{}
	for _, rec := range list {
		// Get the key/val by splitting the string on the first colon found.
		// FIXME: You're sunk if there's a colon in the band name.
		key, val := "", ""
		i := strings.Index(rec, ": ")
		if i > 0 {
			key = rec[:i]
			val = rec[i+2:]
		} else {
			log.Println(fmt.Errorf("list albums: cannot parse %s", rec))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// We'll know to start a new album object when we receive but have
		// already captured an album title.
		if key == "Album" && album.Title != "" {
			resp.Albums = append(resp.Albums, album)
			// Clear out the struct
			album = Album{}
			continue
		}

		// Write the value to its correct struct position
		if key == "Album" {
			album.Title = val
		} else if key == "AlbumArtist" {
			album.Artist = val
		} else if key == "Date" {
			album.Date = val
		} else {
			log.Println(fmt.Errorf("list albums: unhandled key (%s)", key))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Return results
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
		if found[s["Artist"]+"--"+s["Album"]] {
			continue
		}
		a := Album{
			Title:  s["Album"],
			Artist: s["Artist"],
			Date:   s["Date"],
		}
		resp.Albums = append(resp.Albums, a)
		found[s["Artist"]+"--"+s["Album"]] = true
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

	args := []string{}

	album := r.FormValue("album")
	if album != "" {
		args = append(args, "album", album)
	}
	albumartist := r.FormValue("albumartist")
	if albumartist != "" {
		args = append(args, "albumartist", albumartist)
	}

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

	list, err := mpdconn.Find(args...)
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd list"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(list) == 0 {
		log.Printf("couldn't find album '%s'", album)
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
	returnOK(w)
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
	returnOK(w)
}

func playPos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	pos, err := strconv.Atoi(vars["pos"])
	if err != nil {
		log.Println(errors.Wrapf(err, "invalid position '%s'", vars["pos"]))
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
	returnOK(w)
}

func deletePos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	pos, err := strconv.Atoi(vars["pos"])
	if err != nil {
		log.Println(errors.Wrapf(err, "invalid position '%s'", vars["pos"]))
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
	returnOK(w)
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
	returnOK(w)
}

func listPlaylists(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	playlists, err := mpdconn.ListPlaylists()
	if err != nil {
		log.Println(errors.Wrapf(err, "load playlist"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type Playlist struct {
		Name  string
		Songs []mpd.Attrs
	}
	type Resp struct {
		Playlists []Playlist
	}
	resp := Resp{}
	for _, p := range playlists {
		attrs, _ := mpdconn.PlaylistContents(p["playlist"])
		pl := Playlist{
			Name:  p["playlist"],
			Songs: attrs,
		}
		resp.Playlists = append(resp.Playlists, pl)
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

func loadPlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)

	// Connect to MPD server
	mpdconn, err := mpd.Dial("tcp", "localhost:6600")
	if err != nil {
		log.Println(errors.Wrapf(err, "mpd dial"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer mpdconn.Close()

	// Load playlist
	err = mpdconn.PlaylistLoad(vars["name"], -1, -1)
	if err != nil {
		log.Println(errors.Wrapf(err, "load playlist"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	returnOK(w)
}

func savePlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type Playlist struct {
		Name string
	}
	pl := Playlist{}

	err := json.NewDecoder(r.Body).Decode(&pl)
	if err != nil {
		log.Println(errors.Wrapf(err, "save playlist"))
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

	err = mpdconn.PlaylistSave(pl.Name)
	if err != nil {
		log.Println(errors.Wrapf(err, "save playlist"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	returnOK(w)
}

// returnOK prints a `{status: "OK"}` JSON message to the response writer.
func returnOK(w http.ResponseWriter) {
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
