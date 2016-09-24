// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iconvg

import (
	"bytes"
	"errors"
)

var (
	errInconsistentMetadataChunkLength = errors.New("iconvg: inconsistent metadata chunk length")
	errInvalidMagicIdentifier          = errors.New("iconvg: invalid magic identifier")
	errInvalidMetadataChunkLength      = errors.New("iconvg: invalid metadata chunk length")
	errInvalidMetadataIdentifier       = errors.New("iconvg: invalid metadata identifier")
	errInvalidNumber                   = errors.New("iconvg: invalid number")
	errInvalidNumberOfMetadataChunks   = errors.New("iconvg: invalid number of metadata chunks")
	errInvalidViewBox                  = errors.New("iconvg: invalid view box")
	errUnsupportedDrawingOpcode        = errors.New("iconvg: unsupported drawing opcode")
	errUnsupportedMetadataIdentifier   = errors.New("iconvg: unsupported metadata identifier")
	errUnsupportedStylingOpcode        = errors.New("iconvg: unsupported styling opcode")
)

var midDescriptions = [...]string{
	midViewBox:          "viewBox",
	midSuggestedPalette: "suggested palette",
}

// Destination handles the actions decoded from an IconVG graphic's opcodes.
//
// When passed to Decode, the first method called (if any) will be Reset. No
// methods will be called at all if an error is encountered in the encoded form
// before the metadata is fully decoded.
type Destination interface {
	Reset(m Metadata)

	// TODO: styling mode ops other than StartPath.

	StartPath(adj int, x, y float32)
	ClosePathEndPath()
	ClosePathAbsMoveTo(x, y float32)
	ClosePathRelMoveTo(x, y float32)

	AbsHLineTo(x float32)
	RelHLineTo(x float32)
	AbsVLineTo(y float32)
	RelVLineTo(y float32)
	AbsLineTo(x, y float32)
	RelLineTo(x, y float32)
	AbsSmoothQuadTo(x, y float32)
	RelSmoothQuadTo(x, y float32)
	AbsQuadTo(x1, y1, x, y float32)
	RelQuadTo(x1, y1, x, y float32)
	AbsSmoothCubeTo(x2, y2, x, y float32)
	RelSmoothCubeTo(x2, y2, x, y float32)
	AbsCubeTo(x1, y1, x2, y2, x, y float32)
	RelCubeTo(x1, y1, x2, y2, x, y float32)
	AbsArcTo(rx, ry, xAxisRotation float32, largeArc, sweep bool, x, y float32)
	RelArcTo(rx, ry, xAxisRotation float32, largeArc, sweep bool, x, y float32)
}

type printer func(b []byte, format string, args ...interface{})

// DecodeOptions are the optional parameters to the Decode function.
type DecodeOptions struct {
	// Palette is an optional 64 color palette. If one isn't provided, the
	// IconVG graphic's suggested palette will be used.
	Palette *Palette
}

// DecodeMetadata decodes only the metadata in an IconVG graphic.
func DecodeMetadata(src []byte) (m Metadata, err error) {
	m.ViewBox = DefaultViewBox
	m.Palette = DefaultPalette
	if err = decode(nil, nil, &m, true, src, nil); err != nil {
		return Metadata{}, err
	}
	return m, nil
}

// Decode decodes an IconVG graphic.
func Decode(dst Destination, src []byte, opts *DecodeOptions) error {
	m := Metadata{
		ViewBox: DefaultViewBox,
		Palette: DefaultPalette,
	}
	if opts != nil && opts.Palette != nil {
		m.Palette = *opts.Palette
	}
	return decode(dst, nil, &m, false, src, opts)
}

func decode(dst Destination, p printer, m *Metadata, metadataOnly bool, src buffer, opts *DecodeOptions) (err error) {
	if !bytes.HasPrefix(src, magicBytes) {
		return errInvalidMagicIdentifier
	}
	if p != nil {
		p(src[:len(magic)], "Magic identifier\n")
	}
	src = src[len(magic):]

	nMetadataChunks, n := src.decodeNatural()
	if n == 0 {
		return errInvalidNumberOfMetadataChunks
	}
	if p != nil {
		p(src[:n], "Number of metadata chunks: %d\n", nMetadataChunks)
	}
	src = src[n:]

	for ; nMetadataChunks > 0; nMetadataChunks-- {
		src, err = decodeMetadataChunk(p, m, src, opts)
		if err != nil {
			return err
		}
	}
	if metadataOnly {
		return nil
	}
	if dst != nil {
		dst.Reset(*m)
	}

	mf := modeFunc(decodeStyling)
	for len(src) > 0 {
		mf, src, err = mf(dst, p, src)
		if err != nil {
			return err
		}
	}
	return nil
}

