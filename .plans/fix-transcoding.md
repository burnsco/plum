You are reviewing and improving the media playback pipeline of a Jellyfin-style streaming server.

Currently the server is **transcoding every file**, which is incorrect behavior. The goal is to implement **smart playback mode selection** so transcoding is only used when necessary.

Implement the following playback decision priority:

1. **Direct Play (highest priority)**
   If the client supports the original:

   * container format
   * video codec
   * audio codec
   * subtitle format
   * bitrate

   then stream the file **without modification**.
   The server should serve the original file directly.

2. **Remux (container conversion only)**
   If the client supports the **video and audio codecs**, but **not the container**, then:

   * remux the file into a compatible container
   * do **not re-encode video or audio**

   Example:
   MKV → MP4 while keeping H264 + AAC unchanged.

3. **Direct Stream / Audio Transcode**
   If the client supports the **video codec** but **not the audio codec**, then:

   * copy the video stream (`-c:v copy`)
   * transcode only the audio (`-c:a aac` or other compatible codec)

4. **Full Video Transcode (last resort)**
   Only perform a full video transcode if the client **cannot decode the video codec** or the bitrate/resolution exceeds device limits.

   In that case:

   * transcode video to a compatible codec (ex: H264)
   * transcode audio if required

Implementation requirements:

* Add a **playback decision engine** that compares:

  * client codec support
  * container support
  * max resolution
  * max bitrate
  * subtitle support

* Before starting FFmpeg, evaluate the streams and choose the **cheapest playback mode**.

* Use `ffprobe` to detect:

  * video codec
  * audio codec
  * container
  * resolution
  * bitrate
  * subtitle streams

* Only spawn FFmpeg when:

  * remuxing
  * audio transcoding
  * full transcoding

* Log the selected playback mode:

  ```
  PlaybackMode: DirectPlay
  PlaybackMode: Remux
  PlaybackMode: DirectStream
  PlaybackMode: Transcode
  ```

Goal:
Most playback should resolve to **DirectPlay or Remux** whenever the client supports the media.

Avoid unnecessary transcoding because it wastes CPU/GPU r
