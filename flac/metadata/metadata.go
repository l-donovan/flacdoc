package metadata

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
)

type StreamInfo struct {
	SampleRate   uint32
	ChannelCount uint8
	BitDepth     uint8
	SampleCount  uint64
}

type Metadata any

func parseStreamInfo(src io.ReadSeeker, metadataSize uint32) (StreamInfo, error) {
	s := StreamInfo{}
	metadata := make([]byte, metadataSize)

	if _, err := src.Read(metadata); err != nil {
		return s, err
	}

	chunk := metadata[10:18]
	s.SampleRate = (uint32(chunk[0]) << 12) | (uint32(chunk[1]) << 4) | (uint32(chunk[2]) >> 4)                       // u20
	s.ChannelCount = ((chunk[2] >> 1) & 0b111) + 1                                                                    // u3
	s.BitDepth = (((chunk[2] & 0b1) << 4) | (chunk[3] >> 4)) + 1                                                      // u5
	s.SampleCount = binary.BigEndian.Uint64([]byte{0, 0, 0, chunk[3] & 0x0f, chunk[4], chunk[5], chunk[6], chunk[7]}) // u36

	return s, nil
}

func Parse(src io.ReadSeeker) ([]Metadata, error) {
	var metadata []Metadata
	magicNumber := make([]byte, 4)

	if _, err := src.Read(magicNumber); err != nil {
		return metadata, err
	}

	if !slices.Equal(magicNumber, []byte("fLaC")) {
		return metadata, fmt.Errorf("magic number was %q intead of 'fLaC'", magicNumber)
	}

	moreMetadata := true
	header := make([]byte, 4)

	for moreMetadata {
		if _, err := src.Read(header); err != nil {
			return metadata, err
		}

		moreMetadata = header[0]&0b10000000 == 0b00000000
		metadataType := header[0] & 0b1111111
		metadataSize := binary.BigEndian.Uint32([]byte{0, header[1], header[2], header[3]}) // u24

		switch metadataType {
		case 0b000000:
			s, err := parseStreamInfo(src, metadataSize)

			if err != nil {
				return metadata, err
			}

			metadata = append(metadata, s)
		default:
			src.Seek(int64(metadataSize), io.SeekCurrent)
		}
	}

	return metadata, nil
}
