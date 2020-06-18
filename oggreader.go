package opusreader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type OGGPageHeader struct {
	CapturePattern          [4]byte
	Version                 uint8
	HeaderType              uint8
	AbsoluteGranulePosition int64
	BitStreamSerialNumber   uint32
	SequenceNumber          uint32
	Checksum                uint32
	SegmentsNumber          uint8
}

type OGGPage struct {
	OGGPageHeader

	initialized bool

	packets      [][]byte
	packetsCount int
	packetSizes  []int
	totalSize    int

	needsContinue bool
}

type OGGReader struct {
	stream               io.Reader
	bytesReadSuccesfully int64
	initialized          bool

	CurrentPage      *OGGPage
	lastPacket       bool
	packetIndex      int
	lastPagePosition int64
}

const (
	headerFlagContinuedPacket   = 1
	headerFlagBeginningOfStream = 2
	headerFlagEndOfStream       = 4
)

var capturePattern = [4]byte{'O', 'g', 'g', 'S'}

//  NewWith returns a new OGGReader with an io.Reader input
func NewOggReader(in io.Reader) (*OGGReader, error) {
	if in == nil {
		return nil, fmt.Errorf("stream is nil")
	}

	reader := &OGGReader{
		stream:               in,
		bytesReadSuccesfully: 0,
	}

	return reader, nil
}

// ResetReader resets the internal stream of OGGReader. This is useful
// for live streams, where the end of the file might be read without the
// data being finished.
func (o *OGGReader) ResetReader(reset func(bytesRead int64) io.Reader) {
	o.stream = reset(o.bytesReadSuccesfully)
}

func (o *OGGReader) readPage() error {
	o.CurrentPage = new(OGGPage)
	if err := o.readPageHeader(); err != nil {
		return err
	}
	if err := o.readPageContent(); err != nil {
		return err
	}

	o.CurrentPage.initialized = true

	return nil
}

func (o *OGGReader) readPageContent() error {
	page := o.CurrentPage
	content := make([]byte, page.totalSize)
	_, err := io.ReadFull(o.stream, content)
	if err != nil {
		return err
	}
	o.bytesReadSuccesfully += int64(page.totalSize)

	page.packets = make([][]byte, page.packetsCount+1)
	offset := 0
	for i, size := range page.packetSizes {
		page.packets[i] = content[offset : offset+size]
		offset += size
	}
	page.packets[page.packetsCount] = content[offset:]

	return nil
}

func (o *OGGReader) readPageHeader() error {
	page := o.CurrentPage
	data := make([]byte, 27)
	_, err := io.ReadFull(o.stream, data)
	if err != nil {
		return err
	}
	o.bytesReadSuccesfully += 27

	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, &page.OGGPageHeader)
	if err != nil {
		return errors.New("ogg: error reading header")
	}
	if page.CapturePattern != capturePattern {
		return errors.New("ogg: missing capture pattern")
	}
	if page.Version != 0 {
		return errors.New("ogg: unsupported version")
	}

	segmentTable := make([]byte, page.SegmentsNumber)
	_, err = io.ReadFull(o.stream, segmentTable)
	if err != nil {
		return err
	}
	o.bytesReadSuccesfully += int64(page.SegmentsNumber)

	size := 0
	page.totalSize = 0
	page.packetsCount = 0
	page.packetSizes = nil
	for _, s := range segmentTable {
		size += int(s)
		page.totalSize += int(s)
		if s < 0xFF {
			page.packetsCount++
			page.packetSizes = append(page.packetSizes, size)
			size = 0
		}
	}
	page.needsContinue = segmentTable[page.OGGPageHeader.SegmentsNumber-1] == 0xFF

	return nil
}

func (p *OGGPage) isFirst() bool { return p.OGGPageHeader.HeaderType&headerFlagBeginningOfStream != 0 }
func (p *OGGPage) isLast() bool  { return p.OGGPageHeader.HeaderType&headerFlagEndOfStream != 0 }

func (o *OGGReader) NextPacket() ([]byte, error) {
	if !o.initialized {
		err := o.readPage()
		if err != nil {
			return nil, err
		}
		o.packetIndex = 0
		if o.CurrentPage.HeaderType&headerFlagContinuedPacket != 0 {
			o.packetIndex = 1
		}
		o.initialized = true
	}
	if o.packetIndex == o.CurrentPage.packetsCount {
		rest := o.CurrentPage.packets[o.CurrentPage.packetsCount]
		if o.CurrentPage.AbsoluteGranulePosition != -1 {
			o.lastPagePosition = o.CurrentPage.AbsoluteGranulePosition
		}
		err := o.readPage()
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			o.CurrentPage.packets[0] = append(rest, o.CurrentPage.packets[0]...)
		}
		o.packetIndex = 0
		return o.NextPacket()
	}
	packet := o.CurrentPage.packets[o.packetIndex]
	o.packetIndex++
	if o.packetIndex == o.CurrentPage.packetsCount && o.CurrentPage.isLast() {
		o.lastPacket = true
	}
	return packet, nil
}
