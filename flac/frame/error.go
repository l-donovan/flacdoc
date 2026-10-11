package frame

import "errors"

type badScan struct{}

var (
	errorInvalidFrameHeader      = errors.New("invalid frame header")
	errorBlockingStrategyChanged = errors.New("blocking strategy changed mid-stream")
	errorBadCodedNumber          = errors.New("bad coded number")
	errorInvalidCrc              = errors.New("invalid crc")
)
