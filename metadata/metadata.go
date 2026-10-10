package metadata

import (
	"encoding/binary"
	"errors"
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

func ParseStreamInfo(src io.ReadSeeker) (StreamInfo, error) {
	s := StreamInfo{}

	magicNumber := make([]byte, 4)

	if _, err := src.Read(magicNumber); err != nil {
		return s, err
	}

	if !slices.Equal(magicNumber, []byte("fLaC")) {
		return s, fmt.Errorf("magic number was %q intead of 'fLaC'", magicNumber)
	}

	moreMetadata := true
	firstMetadata := true
	header := make([]byte, 4)

	for moreMetadata {
		if _, err := src.Read(header); err != nil {
			return s, err
		}

		moreMetadata = header[0]&0b10000000 == 0b00000000
		metadataSize := binary.BigEndian.Uint32([]byte{0, header[1], header[2], header[3]}) // u24

		if firstMetadata {
			isStreamInfo := header[0]&0b01111111 == 0b00000000

			if !isStreamInfo {
				return s, errors.New("no streaminfo found")
			}

			metadata := make([]byte, metadataSize)

			if _, err := src.Read(metadata); err != nil {
				return s, err
			}

			firstMetadata = false
			chunk := metadata[10:18]
			s.SampleRate = (uint32(chunk[0]) << 12) | (uint32(chunk[1]) << 4) | (uint32(chunk[2]) >> 4)                       // u20
			s.ChannelCount = ((chunk[2] >> 1) & 0b111) + 1                                                                    // u3
			s.BitDepth = (((chunk[2] & 0b1) << 4) | (chunk[3] >> 4)) + 1                                                      // u5
			s.SampleCount = binary.BigEndian.Uint64([]byte{0, 0, 0, chunk[3] & 0x0f, chunk[4], chunk[5], chunk[6], chunk[7]}) // u36
		} else {
			src.Seek(int64(metadataSize), io.SeekCurrent)
		}
	}

	return s, nil
}