func decodeMetadataChunk(p printer, m *Metadata, src buffer, opts *DecodeOptions) (src1 buffer, err error) {
	length, n := src.decodeNatural()
	if n == 0 {
		return nil, errInvalidMetadataChunkLength
	}
	if p != nil {
		p(src[:n], "Metadata chunk length: %d\n", length)
	}
	src = src[n:]
	lenSrcWant := int64(len(src)) - int64(length)

	mid, n := src.decodeNatural()
	if n == 0 {
		return nil, errInvalidMetadataIdentifier
	}
	if mid >= uint32(len(midDescriptions)) {
		return nil, errUnsupportedMetadataIdentifier
	}
	if p != nil {
		p(src[:n], "Metadata Identifier: %d (%s)\n", mid, midDescriptions[mid])
	}
	src = src[n:]

	switch mid {
	case midViewBox:
		if m.ViewBox.Min[0], src, err = decodeNumber(p, src, buffer.decodeCoordinate); err != nil {
			return nil, errInvalidViewBox
		}
		if m.ViewBox.Min[1], src, err = decodeNumber(p, src, buffer.decodeCoordinate); err != nil {
			return nil, errInvalidViewBox
		}
		if m.ViewBox.Max[0], src, err = decodeNumber(p, src, buffer.decodeCoordinate); err != nil {
			return nil, errInvalidViewBox
		}
		if m.ViewBox.Max[1], src, err = decodeNumber(p, src, buffer.decodeCoordinate); err != nil {
			return nil, errInvalidViewBox
		}
		if m.ViewBox.Min[0] > m.ViewBox.Max[0] || m.ViewBox.Min[1] > m.ViewBox.Max[1] ||
			isNaNOrInfinity(m.ViewBox.Min[0]) || isNaNOrInfinity(m.ViewBox.Min[1]) ||
			isNaNOrInfinity(m.ViewBox.Max[0]) || isNaNOrInfinity(m.ViewBox.Max[1]) {
			return nil, errInvalidViewBox
		}

	case midSuggestedPalette:
		panic("TODO")

	default:
		return nil, errUnsupportedMetadataIdentifier
	}

	if int64(len(src)) != lenSrcWant {
		return nil, errInconsistentMetadataChunkLength
	}
	return src, nil
}

// modeFunc is the decoding mode: whether we are decoding styling or drawing
// opcodes.
//
// It is a function type. The decoding loop calls this function to decode and
// execute the next opcode from the src buffer, returning the subsequent mode
// and the remaining source bytes.
type modeFunc func(dst Destination, p printer, src buffer) (modeFunc, buffer, error)

func decodeStyling(dst Destination, p printer, src buffer) (modeFunc, buffer, error) {
	switch opcode := src[0]; {
	case opcode < 0xc0:
		panic("TODO")
	case opcode < 0xc7:
		return decodeStartPath(dst, p, src, opcode)
	case opcode == 0xc7:
		panic("TODO")
	}
	return nil, nil, errUnsupportedStylingOpcode
}

func decodeStartPath(dst Destination, p printer, src buffer, opcode byte) (modeFunc, buffer, error) {
	adj := int(opcode & 0x07)
	if p != nil {
		p(src[:1], "Start path, filled with CREG[CSEL-%d]; M (absolute moveTo)\n", adj)
	}
	src = src[1:]

	x, src, err := decodeNumber(p, src, buffer.decodeCoordinate)
	if err != nil {
		return nil, nil, err
	}
	y, src, err := decodeNumber(p, src, buffer.decodeCoordinate)
	if err != nil {
		return nil, nil, err
	}

	if dst != nil {
		dst.StartPath(adj, x, y)
	}

	return decodeDrawing, src, nil
}

