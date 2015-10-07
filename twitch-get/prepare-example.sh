
#!/bin/bash
ffmpeg -i input.mp4 \
	-c copy \
	-bsf:a aac_adtstoasc \
	-metadata title="Stream name" \
	-metadata author="Cauthon" \
	-metadata copyright="Copyright 2015 Cauthon" \
	-metadata comment="Stream recorded by ernado" \
	-movflags faststart \
	output.mp4
