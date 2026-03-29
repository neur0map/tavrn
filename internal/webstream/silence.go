package webstream

// SilentFrame is a valid MPEG1 Layer III frame that decodes to silence.
// Header: FF FB 90 00 = MPEG1, Layer3, 128kbps, 44100Hz, Stereo, no CRC.
// Frame body zeroed: part2_3_length=0 for all granules → no audio data.
// Size: floor(144 * 128000 / 44100) = 417 bytes (~26ms of silence).
var SilentFrame = func() []byte {
	f := make([]byte, 417)
	f[0] = 0xFF
	f[1] = 0xFB
	f[2] = 0x90
	f[3] = 0x00
	return f
}()
