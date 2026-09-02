.. index::
   single: artists

Artists
=======

Get artists list
----------------

Load list of artists

::

  GET {URL}/api/v1/artists?mode={MODE}&limit={LIMIT}&page={?PAGE}&searchQuery={?SEARCH_QUERY}

* MODE - one of the following:

  * ALL - no modifier, all artists
  * RANDOM - random selection of artists

* LIMIT - maximum number of items to load
* PAGE - (optional) items offset
* SEARCH_QUERY - (optional) filter by all text fields

Example request:

::

  GET http://localhost:8080/api/v1/artists?mode=RANDOM&limit=50

::

  {
      "status": 200,
      "message": "ok",
      "items": [
          {
              "id": 10,
              "name": "Akira Yamaoka",
              "images": []
          },
          {
              "id": 80,
              "name": "Gojira",
              "logo": {
                  "160": "/api/v1/images/160/167_00_b8ed457b10409bfb.jpg",
                  "320": "/api/v1/images/320/167_00_b8ed457b10409bfb.jpg",
                  "640": "/api/v1/images/640/167_00_b8ed457b10409bfb.jpg"
              },
              "images": [
                  {
                      "url": "/api/v1/images/640/167_00_b8ed457b10409bfb.jpg",
                      "type": "artist-logo",
                      "order": 0
                  }
              ]
          }
          .
          .
          .
      ],
      "next": true
  }

Get single artist
-----------------

Load single artist by id

::

  GET {URL}/api/v1/artists/{ARTIST_ID}

Example request:

::

  GET http://localhost:8080/api/v1/artists/80

::

  {
      "status": 200,
      "message": "ok",
      "id": 80,
      "name": "Gojira",
      "logo": {
          "160": "/api/v1/images/160/167_00_b8ed457b10409bfb.jpg",
          "320": "/api/v1/images/320/167_00_b8ed457b10409bfb.jpg",
          "640": "/api/v1/images/640/167_00_b8ed457b10409bfb.jpg"
      },
      "images": [
          {
              "url": "/api/v1/images/640/167_00_b8ed457b10409bfb.jpg",
              "type": "artist-logo",
              "order": 0
          }
      ],
      "related": [
          {
              "id": 208,
              "name": "Empalot",
              "images": null
          }
      ]
  }
