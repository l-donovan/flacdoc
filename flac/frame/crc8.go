// This code adapted from https://github.com/sigurn/crc8

package frame

var table [256]uint8

func init() {
	for n := range 256 {
		crc := uint8(n)

		for i := 0; i < 8; i++ {
			bit := (crc & 0x80) != 0
			crc <<= 1

			if bit {
				crc ^= 0x07
			}
		}

		table[n] = crc
	}
}

func Checksum(data []byte) (crc uint8) {
	for _, d := range data {
		crc = table[crc^d]
	}

	return crc
}
