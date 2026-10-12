# Rendered from deploy variables: the release ships ffmpeg/ffprobe in bin/,
# and the version plus sha pins come from its ffmpeg-source.json.

ffmpeg_path = 'bin/ffmpeg'
ffprobe_path = 'bin/ffprobe'
version = {{WT_FFMPEG_VERSION}}
required_encoders = ['libx264', 'aac']
ffmpeg_sha256 = {{WT_FFMPEG_SHA256}}
ffprobe_sha256 = {{WT_FFPROBE_SHA256}}