func decodeDrawing(dst Destination, p printer, src buffer) (mf modeFunc, src1 buffer, err error) {
	var coords [6]float32

	switch opcode := src[0]; {
	case opcode < 0xe0:
		op, nCoords, nReps := "", 0, 1+int(opcode&0x0f)
		switch opcode >> 4 {
		case 0x00, 0x01:
			op = "L (absolute lineTo)"
			nCoords = 2
			nReps = 1 + int(opcode&0x1f)
		case 0x02, 0x03:
			op = "l (relative lineTo)"
			nCoords = 2
			nReps = 1 + int(opcode&0x1f)
		case 0x04:
			op = "T (absolute smooth quadTo)"
			nCoords = 2
		case 0x05:
			op = "t (relative smooth quadTo)"
			nCoords = 2
		case 0x06:
			op = "Q (absolute quadTo)"
			nCoords = 4
		case 0x07:
			op = "q (relative quadTo)"
			nCoords = 4
		case 0x08:
			op = "S (absolute smooth cubeTo)"
			nCoords = 4
		case 0x09:
			op = "s (relative smooth cubeTo)"
			nCoords = 4
		case 0x0a:
			op = "C (absolute cubeTo)"
			nCoords = 6
		case 0x0b:
			op = "c (relative cubeTo)"
			nCoords = 6
		case 0x0c:
			op = "A (absolute arcTo)"
			nCoords = 0
		case 0x0d:
			op = "a (relative arcTo)"
			nCoords = 0
		}

		if p != nil {
			p(src[:1], "%s, %d reps\n", op, nReps)
		}
		src = src[1:]

		for i := 0; i < nReps; i++ {
			if p != nil && i != 0 {
				p(nil, "%s, implicit\n", op)
			}
			src, err = decodeCoordinates(coords[:nCoords], p, src)
			if err != nil {
				return nil, nil, err
			}

			if dst == nil {
				continue
			}
			switch op[0] {
			case 'L':
				dst.AbsLineTo(coords[0], coords[1])
				continue
			case 'l':
				dst.RelLineTo(coords[0], coords[1])
				continue
			case 'T':
				dst.AbsSmoothQuadTo(coords[0], coords[1])
				continue
			case 't':
				dst.RelSmoothQuadTo(coords[0], coords[1])
				continue
			case 'Q':
				dst.AbsQuadTo(coords[0], coords[1], coords[2], coords[3])
				continue
			case 'q':
				dst.RelQuadTo(coords[0], coords[1], coords[2], coords[3])
				continue
			case 'S':
				dst.AbsSmoothCubeTo(coords[0], coords[1], coords[2], coords[3])
				continue
			case 's':
				dst.RelSmoothCubeTo(coords[0], coords[1], coords[2], coords[3])
				continue
			case 'C':
				dst.AbsCubeTo(coords[0], coords[1], coords[2], coords[3], coords[4], coords[5])
				continue
			case 'c':
				dst.RelCubeTo(coords[0], coords[1], coords[2], coords[3], coords[4], coords[5])
				continue
			}

			// We have an absolute or relative arcTo.
			src, err = decodeCoordinates(coords[:3], p, src)
			if err != nil {
				return nil, nil, err
			}
			var largeArc, sweep bool
			largeArc, sweep, src, err = decodeArcToFlags(p, src)
			if err != nil {
				return nil, nil, err
			}
			src, err = decodeCoordinates(coords[4:6], p, src)
			if err != nil {
				return nil, nil, err
			}

			if op[0] == 'A' {
				dst.AbsArcTo(coords[0], coords[1], coords[2], largeArc, sweep, coords[4], coords[5])
			} else {
				dst.RelArcTo(coords[0], coords[1], coords[2], largeArc, sweep, coords[4], coords[5])
			}
		}

	case opcode == 0xe1:
		if p != nil {
			p(src[:1], "z (closePath); end path\n")
		}
		src = src[1:]
		if dst != nil {
			dst.ClosePathEndPath()
		}
		return decodeStyling, src, nil

	case opcode == 0xe2:
		if p != nil {
			p(src[:1], "z (closePath); M (absolute moveTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:2], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.ClosePathAbsMoveTo(coords[0], coords[1])
		}

	case opcode == 0xe3:
		if p != nil {
			p(src[:1], "z (closePath); m (relative moveTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:2], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.ClosePathRelMoveTo(coords[0], coords[1])
		}

	case opcode == 0xe6:
		if p != nil {
			p(src[:1], "H (absolute horizontal lineTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:1], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.AbsHLineTo(coords[0])
		}

	case opcode == 0xe7:
		if p != nil {
			p(src[:1], "h (relative horizontal lineTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:1], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.RelHLineTo(coords[0])
		}

	case opcode == 0xe8:
		if p != nil {
			p(src[:1], "V (absolute vertical lineTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:1], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.AbsVLineTo(coords[0])
		}

	case opcode == 0xe9:
		if p != nil {
			p(src[:1], "v (relative vertical lineTo)\n")
		}
		src = src[1:]
		src, err = decodeCoordinates(coords[:1], p, src)
		if err != nil {
			return nil, nil, err
		}
		if dst != nil {
			dst.RelVLineTo(coords[0])
		}

	default:
		return nil, nil, errUnsupportedDrawingOpcode
	}
	return decodeDrawing, src, nil
}

type decodeNumberFunc func(buffer) (float32, int)

func decodeNumber(p printer, src buffer, dnf decodeNumberFunc) (float32, buffer, error) {
	x, n := dnf(src)
	if n == 0 {
		return 0, nil, errInvalidNumber
	}
	if p != nil {
		p(src[:n], "    %+g\n", x)
	}
	return x, src[n:], nil
}

func decodeCoordinates(coords []float32, p printer, src buffer) (src1 buffer, err error) {
	for i := range coords {
		coords[i], src, err = decodeNumber(p, src, buffer.decodeCoordinate)
		if err != nil {
			return nil, err
		}
	}
	return src, nil
}

func decodeArcToFlags(p printer, src buffer) (bool, bool, buffer, error) {
	x, n := src.decodeNatural()
	if n == 0 {
		return false, false, nil, errInvalidNumber
	}
	if p != nil {
		p(src[:n], "    %#x (largeArc=%d, sweep=%d)\n", x, (x>>0)&0x01, (x>>1)&0x01)
	}
	return (x>>0)&0x01 != 0, (x>>1)&0x01 != 0, src[n:], nil
}
