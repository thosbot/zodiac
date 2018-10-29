$(document).ready(function() {
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
            // Build album coverart link from a song filepath (artist/album/song)
            albumCoverFilepath: function(filepath) {
                if ( ! filepath ) {
                    return '';
                }
                var f = filepath.split('/');
                var c = 'img/albumart/' + encodeURIComponent(f[0]) +
                    '__' + encodeURIComponent(f[1]) + '__cover' + '.png';
                return c;
            },

            // Convert seconds count to H:MM:SS
            seconds2HMS: function(sec) {
                // Double-tilde is double-bitwise-not, same as Math.floor()
                var h = ~~( sec / 3600 );
                var m = ~~(( sec % 3600 ) / 60 );
                var s = ~~sec % 60;

                var t = '';
                // Pad out zeros for display
                if ( h > 0 ) {
                    t = h + ':' + ( m < 10 ? '0' : '' );
                }
                t += m + ':' + ( s < 10 ? '0' : '' ) + s;
                return t;
            },

            seconds2Time: function(sec) {
                // Double-tilde is double-bitwise-not, same as Math.floor()
                var h = ~~( sec / 3600 );
                var m = ~~(( sec % 3600 ) / 60 );
                var s = ~~sec % 60;

                var t = '';
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
                $.get(apiURL + '/now-playing', (resp) => {
                    $('#now-playing').text(resp.Title);
                })
                .fail(function() { console.log('Error fetching current song'); });
            },

            loadAltAlbumCover: function(evt) {
                // event.target.src = "/img/albumart/album.jpg"
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
            play:  function() { $.post(apiURL + '/player/play') },
            pause: function() { $.post(apiURL + '/player/pause') },
            stop:  function() { $.post(apiURL + '/player/stop') },
            next:  function() { $.post(apiURL + '/player/next') },
            prev:  function() { $.post(apiURL + '/player/previous') },

            adjustVolume: function(d) {
                vol = parseInt(store.state.Status.volume) + d;
                $.post(apiURL + '/volume/' + vol);
            },

            // *******
            // Playlist controls
            //
            // *******

            // clearPlaylist removes all songs from the current playlist.
            clearPlaylist: function() { $.post(apiURL + '/playlist/clear') },

            // plSongDivId generates an element ID for playlist members.
            plSongDivID: function(id) { return 'pl-song-' + id; },

            // Play a song from playlist by playlist position
            playPos: function(pos) {
                $.post(apiURL + '/playlist/play/' + pos);
            },

            // Deletes song from playlist by position
            deletePos: function(pos) {
                scrollPos = $(window).scrollTop();
                $.post(apiURL + '/playlist/delete/' + pos);
            },

            togglePlaylistOptsDiv: function() {
                this.showPlaylistOpts = !this.showPlaylistOpts;
            },

            saveCurrentPlaylist: function(evt) {
                evt.preventDefault();
                var plName = document.getElementById('save-playlist-name').value;

                $.ajax({
                    url: apiURL + '/playlist/save',
                    type: 'POST',
                    data: JSON.stringify({ Name: plName }),
                    contentType: 'application/json; charset=utf-8',
                    dataType: 'json',
                    success: function(resp) {
                        this.resp.status = 'OK';
                        this.resp.msg = 'Success!';
                    }.bind(this),
                });
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
                // Use EC2015 arrow notation to get at the component's `this`.
                // TODO: bind(this)
                $.get(apiURL + '/list/' + listby, (resp) => {
                    this.artists = resp.List;
                })
                .fail(function() { console.log('Error fetching artist list'); });
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
                // Use EC2015 arrow notation to get at the component's `this`.
                // TODO: bind(this)
                $.get(apiURL + '/list/genre', (resp) => {
                    this.genres = resp.List;
                })
                .fail(function() { console.log('Error fetching genre list'); });
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
                var artist = this.$route.query.artist;
                var genre = this.$route.query.genre;

                if (artist && artist != "") {
                    artist = encodeURIComponent(artist);
                    $.get(apiURL + '/find/albums?artist=' + artist, (resp) => {
                        this.albums = resp.Albums;
                    })
                    .fail(function() { console.log('Error fetching album list'); });
                }
                else if (genre && genre != "") {
                    genre = encodeURIComponent(genre);
                    $.get(apiURL + '/find/albums?genre=' + genre, (resp) => {
                        this.albums = resp.Albums;
                    })
                    .fail(function() { console.log('Error fetching album list'); });
                }
                else {
                    $.get(apiURL + '/list/albums', (resp) => {
                        this.albums = resp.Albums;
                    })
                    .fail(function() { console.log('Error fetching album list'); });
                }
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
                var album = encodeURIComponent(this.$route.query.album);
                $.get(baseURL + '/find/songs?album=' + album, (resp) => {
                    this.album = resp;

                    // Set a various artist boolean
                    if (
                        this.album.Artist === "Various Artists" ||
                        this.album.Artist === "Various"
                    ) {
                        this.album.Various = true;
                    }
                })
                .fail(function() { console.log('Error fetching album list'); });
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
                loc = encodeURIComponent(loc);
                // Clear playlist
                $.post(apiURL + '/playlist/clear',
                    function(data) {
                        // Add the song or album to playlist
                        $.post(apiURL + '/playlist/add?loc=' + loc,
                            function(data) {
                                // Start playing
                                $.post(apiURL + '/playlist/play/-1',
                                    function(resp) {
                                        this.resp.status = 'OK';
                                        this.resp.msg = 'Done!';
                                    }.bind(this)
                                );
                            }.bind(this)
                        );
                    }.bind(this)
                );
            },
            queue: function(loc) {
                loc = encodeURIComponent(loc);
                $.post(apiURL + '/playlist/add?loc=' + loc, function(resp) {
                    this.resp.status = 'OK';
                    this.resp.msg = 'Queued!';
                }.bind(this));
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
                // Use EC2015 arrow notation to get at the component's `this`.
                // TODO: Bind this.
                $.get(apiURL + '/playlists', (resp) => {
                    this.playlists = resp.Playlists;
                })
                .fail(function() { console.log('Error fetching playlists'); });
            },
            play: function(name) {
                name = encodeURIComponent(name);
                $.post(apiURL + '/playlist/clear',
                    function(data) {
                        $.post(apiURL + '/playlist/load/' + name,
                            function(data) {
                                $.post(apiURL + '/playlist/play/-1');
                            }
                        );
                    }
                );
            },
            queue: function(name) {
                loc = encodeURIComponent(loc);
                $.post(apiURL + '/playlist/load/' + name);
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

    // Close the navbar on link clink
    $(document).on('click', '.navbar-collapse.in',function(e) {
        if( $(e.target).is('a:not(".dropdown-toggle")') ) {
            $(this).collapse('hide');
        }
    });

    // WebSocket connection
    openWebSocket();
    function openWebSocket() {
        var ws = new WebSocket(wsURL);
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
        var resp = JSON.parse(evt.data);

        // console.log(resp);
        // Update the Vuex datastore, with each message received.
        if ( resp.Status ) { store.commit('setStatus', resp.Status); }

        store.commit('setCurrentSong', resp.CurrentSong);
        if ( resp.CurrentSong && resp.CurrentSong.Title ) {
            document.title = resp.CurrentSong.Artist + ' - ' +
                resp.CurrentSong.Title;
            // Need to set navbar manually -- it's not currently in the app.
            $('#now-playing').html(
                '<strong>' + resp.CurrentSong.Title + '</strong> by ' +
                resp.CurrentSong.Artist
            );
        }
        else {
            store.commit('setCurrentSong', {})
            $('#now-playing').html('&mdash;');
            document.title = 'Juke';
        }

        // Update the current loaded playlist
        if (resp.Playlist) {
            store.commit( 'setPlaylist', resp.Playlist );
        }

        $(window).scrollTop(scrollPos);
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
