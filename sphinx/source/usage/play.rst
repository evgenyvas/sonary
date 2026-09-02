.. index::
   single: play

Play
====

Sonary allows to play tracks in local music player. For now only foobar2000 is supported.

Play in foobar2000
------------------

::

  POST {URL}/api/v1/play
  {
      "track_ids": [{TRACK_IDS}]
  }

track_ids - array of track IDs

Example request:

::

  POST http://localhost:8080/api/v1/play
  {
      track_ids: [99, 7323]
  }

Method returns empty response with status 200.
