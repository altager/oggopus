// This package contains OGG-based OPUS reader
package opusreader

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	opusHeadPrefix = "OpusHead"
	opusTagsPrefix = "OpusTags"
)

// Contains fields used in opus identification header
// https://tools.ietf.org/html/rfc7845#section-5.1
type OPUSIDHeader struct {
	CapturePattern       [8]byte
	Version              uint8
	ChannelCount         uint8  // 1...255
	PreSkip              uint16 // LE
	InputSampleRate      uint32 // LE
	OutputGain           uint16 // LE
	ChannelMappingFamily uint8
}

// Contains fields used in TOC byte + some additional packet info
// https://tools.ietf.org/html/rfc6716#section-3.1
type OPUSPacketConfig struct {
	ConfigCode uint8
	SoundMode  uint8

	FramesNumber          int
	SamplesNumberPerFrame int
	TotalSamples          int
}

// Contains packet config and raw packet data
type OPUSPacket struct {
	OPUSPacketConfig

	PacketData []byte
}

// Reader object which encapsulates OGG-reader
type OPUSReader struct {
	OGGReader *OGGReader

	OPUSIDHeader
	VendorName []byte

	CurrentPacket *OPUSPacket

	skipped     int
	initialized bool
	LastPacket  bool
	Duration    int
}

// Get samples number per frame
func getSamplesPerFrame(data []byte) int {
	fs := 48000
	var audiosize int
	if (data[0] & 0x80) != 0 {
		audiosize = int((data[0] >> 3) & 0x3)
		audiosize = (fs << audiosize) / 400
	} else if (data[0] & 0x60) == 0x60 {
		if (data[0] & 0x8) != 0 {
			audiosize = fs / 50
		} else {
			audiosize = fs / 100
		}
	} else {
		audiosize = int((data[0] >> 3) & 0x3)
		if audiosize == 3 {
			audiosize = fs * 60 / 1000
		} else {
			audiosize = (fs << audiosize) / 100
		}
	}
	return audiosize
}

// Return a OPUSReader containing the input stream
func NewOpusReader(in io.Reader) (*OPUSReader, error) {
	oggReader, _ := NewOggReader(in)
	return &OPUSReader{
		OGGReader: oggReader,
	}, nil
}

func (p *OPUSPacket) readPacketConfig() error {
	if len(p.PacketData) < 1 {
		return errors.New("opusreader: invalid TOC byte")
	}
	p.OPUSPacketConfig = OPUSPacketConfig{
		ConfigCode:            (p.PacketData[0] >> 3) & 31,
		SoundMode:             (p.PacketData[0] >> 2) & 1,
		FramesNumber:          getFramesNumberInPacket(p.PacketData),
		SamplesNumberPerFrame: getSamplesPerFrame(p.PacketData),
	}

	p.OPUSPacketConfig.TotalSamples = p.FramesNumber * p.SamplesNumberPerFrame

	return nil
}

func getFramesNumberInPacket(packet []byte) int {
	code := packet[0] & 3
	if code == 0 {
		return 1
	} else if code != 3 {
		return 2
	} else {
		// TODO: FrameCountByte
		return int(packet[1]) & 0x3F
	}
}

func (o *OPUSReader) readHeaders() error {
	err := o.readIDHeader()
	if err != nil {
		return err
	}

	err = o.readTags()
	if err != nil {
		return err
	}

	o.initialized = true

	return nil
}

// This methods reads th OPUS identification header
func (o *OPUSReader) readIDHeader() error {
	headerPacketData, err := o.OGGReader.NextPacket()
	if err != nil {
		return err
	}

	opusHeader := OPUSIDHeader{}

	if string(headerPacketData[:8]) != opusHeadPrefix {
		return errors.New("opusreader: invalid id header prefix")
	}

	opusHeader.Version = headerPacketData[8]
	opusHeader.ChannelCount = headerPacketData[9]
	if opusHeader.ChannelCount == 0 {
		return errors.New("opusreader: channels count < 1")
	}

	opusHeader.PreSkip = binary.LittleEndian.Uint16(headerPacketData[10:12])
	opusHeader.InputSampleRate = binary.LittleEndian.Uint32(headerPacketData[12:16])
	opusHeader.OutputGain = binary.LittleEndian.Uint16(headerPacketData[16:18])

	opusHeader.ChannelMappingFamily = headerPacketData[18]
	if opusHeader.ChannelMappingFamily != 0 {
		// TODO: support mappings > 0
		return errors.New("opusreader: for now library supports only channel mapping 0")
	}

	o.OPUSIDHeader = opusHeader

	return nil
}

// For now it reads only the vendor name
// https://tools.ietf.org/html/rfc7845#section-5.2
func (o *OPUSReader) readTags() error {
	headerPacketData, err := o.OGGReader.NextPacket()
	if err != nil {
		return err
	}

	if string(headerPacketData[:8]) != opusTagsPrefix {
		return errors.New("opusreader: invalid tags header prefix")
	}

	var vendorNameLength uint32
	vendorNameLength = binary.LittleEndian.Uint32(headerPacketData[8:12])
	o.VendorName = headerPacketData[12 : 12+vendorNameLength]

	return nil
}

// Method for iterating over the opus packets
func (o *OPUSReader) NextPacket() (*OPUSPacket, error) {
	if o.LastPacket {
		return nil, errors.New("opusreader: EOS")
	}

	opusPacket := new(OPUSPacket)

	if !o.initialized {
		err := o.readHeaders()
		if err != nil {
			return nil, err
		}
	}

	packetData, err := o.OGGReader.NextPacket()
	if err != nil {
		return nil, err
	}

	opusPacket.PacketData = packetData
	err = opusPacket.readPacketConfig()
	if err != nil {
		return nil, err
	}

	if o.OGGReader.lastPacket {
		o.LastPacket = true
	}

	if string(packetData[:2]) == "Op" {
		// Just skip an additional tags
		return o.NextPacket()
	}

	if opusPacket.SamplesNumberPerFrame > 0 {
		if opusPacket.FramesNumber > 0 {
			var needsSkip int
			needsSkip = int(o.PreSkip) - o.skipped
			if needsSkip > 0 {
				var skip int
				if opusPacket.TotalSamples < needsSkip {
					skip = opusPacket.TotalSamples
				} else {
					skip = needsSkip
				}
				opusPacket.TotalSamples -= skip
				o.skipped += skip
			}
			// in microseconds
			o.Duration += opusPacket.TotalSamples * 1000000 / 48000
		}
	}

	return opusPacket, nil
}
