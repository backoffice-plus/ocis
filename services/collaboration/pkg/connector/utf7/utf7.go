package utf7

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode"
	"unicode/utf16"
)

type Range struct {
	Name string
	Low  int
	High int
}

var utf7AsciiRT = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x27, 0x29, 1}, // '()
		{0x2c, 0x2f, 1}, // ,-./
		{0x30, 0x39, 1}, // 0-9
		{0x3a, 0x3f, 5}, // :?
		{0x41, 0x5a, 1}, // A-Z
		{0x61, 0x7a, 1}, // a-z
	},
}

func EncodeString(s string) string {
	runes := []rune(s)

	ranges := analyzeRanges(runes)

	var sb strings.Builder
	sb.Grow(len(runes) * 2)
	for _, v := range ranges {
		if v.Name == "ascii" {
			sb.WriteString(string(runes[v.Low:v.High]))
		} else {
			sb.WriteString(convertToUtf7(runes[v.Low:v.High]))
		}
	}
	return sb.String()
}

func DecodeString(s string) (string, error) {
	bytes := []byte(s)

	ranges := analyzeUtf7(bytes)

	var sb strings.Builder
	sb.Grow(len(bytes))
	for _, v := range ranges {
		if v.Name == "ascii" {
			sb.WriteString(string(bytes[v.Low:v.High]))
		} else {
			utf7ByteRange := bytes[v.Low:v.High]
			if len(utf7ByteRange) == 2 && utf7ByteRange[0] == '+' && utf7ByteRange[1] == '-' {
				sb.WriteByte('+')
			} else {
				decodedStr, err := convertFromUtf7(bytes[v.Low+1 : v.High-1])
				if err != nil {
					return "", err
				}
				sb.WriteString(decodedStr)
			}
		}
	}
	return sb.String(), nil
}

func analyzeRanges(runes []rune) []Range {
	ranges := make([]Range, 0)

	var currentRange Range
	for k, v := range runes {
		if unicode.Is(utf7AsciiRT, v) {
			if currentRange.Name == "" {
				// take control of the current range
				currentRange.Name = "ascii"
				currentRange.Low = k
			} else if currentRange.Name != "ascii" {
				// close current range and open a new one
				currentRange.High = k
				ranges = append(ranges, currentRange)
				currentRange = Range{
					Name: "ascii",
					Low:  k,
				}
			}
		} else {
			if currentRange.Name == "" {
				// take control of the current range
				currentRange.Name = "utf7"
				currentRange.Low = k
			} else if currentRange.Name != "utf7" {
				// close current range and open a new one
				currentRange.High = k
				ranges = append(ranges, currentRange)
				currentRange = Range{
					Name: "utf7",
					Low:  k,
				}
			}
		}
	}
	// close the last range
	currentRange.High = len(runes)
	ranges = append(ranges, currentRange)

	return ranges
}

func convertToUtf7(runes []rune) string {
	bytes := make([]byte, 0)

	u16 := utf16.Encode(runes)
	for _, v := range u16 {
		bytes = binary.BigEndian.AppendUint16(bytes, v)
	}
	return "+" + base64.RawStdEncoding.EncodeToString(bytes) + "-"
}

func convertFromUtf7(bytes []byte) (string, error) {
	dst := make([]byte, base64.RawStdEncoding.DecodedLen(len(bytes)))

	_, err := base64.RawStdEncoding.Decode(dst, bytes)
	if err != nil {
		return "", err
	}

	u16array := make([]uint16, 0)
	for i := 0; i < len(dst); i++ {
		u16array = append(u16array, binary.BigEndian.Uint16(dst[i:i+2]))
		i = i + 1
	}
	s4 := utf16.Decode(u16array)
	return string(s4), nil
}

func analyzeUtf7(bytes []byte) []Range {
	ranges := make([]Range, 0)

	currentRange := Range{
		Name: "ascii",
		Low:  0,
	}

	for k, v := range bytes {
		if v == '+' {
			// start utf7-encoded range
			currentRange.High = k
			ranges = append(ranges, currentRange)
			currentRange = Range{
				Name: "utf7",
				Low:  k,
			}
		} else if v == '-' {
			// close utf7-encoded range
			currentRange.High = k + 1 // the '-' char is part of the range
			ranges = append(ranges, currentRange)
			currentRange = Range{
				Name: "ascii",
				Low:  k + 1,
			}
		}
	}

	// close the last range
	currentRange.High = len(bytes)
	ranges = append(ranges, currentRange)

	// there might be empty ranges we need to clear
	// empty ranges have Low = High
	realRanges := make([]Range, 0, len(ranges))
	for _, v := range ranges {
		if v.Low != v.High {
			realRanges = append(realRanges, v)
		}
	}

	return realRanges
}
