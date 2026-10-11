package frame

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/l-donovan/flacdoc/flac/metadata"
	"github.com/sigurn/crc8"
)

var table *crc8.Table

func init() {
	table = crc8.MakeTable(crc8.CRC8)
}

type Scanner struct {
	buf              []byte
	readIdx          int
	writeIdx         int
	src              io.ReadSeeker
	streaminfo       metadata.StreamInfo
	blockingStrategy *uint8
	lastCodedNumber  *int
	SampleCount      uint64
}

func NewScanner(src io.ReadSeeker, streaminfo metadata.StreamInfo) Scanner {
	// A FLAC frame header is always between 5 and 15 Bytes, not including the CRC.
	buf := make([]byte, 15)

	f := Scanner{
		buf:         buf,
		readIdx:     0,
		writeIdx:    0,
		src:         src,
		streaminfo:  streaminfo,
		SampleCount: 0,
	}

	return f
}

func (f *Scanner) readCodedNumber() (int, error) {
	n, err := f.ReadBuf(1)

	if err != nil {
		return 0, err
	}

	if (n[0] >> 7) == 0b0 {
		return int(n[0]), nil
	} else if (n[0] >> 5) == 0b110 {
		next, err := f.ReadBuf(1)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		return ((int(n[0]) & 0b00011111) << 6) | (int(n[1]) & 0b00111111), nil
	} else if (n[0] >> 4) == 0b1110 {
		next, err := f.ReadBuf(2)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		return ((int(n[0]) & 0b00001111) << 12) | ((int(n[1]) & 0b00111111) << 6) | (int(n[2]) & 0b00111111), nil
	} else if (n[0] >> 3) == 0b11110 {
		next, err := f.ReadBuf(3)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		// TODO
	} else if (n[0] >> 2) == 0b111110 {
		next, err := f.ReadBuf(4)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		// TODO
	} else if (n[0] >> 1) == 0b1111110 {
		next, err := f.ReadBuf(5)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		// TODO
	} else if n[0] == 0b11111110 {
		next, err := f.ReadBuf(6)

		if err != nil {
			return 0, err
		}

		n = append(n, next...)
		// TODO
	}

	return 0, fmt.Errorf("invalid coded number bits %d", n[0])
}

func (f *Scanner) readBlockSize(bits uint8) (uint16, error) {
	if bits == 0b0000 {
		return 0, fmt.Errorf("invalid block size bits %d", bits)
	} else if bits == 0b0001 {
		return 192, nil
	} else if bits >= 0b0010 && bits < 0b0110 {
		return 144 * (1 << bits), nil
	} else if bits == 0b0110 {
		next, err := f.ReadBuf(1)

		if err != nil {
			return 0, err
		}

		return uint16(next[0]) + 1, nil
	} else if bits == 0b0111 {
		next, err := f.ReadBuf(2)

		if err != nil {
			return 0, err
		}

		return uint16(binary.BigEndian.Uint16(next)) + 1, nil
	} else {
		return 1 << bits, nil
	}
}

func (f *Scanner) readSampleRate(bits uint8) (uint32, error) {
	switch bits {
	case 0b0000:
		return f.streaminfo.SampleRate, nil
	case 0b0001:
		return 88200, nil
	case 0b0010:
		return 176400, nil
	case 0b0011:
		return 192000, nil
	case 0b0100:
		return 8000, nil
	case 0b0101:
		return 16000, nil
	case 0b0110:
		return 22050, nil
	case 0b0111:
		return 24000, nil
	case 0b1000:
		return 32000, nil
	case 0b1001:
		return 44100, nil
	case 0b1010:
		return 48000, nil
	case 0b1011:
		return 96000, nil
	case 0b1100:
		next, err := f.ReadBuf(1)

		if err != nil {
			return 0, err
		}

		return 1000 * uint32(next[0]), nil
	case 0b1101:
		next, err := f.ReadBuf(2)

		if err != nil {
			return 0, err
		}

		return uint32(binary.BigEndian.Uint16(next)), nil
	case 0b1110:
		next, err := f.ReadBuf(2)

		if err != nil {
			return 0, err
		}

		return 10 * uint32(binary.BigEndian.Uint16(next)), nil
	default:
		return 0, fmt.Errorf("invalid sample rate %d", bits)
	}
}

func (f *Scanner) readChannels(bits uint8) (string, error) {
	switch bits {
	case 0b0000:
		return "1 channel: mono", nil
	case 0b0001:
		return "2 channels: left, right", nil
	case 0b0010:
		return "3 channels: left, right, center", nil
	case 0b0011:
		return "4 channels: front left, front right, back left, back right", nil
	case 0b0100:
		return "5 channels, front left, front right, front center, back/surround left, back/surround right", nil
	case 0b0101:
		return "6 channels: front left, front right, front center, LFE, back/surround left, back/surround right", nil
	case 0b0110:
		return "7 channels: front left, front right, front center, LFE, back center, side left, side right", nil
	case 0b0111:
		return "8 channels: front left, front right, front center, LFE, back left, back right, side left, side right", nil
	case 0b1000:
		return "2 channels: left, right; stored as left-side stereo", nil
	case 0b1001:
		return "2 channels: left, right; stored as side-right stereo", nil
	case 0b1010:
		return "2 channels: left, right; stored as mid-side stereo", nil
	default:
		return "", fmt.Errorf("invalid channel bits %d", bits)
	}
}

