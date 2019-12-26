var ready = (callback) => {
  if (document.readyState != "loading") callback();
  else document.addEventListener("DOMContentLoaded", callback);
}

ready(() => {
    Vue.config.devtools = true;
    Vue.config.debug = true;

    var scrollPos = 0;

    const baseURL = 'http://localhost';
    const apiURL  = baseURL + ':3000';
    const wsURL   = 'ws://localhost:3000/ws';

    // Global Vuex datastore
    const store = new Vuex.Store({
        state: {
            Status: {},
            CurrentSong: { Title: '', Artist: '' },
            Playlist: [],
        },
        mutations: {
            setStatus (state, status) { state.Status = status },
            setCurrentSong (state, song) { state.CurrentSong = song; },
            setPlaylist (state, playlist) { state.Playlist = playlist },
        },
    });

    // Mixins
    var helpers = {
        methods: {
            // Build albumart link from a song filepath (artist/album/song)
            albumCoverFilepath: function(filepath) {
                if ( ! filepath ) {
                    return '/img/albumart/album.jpg';
                }
                let f = filepath.split('/');
                let c = 'img/albumart/' + encodeURIComponent(f[0]) +
                    '__' + encodeURIComponent(f[1]) + '__cover' + '.jpg';
                return c;
            },

            // Convert seconds count to H:MM:SS
            seconds2HMS: function(sec) {
                // Double-tilde is double-bitwise-not, same as Math.floor()
                let h = ~~( sec / 3600 );
                let m = ~~(( sec % 3600 ) / 60 );
                let s = ~~sec % 60;

                let t = '';
                // Pad out zeros for display
                if ( h > 0 ) {
                    t = h + ':' + ( m < 10 ? '0' : '' );
                }
                t += m + ':' + ( s < 10 ? '0' : '' ) + s;
                return t;
            },

            seconds2Time: function(sec) {
                // Double-tilde is double-bitwise-not, same as Math.floor()
                let h = ~~( sec / 3600 );
                let m = ~~(( sec % 3600 ) / 60 );
                let s = ~~sec % 60;

                let t = '';
                if ( h > 0 ) {
                    t = h + ' hour' + ( h > 1 ? 's' : '' ) + ', ';
                }
                t += m + ' minute' + ( m > 1 ? 's' : '' ) + ', ';
                t += s + ' second' + ( s > 1 ? 's' : '' );
                return t;
            },

            // The navbar currently lives outside the app, so we're updaing
            // the app so we're updating the old fashioned way.
            fetchNowPlaying: function() {
                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('GET', apiURL + '/now-playing', true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        let resp = JSON.parse(this.response);
                        document.getElementById('now-playing').textContent = resp.Title;
                    } else {
                        console.log('Error getting current song');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting current song'); };
                xhr.send();
            },

            loadAltAlbumCover: function(evt) {
                // Verify that the image exists on the server before we reset
                // the image URL.
                let altImgPath = '/img/albumart/album.jpg';

                let xhr = new XMLHttpRequest();
                xhr.open('GET', baseURL + altImgPath, true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        evt.target.src = altImgPath;
                    } else {
                        console.log('Error getting album cover');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting album cover'); };
                xhr.send();
            },

            getYear: function(date) {
                if ( date != null && date != "" ) {
                    return date.substr(0, 4);
                }
            },
        }
    }

    // Components
    var Home = {
        name: 'Home',
        template: '#home-template',
        data: function() {
            return {
                showPlaylistOpts: false,
                resp: { status: '', msg: '' },
            };
        },
        computed: {
            currentsong () { return store.state.CurrentSong; },
            playlist () { return store.state.Playlist },
            status () { return store.state.Status; },

            // Computed properties are cached based on their dependencies
            playliststats () {
                numSongs = store.state.Status.playlistlength;
                duration = 0;
                if (store.state.Playlist) {
                    store.state.Playlist.forEach(
                        function(el) { duration += parseInt(el.Time) }
                    );
                }
                return {
                    numSongs: numSongs,
                    duration: duration,
                };
            },
        },
        mixins: [ helpers ],
        methods: {
            // Player controls
            // NOTE: Updating the datastore is handled via messages received
            //       over the websocket.
            play:  function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/player/play', true);
                xhr.send();
            },
            pause: function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/player/pause', true);
                xhr.send();
            },
            stop:  function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/player/stop', true);
                xhr.send();
            },
            next:  function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/player/next', true);
                xhr.send();
            },
            prev:  function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/player/previous', true);
                xhr.send();
            },
            adjustVolume: function(d) {
                let xhr = new XMLHttpRequest();
                vol = parseInt(store.state.Status.volume) + d;
                xhr.open('POST', apiURL + '/volume/' + vol, true);
                xhr.send();
            },

            // *******
            // Playlist controls
            //
            // *******

            // clearPlaylist removes all songs from the current playlist.
            clearPlaylist: function() {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/clear', true);
                xhr.send();
            },

            // plSongDivId generates an element ID for playlist members.
            plSongDivID: function(id) { return 'pl-song-' + id; },

            // Play a song from playlist by playlist position
            playPos: function(pos) {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/play/' + pos, true);
                xhr.send();
            },

            // Deletes song from playlist by position
            deletePos: function(pos) {
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/delete/' + pos, true);
                xhr.send();
                // TODO: Vanilla JS
                // scrollPos = $(window).scrollTop();
            },

            togglePlaylistOptsDiv: function() {
                this.showPlaylistOpts = !this.showPlaylistOpts;
            },

            saveCurrentPlaylist: function(evt) {
                evt.preventDefault();

                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/save', true);
                xhr.setRequestHeader('Content-Type', 'application/json; charset=UTF-8');
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        self.resp.status = 'OK';
                        self.resp.msg = 'Success!';
                    } else {
                        console.log('Error saving playlist');
                    }
                };
                xhr.onerror = function(){ console.log('Error saving playlist'); };
                let plName = document.getElementById('save-playlist-name').value;
                xhr.send(JSON.stringify( {Name: plName} ));
            },
        },
    };

    var ArtistBrowser = {
        name: 'ArtistBrowser',
        template: '#artist-browser-template',
        data: function() {
            return { artists: [] };
        },
        created: function() { this.fetchArtists('albumartist'); },
        mixins: [ helpers ],
        methods: {
            fetchArtists: function(listby) {
                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('GET', apiURL + '/list/' + listby, true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        let resp = JSON.parse(this.response);
                        self.artists = resp.List;
                    } else {
                        console.log('Error getting artist list');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting artist list'); };
                xhr.send();
            },
        },
    };

    var GenreBrowser = {
        name: 'GenreBrowser',
        template: '#genre-browser-template',
        data: function() {
            return { genres: [] };
        },
        created: function() { this.fetchGenres(); },
        mixins: [ helpers ],
        methods: {
            fetchGenres: function(listby) {
                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('GET', apiURL + '/list/genre', true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        let resp = JSON.parse(this.response);
                        self.genres = resp.List;
                    } else {
                        console.log('Error getting genres');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting genres'); };
                xhr.send();
            },
        },
    };

    var AlbumBrowser = {
        name: 'AlbumBrowser',
        template: '#album-browser-template',
        data: function() {
            return { albums: [] };
        },
        created: function() { this.fetchAlbums(); },
        mixins: [ helpers ],
        methods: {
            fetchAlbums: function() {
                let self = this;
                let xhr = new XMLHttpRequest();

                let artist = this.$route.query.artist;
                let genre = this.$route.query.genre;

                if (artist && artist != "") {
                    artist = encodeURIComponent(artist);
                    xhr.open('GET', apiURL + '/find/albums?artist=' + artist, true);
                }
                else if (genre && genre != "") {
                    genre = encodeURIComponent(genre);
                    xhr.open('GET', apiURL + '/find/albums?genre=' + genre, true);
                }
                else {
                    xhr.open('GET', apiURL + '/list/albums', true);
                }

                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        let resp = JSON.parse(this.response);
                        self.albums = resp.Albums
                    } else {
                        console.log('Error getting album list');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting album list'); };
                xhr.send();
            },
        },
    };

    var SongBrowser = {
        name: 'SongBrowser',
        template: '#song-browser-template',
        data: function() {
            return {
                album: {},
                songs: [],
                optsToggle: '',
                resp: { status: '', msg: '' },
            };
        },
        created: function() { this.fetchSongs(); },
        mixins: [ helpers ],
        methods: {
            fetchSongs: function() {
                let params = [];
                if (this.$route.query.album != "") {
                    params.push("album=" + encodeURIComponent(this.$route.query.album));
                }
                if (this.$route.query.artist != "") {
                    params.push("param.albumartist" + encodeURIComponent(this.$route.query.artist));
                }

                let qstr = '';
                if (params.length > 0) {
                    qstr = "?" + params.join('&');
                }

                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('GET', apiURL + '/find/songs' + qstr, true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        self.album = JSON.parse(this.response);

                        if (
                            self.album.Artist === "Various Artists" ||
                            self.album.Artist === "Various"
                        ) {
                            self.album.Various = true;
                        }
                    } else {
                        console.log('Error finding songs');
                    }
                };
                xhr.onerror = function(){ console.log('Error finding songs'); };
                xhr.send();
            },

            // Show or hide options for song
            toggleOptsDiv: function(file) {
                if ( this.optsToggle === file ) {
                    this.optsToggle = '';
                } else {
                    this.optsToggle = file;
                }
            },

            // Play now
            play: function(loc) {
                let self = this;
                let xhr = new XMLHttpRequest();

                // Clear current playlist
                xhr.open('POST', apiURL + '/playlist/clear', true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error clearing playlist');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error clearing playlist');
                    return;
                };
                xhr.send();

                // Add the song or album to playlist
                xhr = new XMLHttpRequest();
                loc = encodeURIComponent(loc);
                xhr.open('POST', apiURL + '/playlist/add?loc=' + loc, true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error adding song(s) to playlist');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error adding song(s) to playlist');
                    return;
                };
                xhr.send();

                // Play
                xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/play/-1', true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error playing');
                        return;
                    }
                    self.resp.status = 'OK';
                    self.resp.msg = 'Done!';
                };
                xhr.onerror = function(){
                    console.log('Error playing');
                    return;
                };
                xhr.send();
            },

            // Add a song or album to bottom of queue
            queue: function(loc) {
                let self = this;
                let xhr = new XMLHttpRequest();

                loc = encodeURIComponent(loc);
                xhr.open('POST', apiURL + '/playlist/add?loc=' + loc, true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        self.resp.status = 'OK';
                        self.resp.msg = 'Queued!';
                    } else {
                        console.log('Error adding song to queue');
                    }
                };
                xhr.onerror = function(){ console.log('Error adding song to queue'); };
                xhr.send();
            },
        }
    };

    var PlaylistBrowser = {
        name: 'PlaylistBrowser',
        template: '#playlist-browser-template',
        data: function() {
            return { playlists: [] };
        },
        created: function() { this.fetchPlaylists(); },
        mixins: [ helpers ],
        methods: {
            fetchPlaylists: function(listby) {
                let self = this;

                let xhr = new XMLHttpRequest();
                xhr.open('GET', apiURL + '/playlists', true);
                xhr.onload = function() {
                    if (this.status >= 200 && this.status < 400) {
                        let resp = JSON.parse(this.response);
                        self.playlists = resp.Playlists;
                    } else {
                        console.log('Error getting playlists');
                    }
                };
                xhr.onerror = function(){ console.log('Error getting playlists'); };
                xhr.send();
            },
            play: function(loc) {
                // Clear current playlist
                let xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/clear', true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error clearing playlist');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error clearing playlist');
                    return;
                };
                xhr.send();

                // Load new playlist
                xhr = new XMLHttpRequest();
                loc = encodeURIComponent(loc);
                xhr.open('POST', apiURL + '/playlist/load/' + loc, true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error loading playlist');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error loading playlist');
                    return;
                };
                xhr.send();

                // Play
                xhr = new XMLHttpRequest();
                xhr.open('POST', apiURL + '/playlist/play/-1', true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error playing');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error playing');
                    return;
                };
                xhr.send();
            },
            queue: function(loc) {
                let xhr = new XMLHttpRequest();
                loc = encodeURIComponent(loc);
                xhr.open('POST', apiURL + '/playlist/load/' + loc, true);
                xhr.onload = function() {
                    if (this.status < 200 && this.status >= 400) {
                        console.log('Error loading playlist');
                        return;
                    }
                };
                xhr.onerror = function(){
                    console.log('Error loading playlist');
                    return;
                };
                xhr.send();
            },
        },
    };

    var router = new VueRouter({
        mode: 'hash',
        base: window.location.href,
        routes: [
            { path: '/', name: 'home', component: Home },
            { path: '/browse/albums', name: 'albums', component: AlbumBrowser },
            { path: '/browse/artists', name: 'artists', component: ArtistBrowser },
            { path: '/browse/songs', name: 'songs', component: SongBrowser },
            { path: '/browse/genres', name: 'genres', component: GenreBrowser },
            { path: '/browse/playlists', name: 'playlists', component: PlaylistBrowser },
        ],
    });

    // TODO: Vanilla JS
    // Close the navbar on link clink
    // $(document).on('click', '.navbar-collapse.in',function(e) {
    //     if( $(e.target).is('a:not(".dropdown-toggle")') ) {
    //         $(this).collapse('hide');
    //     }
    // });

    // WebSocket connection
    openWebSocket();
    function openWebSocket() {
        let ws = new WebSocket(wsURL);
        ws.onopen = function(evt) { onOpen(evt); };
        ws.onmessage = function(evt) { onMessage(evt); };
        ws.onclose = function(){
            // Try to reconnect
            setTimeout(function(){ openWebSocket() }, 2500);
        };
    }
    function onOpen(evt) {
        // console.log('WebSocket connection open');
    }
    function onMessage(evt) {
        let resp = JSON.parse(evt.data);

        // Update the Vuex datastore, with each message received.
        if ( resp.Status ) { store.commit('setStatus', resp.Status); }

        store.commit('setCurrentSong', resp.CurrentSong);
        if ( resp.CurrentSong && resp.CurrentSong.Title ) {
            document.title = resp.CurrentSong.Artist + ' - ' +
                resp.CurrentSong.Title;
            // Need to set navbar manually -- it's not currently in the app.
            document.getElementById('now-playing').innerHTML =
                '<strong>' + resp.CurrentSong.Title + '</strong> by ' + resp.CurrentSong.Artist;
        }
        else {
            store.commit('setCurrentSong', {})
            document.getElementById('now-playing').innerHTML = '&mdash;'
            document.title = 'Juke';
        }

        // Update the current loaded playlist
        if (resp.Playlist) {
            store.commit('setPlaylist', resp.Playlist);
        }

        // TODO: Vanilla JS
        // $(window).scrollTop(scrollPos);
        scrollPos = 0;
    }

    var Juke = new Vue({
        el: '#juke-app',
        router: router,
        store: store,
        data: {},
        computed: {},
        methods: {}
    });
});
