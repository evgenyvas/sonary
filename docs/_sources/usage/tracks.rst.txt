.. index::
   single: tracks

Tracks
======

Get tracks list
---------------

Load list of tracks

::

  GET {URL}/api/v1/tracks?mode={MODE}&limit={LIMIT}&page={?PAGE}&artistId={?ARTIST_ID}&searchQuery={?SEARCH_QUERY}

* MODE - one of the following:

  * ALL - no modifier, all tracks
  * RANDOM - random selection of tracks
  * FAVORITES - tracks, selected as favorites
  * NOALBUM - tracks which artist is not equals to album artist

* LIMIT - maximum number of items to load
* PAGE - (optional) items offset
* ARTIST_ID - (optional) filter by artist
* SEARCH_QUERY - (optional) filter by all text fields

Example request:

::

  GET http://localhost:8080/api/v1/tracks?mode=RANDOM&limit=50

::

  {
      "status": 200,
      "message": "ok",
      "items": [
          {
              "id": 99,
              "type": "FLAC",
              "title": "Polyphony",
              "artist": "Strapping Young Lad",
              "artist_id": 17,
              "albumArtist": "Strapping Young Lad",
              "year": 2006,
              "genre": "Thrash / Death / Heavy / Industrial Metal",
              "album": "The New Black",
              "album_id": 14,
              "cover": {
                  "160": "/api/v1/images/160/120_01_072bb61c6265f4cd.jpg",
                  "320": "/api/v1/images/320/120_01_072bb61c6265f4cd.jpg",
                  "640": "/api/v1/images/640/120_01_072bb61c6265f4cd.jpg"
              },
              "number": 10,
              "duration": 114,
              "pregap": false,
              "pregap_duration": 0,
              "lyrics": "",
              "like": true
          },
          {
              "id": 7323,
              "type": "OGG",
              "title": "Where Cash Is King (Radio, Full)",
              "artist": "Stephen Yerkey",
              "artist_id": 694,
              "albumArtist": "VA",
              "year": 0,
              "genre": "Soundtrack",
              "album": "Silent Hill Downpour [xbox360-rip] (Licensed Music)",
              "album_id": 1022,
              "number": 11,
              "duration": 144,
              "pregap": false,
              "pregap_duration": 0,
              "lyrics": "",
              "like": false
          },
          .
          .
          .
      ],
      "next": true
  }

Get single track
----------------

Load single track by id

::

  GET {URL}/api/v1/tracks/{TRACK_ID}

Example request:

::

  GET http://localhost:8080/api/v1/tracks/5123

Example output:

::

  {
      "status": 200,
      "message": "ok",
      "id": 5123,
      "type": "FLAC",
      "title": "The Game",
      "artist": "Pain",
      "artist_id": 160,
      "albumArtist": "Pain",
      "year": 2002,
      "genre": "Industrial Metal",
      "album": "Nothing Remains The Same",
      "album_id": 713,
      "cover": {
          "160": "/api/v1/images/160/191_01_072bb61c6265f4cd.jpg",
          "320": "/api/v1/images/320/191_01_072bb61c6265f4cd.jpg",
          "640": "/api/v1/images/640/191_01_072bb61c6265f4cd.jpg"
      },
      "number": 10,
      "duration": 244,
      "pregap": true,
      "pregap_duration": 1,
      "lyrics": "",
      "like": false
  }

Update single track
-------------------

::

  PUT {URL}/api/v1/tracks/{TRACK_ID}
  {
    "like": true
  }

Example request:

::

  PUT http://localhost:8080/api/v1/tracks/5123

And JSON body:

::

  {
    "like": true
  }

For now the only field available to change is **like**

Example output:

::

  {
      "status": 200,
      "message": "ok",
      "id": 5123,
      "type": "FLAC",
      "title": "The Game",
      "artist": "Pain",
      "artist_id": 160,
      "albumArtist": "Pain",
      "year": 2002,
      "genre": "Industrial Metal",
      "album": "Nothing Remains The Same",
      "album_id": 713,
      "cover": {
          "160": "/api/v1/images/160/191_01_072bb61c6265f4cd.jpg",
          "320": "/api/v1/images/320/191_01_072bb61c6265f4cd.jpg",
          "640": "/api/v1/images/640/191_01_072bb61c6265f4cd.jpg"
      },
      "number": 10,
      "duration": 244,
      "pregap": true,
      "pregap_duration": 1,
      "lyrics": "",
      "like": true
  }

Method returns track data with changed fields

Update single track
-------------------

::

  PUT {URL}/api/v1/tracks/{TRACK_ID}
  {
    "like": true
  }

Example request:

::

  PUT http://localhost:8080/api/v1/tracks/5123

And JSON body:

::

  {
    "like": true
  }

For now the only field available to change is **like**

Example output:

::

  {
      "status": 200,
      "message": "ok",
      "id": 5123,
      "type": "FLAC",
      "title": "The Game",
      "artist": "Pain",
      "artist_id": 160,
      "albumArtist": "Pain",
      "year": 2002,
      "genre": "Industrial Metal",
      "album": "Nothing Remains The Same",
      "album_id": 713,
      "cover": {
          "160": "/api/v1/images/160/191_01_072bb61c6265f4cd.jpg",
          "320": "/api/v1/images/320/191_01_072bb61c6265f4cd.jpg",
          "640": "/api/v1/images/640/191_01_072bb61c6265f4cd.jpg"
      },
      "number": 10,
      "duration": 244,
      "pregap": true,
      "pregap_duration": 1,
      "lyrics": "",
      "like": true
  }

Method returns track data with changed fields
