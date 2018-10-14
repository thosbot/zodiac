$(document).ready( function() {
    Vue.config.devtools = true;
    Vue.config.debug = true;

    var scrollPos = 0;
    var baseURL = 'http://localhost:3000';
    var wsURL = 'ws://localhost:3000/ws';

    // Global Vuex datastore
    var store = new Vuex.Store({
        state: {
            Status: {},
            CurrentSong: { Title: '', Artist: '' },
            Queue: [],
        },
        mutations: {
            setStatus (state, status) { state.Status = status },
            setCurrentSong (state, song) { state.CurrentSong = song; },
            setQueue (state, queue) { state.Queue = queue },
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
            seconds2Time: function(sec) {
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

            // The navbar currently lives outside the app, so we're updaing
            // the app so we're updating the old fashioned way.
            fetchNowPlaying: function() {
                $.get(baseURL + '/now-playing', (resp) => {
                    $('#now-playing').text(resp.Title);
                })
                .fail(function() { console.log('Error fetching current song'); });
            },

            loadAltAlbumCover: function(evt) {
                event.target.parentNode.style.display = 'none';
                // Add 0.5em padding to the album cover
                // TODO: event.target.src = "working-image.jpg" // imgUrl
            },
        }
    }

    // Components
    var Home = {
        name: 'Home',
        template: '#home-template',
        computed: {
            currentsong () { return store.state.CurrentSong; },
            queue () { return store.state.Queue },
            status () { return store.state.Status; },
        },
        mixins: [ helpers ],
        methods: {
            // Player controls
            // NOTE: Updating the datastore is handled via messages received
            //       over the websocket.
            play:  function() { $.post(baseURL + '/player/play') },
            pause: function() { $.post(baseURL + '/player/pause') },
            stop:  function() { $.post(baseURL + '/player/stop') },
            next:  function() { $.post(baseURL + '/player/next') },
            prev:  function() { $.post(baseURL + '/player/previous') },
            clearPlaylist: function() { $.post(baseURL + '/playlist/clear') },

            // Playlist controls

            // Generate an ID for playlist members
            plSongDivID: function(id) { return 'pl-song-' + id; },

            // Play a song from playlist by playlist position
            playPos: function(pos) {
                $.post(baseURL + '/playlist/play/' + pos);
            },

            // Deletes song from playlist by position
            deletePos: function(pos) {
                scrollPos = $(window).scrollTop();
                $.post(baseURL + '/playlist/delete/' + pos);
            }
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
                $.get(baseURL + '/list/' + listby, (resp) => {
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
                $.get(baseURL + '/list/genre', (resp) => {
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
                    $.get(baseURL + '/find/albums?artist=' + artist, (resp) => {
                        this.albums = resp.Albums;
                    })
                    .fail(function() { console.log('Error fetching album list'); });
                }
                else if (genre && genre != "") {
                    genre = encodeURIComponent(genre);
                    $.get(baseURL + '/find/albums?genre=' + genre, (resp) => {
                        this.albums = resp.Albums;
                    })
                    .fail(function() { console.log('Error fetching album list'); });
                }
                else {
                    $.get(baseURL + '/list/album', (resp) => {
                        var albums = [];
                        resp.List.forEach(
                            function(el) { albums.push({Title: el}); }
                        )
                        this.albums = albums;
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
            return { album: {}, songs: [], optsToggle: '' };
        },
        created: function() { this.fetchSongs(); },
        mixins: [ helpers ],
        methods: {
            fetchSongs: function() {
                var album = encodeURIComponent(this.$route.query.album);
                $.get(baseURL + '/find/songs?album=' + album, (resp) => {
                    this.album = resp;
                    if (
                        this.album.Artist === "Various Artists" ||
                        this.album.Artist === "Various"
                    ) {
                        this.album.Various = true;
                    }
                })
                .fail(function() { console.log('Error fetching album list'); });
            },
            toggleOptsDiv: function(file) {
                if ( this.optsToggle === file ) {
                    this.optsToggle = '';
                } else {
                    this.optsToggle = file;
                }
            },
            play: function(loc) {
                loc = encodeURIComponent(loc);
                $.post(baseURL + '/playlist/clear',
                    function(data) {
                        $.post(baseURL + '/playlist/add?loc=' + loc,
                            function(data) {
                                $.post(baseURL + '/playlist/play/-1');
                            }
                        );
                    }
                );
            },
            queue: function(loc) {
                loc = encodeURIComponent(loc);
                $.post(baseURL + '/playlist/add?loc=' + loc);
            },
        }
    };

    var router = new VueRouter( {
        mode: 'hash',
        base: window.location.href,
        routes: [
            { path: '/', name: 'home', component: Home },
            { path: '/browse/albums', name: 'albums', component: AlbumBrowser },
            { path: '/browse/artists', name: 'artists', component: ArtistBrowser },
            { path: '/browse/songs', name: 'songs', component: SongBrowser },
            { path: '/browse/genres', name: 'genres', component: GenreBrowser },
        ],
    } );

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

        // Update the current loaded queue / playlist
        if (resp.Queue) {
            store.commit( 'setQueue', resp.Queue );
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
} );
