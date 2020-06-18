package opusreader

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestIDHeader(t *testing.T) {
	ogg, err := os.Open("testdata/speech_orig.ogg")
	if err != nil {
		t.Fatal(err)
	}
	defer ogg.Close()

	reader, err := NewOpusReader(ogg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = reader.NextPacket()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint16(0x138), reader.PreSkip, "Pre-skip is not 0")
	assert.Equal(t, true, reader.initialized, "Reader is not initialized")
	assert.Equal(t, "Lavf58.42.101", string(reader.VendorName), "Wrong vendor name")
	assert.Equal(t, uint8(0x02), reader.ChannelCount, "Wrong channel count")
	assert.Equal(t, uint32(48000), reader.InputSampleRate, "Wrong sample rates")
	assert.Equal(t, uint8(1), reader.Version, "Wrong version")
}
