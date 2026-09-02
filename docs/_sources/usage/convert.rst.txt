.. index::
   single: convert

Convert
=======

Convert tracks works with two stages - convert and download

Start convert
-------------

Convert tracks and store them into temporary files

::

  POST {URL}/api/v1/convert/start
  {
      userId: "{WEBSOCKET_USER_ID}",
      track_ids: [{TRACK_IDS}],
      format: "{FORMAT}",
      mode: "{MODE}",
      quality: "{QUALITY}",
      include_pregap: {PREGAP}
  }

Convert parameters JSON:

* userId - ID of websocket user for receive convert info and notifications
* track_ids - array of track IDs
* format - which format convert to
* mode - for mp3 is **cbr** or **vbr**
* quality - bitrate of converted result
* include_pregap - for .cue tracks it is possible to include pregap data if it's exists

Example request:

::

  POST http://localhost:8080/api/v1/convert/start
  {
      userId: "a09f59ef-0764-4dd7-9e32-7b769bd0e521",
      track_ids: [99, 7323],
      format: "mp3",
      mode: "cbr",
      quality: "320",
      include_pregap: false
  }

Method returns ID of job, which was created for convert operation. If convert track is initially .mp3, it will be returned as blob to download immediately

::

  {
      "status": 200,
      "message": "ok",
      "job_id": 2453
  }

Download result
---------------

Download converted track or tracks

::

  POST {URL}/api/v1/convert/download/{JOB_ID}

Example request:

::

  POST http://localhost:8080/api/v1/convert/download/2453

Method returns blob data to download. It will be either .mp3 for single track or .zip archive for multiple tracks.
