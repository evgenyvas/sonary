.. index::
   single: albums

Albums
======

Get albums list
---------------

Load list of albums

::

  GET {URL}/api/v1/albums?mode={MODE}&limit={LIMIT}&page={?PAGE}&artistId={?ARTIST_ID}&searchQuery={?SEARCH_QUERY}

* MODE - one of the following:

  * ALL - no modifier, all artists
  * RANDOM - random selection of artists

* LIMIT - maximum number of items to load
* PAGE - (optional) items offset
* ARTIST_ID - (optional) filter by artist
* SEARCH_QUERY - (optional) filter by all text fields

Example request:

::

  GET http://localhost:8080/api/v1/albums?mode=RANDOM&limit=50

::

  {
      "status": 200,
      "message": "ok",
      "items": [
          {
              "id": 1078,
              "artist": "Veil Of Maya",
              "artist_id": 52,
              "title": "Outrun",
              "year": 2021,
              "cover": {
                  "160": "/api/v1/images/160/842_01_072bb61c6265f4cd.jpg",
                  "320": "/api/v1/images/320/842_01_072bb61c6265f4cd.jpg",
                  "640": "/api/v1/images/640/842_01_072bb61c6265f4cd.jpg"
              },
              "images": [
                  {
                      "url": "/api/v1/images/640/842_02_bae07a66b17c3a08.jpg",
                      "type": "front",
                      "order": 0
                  },
                  {
                      "url": "/api/v1/images/640/842_02_fbd2c3f8801c3f9c.jpg",
                      "type": "front",
                      "order": 1
                  },
                  {
                      "url": "/api/v1/images/640/842_01_072bb61c6265f4cd.jpg",
                      "type": "main-front",
                      "order": 2
                  }
              ]
          },
          {
              "id": 831,
              "artist": "Metallica",
              "artist_id": 167,
              "artist_logo": {
                  "160": "/api/v1/images/160/1584_00_f06cd9caf881914a.jpg",
                  "320": "/api/v1/images/320/1584_00_f06cd9caf881914a.jpg",
                  "640": "/api/v1/images/640/1584_00_f06cd9caf881914a.jpg"
              },
              "title": "Sad But True",
              "year": 1993,
              "images": null
          },
          .
          .
          .
      ],
      "next": true
  }

Get single album
----------------

Load single album with tracks by id

::

  GET {URL}/api/v1/albums/{ALBUM_ID}

Example request:

::

  GET http://localhost:8080/api/v1/albums/831

::

  {
      "status": 200,
      "message": "ok",
      "id": 831,
      "artist": "Metallica",
      "artist_id": 167,
      "artist_logo": {
          "160": "/api/v1/images/160/1584_00_7d744989f3671f77.jpg",
          "320": "/api/v1/images/320/1584_00_7d744989f3671f77.jpg",
          "640": "/api/v1/images/640/1584_00_7d744989f3671f77.jpg"
      },
      "title": "Sad But True",
      "year": 1993,
      "images": [],
      "tracks": [
          {
              "id": 5772,
              "type": "FLAC",
              "title": "Nothing Else Matters (Elevator Version)",
              "artist": "Metallica",
              "artist_id": 167,
              "albumArtist": "Metallica",
              "year": 1993,
              "genre": "Thrash Metal",
              "album": "Sad But True",
              "album_id": 831,
              "number": 2,
              "duration": 391,
              "pregap": false,
              "pregap_duration": 0,
              "lyrics": "",
              "like": false
          }
      ]
  }
