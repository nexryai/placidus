package transcode

// LiveArguments builds deterministic FFmpeg arguments for low-latency browser HLS.
func LiveArguments(inputURL, playlistPath string, encoder Encoder) []string {
	args := []string{"-hide_banner", "-loglevel", "warning", "-nostdin", "-y"}
	args = append(args, encoder.InputArgs...)
	args = append(args,
		"-i", inputURL,
		"-map", "0:v:0",
		"-map", "0:a:0?",
	)
	args = append(args, encoder.FilterArgs...)
	args = append(args, "-c:v", encoder.Name)
	args = append(args, encoder.VideoArgs...)
	args = append(args,
		"-g", "60",
		"-keyint_min", "60",
		"-sc_threshold", "0",
		"-c:a", "aac",
		"-b:a", "192k",
		"-ac", "2",
		"-ar", "48000",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "6",
		"-hls_flags", "delete_segments+append_list+independent_segments",
		"-hls_segment_filename", playlistPath[:len(playlistPath)-len("index.m3u8")]+"segment-%06d.ts",
		playlistPath,
	)
	return args
}