func (f *Scanner) readBitDepth(bits uint8) (uint8, error) {
	switch bits {
	case 0b000:
		return f.streaminfo.BitDepth, nil
	case 0b001:
		return 8, nil
	case 0b010:
		return 12, nil
	default: // bits should only be u3, so this is an exhaustive switch. But using default keeps the compiler happy.
		return 0, fmt.Errorf("invalid bit depth bits %d", bits)
	case 0b100:
		return 16, nil
	case 0b101:
		return 20, nil
	case 0b110:
		return 24, nil
	case 0b111:
		return 32, nil
	}
}

func (f *Scanner) rotateBufLeft() {
	for i := range 14 {
		f.buf[i] = f.buf[i+1]
	}

	f.readIdx--
	f.writeIdx--
}

func (f *Scanner) ReadBuf(n int) ([]byte, error) {
	available := f.writeIdx - f.readIdx

	if n > available {
		// We always want to fill the buffer.
		// One 10-Byte read is much faster than ten 1-Byte reads.

		m, err := f.src.Read(f.buf[f.writeIdx:])

		if err != nil {
			return nil, err
		}

		f.writeIdx += m
	}

	out := f.buf[f.readIdx : f.readIdx+n]
	f.readIdx += n

	return out, nil
}

func (f *Scanner) Init() error {
	frameHeaderBytes, err := f.ReadBuf(2)

	if err != nil {
		return err
	}

	frameHeader := binary.BigEndian.Uint16(frameHeaderBytes)

	if (frameHeader & 0xfffe) != 0xfff8 {
		return errors.New("invalid frame header")
	}

	frameBlockingStrategy := uint8(frameHeader & 0b1)

	if _, err := f.src.Seek(-2, io.SeekCurrent); err != nil {
		return err
	}

	f.blockingStrategy = new(frameBlockingStrategy)

	return nil
}

func (f *Scanner) ReadFrameHeader() error {
	var location string
	frameHeaderBytes, err := f.ReadBuf(2)

	if err != nil {
		return fmt.Errorf("error on line 371: %w", err)
	}

	frameHeader := binary.BigEndian.Uint16(frameHeaderBytes)

	if (frameHeader & 0xfffe) != 0xfff8 {
		return errors.New("invalid frame header")
	}

	frameBlockingStrategy := uint8(frameHeader & 0b1)

	if frameBlockingStrategy != *f.blockingStrategy {
		return errors.New("blocking strategy changed mid-stream")
	}

	meta, err := f.ReadBuf(2)

	if err != nil {
		return fmt.Errorf("error on line 391: %w", err)
	}

	blockSizeBits := meta[0] >> 4
	sampleRateBits := meta[0] & 0x0f
	channelBits := meta[1] >> 4
	bitDepthBits := (meta[1] >> 1) & 0x07

	codedNumber, err := f.readCodedNumber()

	if err != nil {
		return err
	}

	blockSize, err := f.readBlockSize(blockSizeBits)

	if err != nil {
		return err
	}

	if *f.blockingStrategy == 0b0 && (f.lastCodedNumber == nil || codedNumber == *f.lastCodedNumber+1) {
		location = fmt.Sprintf("Frame %d", codedNumber)
	} else if *f.blockingStrategy == 0b1 && (f.lastCodedNumber == nil || codedNumber == *f.lastCodedNumber+int(blockSize)) {
		location = fmt.Sprintf("Sample %d", codedNumber)
	} else {
		return errors.New("bad coded number")
	}

	sampleRate, err := f.readSampleRate(sampleRateBits)

	if err != nil {
		return err
	}

	channels, err := f.readChannels(channelBits)

	if err != nil {
		return err
	}

	bitDepth, err := f.readBitDepth(bitDepthBits)

	if err != nil {
		return err
	}

	crcChecksum := crc8.Checksum(f.buf[:f.readIdx], table)
	crcExpected, err := f.ReadBuf(1)

	if err != nil {
		return fmt.Errorf("error on line 434: %w", err)
	}

	if crcChecksum != crcExpected[0] {
		return fmt.Errorf("invalid crc")
	}

	// if *verbose {
	here, _ := f.src.Seek(0, io.SeekCurrent)
	fmt.Printf("(%d-%d) [%d] %s, %d block size, %dHz/%db, %s\n", int(here)-f.readIdx, here, f.SampleCount, location, blockSize, sampleRate, bitDepth, channels)
	// }

	f.lastCodedNumber = &codedNumber
	f.SampleCount += uint64(blockSize)

	return nil
}

func (f *Scanner) Scan() error {
	for {
		f.readIdx = 0

		if err := f.ReadFrameHeader(); err != nil {
			if errors.Is(err, io.EOF) {
				return err
			}

			f.rotateBufLeft()
		} else {
			return nil
		}
	}
}
