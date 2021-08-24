package flac

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type AnalyzeError int

func (e AnalyzeError) Error() string {
	return errors[int(e)]
}

const (
	WRONG_MARKER AnalyzeError = iota
	NOT_FIND_EQUAL_SIGN
)

var errors = []string{
	WRONG_MARKER:        "marker of the file is not 'fLaC'",
	NOT_FIND_EQUAL_SIGN: "not find '='",
}

type NotFLACFormatError struct {
	When string
	Err  error
}

func (e NotFLACFormatError) Error() string {
	return "not flac format: " + e.When + ": " + e.Err.Error()
}

func (e NotFLACFormatError) Unwrap() error {
	return e.Err
}

type RepackError struct {
	When string
	Err  error
}

func (e RepackError) Error() string {
	return "repack flac failed: " + e.When + ": " + e.Err.Error()
}

func (e RepackError) Unwrap() error {
	return e.Err
}

type Steam struct {
	Marker         [4]byte
	MetadataBlock  []Metadata
	VorbisComments Vorbis
	StreamInfo     Metadata
	Frame          []byte
}

type Metadata struct {
	BlockType byte
	Data      []byte
}

type Vorbis struct {
	VendorString    string
	UserCommentList map[string]string
}

func Analyze(path string) (steam Steam, err error) {
	f, err := os.Open(path)
	if err != nil {
		return steam, err
	}
	defer func() {
		err = f.Close()
	}()

	steam.Marker, err = readMarker(f)
	if err != nil {
		return
	}
	if steam.Marker != [4]byte{'f', 'L', 'a', 'C'} {
		return steam, NotFLACFormatError{"read marker", WRONG_MARKER}
	}

	// Steam info must be present as the first metadata block in the stream.
	var last bool
	steam.StreamInfo, last, err = readMetadata(f)
	if err != nil {
		return
	}
	if !last {
		steam.MetadataBlock, steam.VorbisComments, err = readMetadataBlock(f)
		if err != nil {
			return
		}
	}

	steam.Frame, err = ioutil.ReadAll(f)
	if err != nil {
		return
	}

	return
}

func readMarker(f *os.File) (marker [4]byte, err error) {
	_, err = f.Read(marker[:])
	if err != nil {
		return marker, NotFLACFormatError{"read marker", err}
	}
	return marker, nil
}

func readMetadataBlock(f *os.File) (metadataBlock []Metadata, vorbisComments Vorbis, err error) {
	metadataBlock = make([]Metadata, 0, 6)
	for {
		var metadata, last = Metadata{}, false
		metadata, last, err = readMetadata(f)
		if err != nil {
			return
		}
		switch metadata.BlockType {
		case 4:
			vorbisComments, err = parseVorbis(metadata.Data)
			if err != nil {
				return
			}
		default:
			metadataBlock = append(metadataBlock, metadata)
		}
		if last {
			return metadataBlock, vorbisComments, nil
		}
	}
}

func readMetadata(f *os.File) (metadata Metadata, last bool, err error) {
	var header = make([]byte, 4)
	_, err = f.Read(header)
	if err != nil {
		return Metadata{}, false, NotFLACFormatError{"read metadata header", err}
	}
	last = header[0]>>7 == 1
	metadata.BlockType = header[0] & 0b0111_1111
	var length = int(header[1])<<16 + int(header[2])<<8 + int(header[3])
	metadata.Data = make([]byte, length)
	_, err = f.Read(metadata.Data)
	if err != nil {
		return Metadata{}, false, NotFLACFormatError{"read metadata data", err}
	}
	return
}

func parseVorbis(data []byte) (vorbisComments Vorbis, err error) {
	var r = bytes.NewReader(data)
	vorbisComments.VendorString, err = readData(r)
	if err != nil {
		return Vorbis{}, NotFLACFormatError{"read vorbis vendor", err}
	}
	userCommentListLength, err := readLength(r)
	if err != nil {
		return Vorbis{}, NotFLACFormatError{"read vorbis user comment list length", err}
	}
	vorbisComments.UserCommentList = make(map[string]string, userCommentListLength)
	for i := 0; i < userCommentListLength; i++ {
		var userComment, err = readData(r)
		if err != nil {
			return Vorbis{}, NotFLACFormatError{"read vorbis user comment data", err}
		}
		k, v, err := analyzeUserComment(userComment)
		if err != nil {
			return Vorbis{}, NotFLACFormatError{"analyze vorbis user comment data", err}
		}
		vorbisComments.UserCommentList[k] = v
	}
	return
}

func readLength(r *bytes.Reader) (length int, err error) {
	var data = make([]byte, 4)
	_, err = r.Read(data)
	if err != nil {
		return
	}
	return int(binary.LittleEndian.Uint32(data)), nil
}

func readData(r *bytes.Reader) (value string, err error) {
	length, err := readLength(r)
	if err != nil {
		return
	}
	var data = make([]byte, length)
	_, err = r.Read(data)
	return string(data), err
}

func analyzeUserComment(data string) (key, value string, err error) {
	var equalSignIndex = strings.Index(data, "=")
	if equalSignIndex == -1 {
		return "", "", NOT_FIND_EQUAL_SIGN
	}
	return data[:equalSignIndex], data[equalSignIndex+1:], nil
}

func (s *Steam) RepackFile(path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return RepackError{"Create file: " + path, err}
	}
	defer func() {
		var err2 = f.Close()
		if err2 != nil {
			err = RepackError{"Close file: " + path, err2}
		}
	}()
	return s.Repack(f)
}

func (s *Steam) Repack(w io.Writer) error {
	var err = s.repackMarker(w)
	if err != nil {
		return err
	}

	err = s.repackStreamInfo(w)
	if err != nil {
		return err
	}

	err = s.repackVorbisComment(w)
	if err != nil {
		return err
	}

	err = s.repackMetadataBlock(w)
	if err != nil {
		return err
	}

	err = s.repackFrame(w)
	if err != nil {
		return err
	}

	return nil
}

func (s *Steam) repackMarker(w io.Writer) error {
	var _, err = w.Write(s.Marker[:])
	if err != nil {
		return RepackError{"repack marker", err}
	}
	return nil
}

func (s *Steam) repackStreamInfo(w io.Writer) error {
	var _, err = w.Write([]byte{s.StreamInfo.BlockType})
	if err != nil {
		return RepackError{"repack stream info block type", err}
	}
	var length = make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(s.StreamInfo.Data)))
	_, err = w.Write(length[1:])
	if err != nil {
		return RepackError{"repack stream info data length", err}
	}
	_, err = w.Write(s.StreamInfo.Data)
	if err != nil {
		return RepackError{"repack stream info data", err}
	}
	return nil
}

func (s *Steam) repackVorbisComment(w io.Writer) error {
	var _, err = w.Write([]byte{0b00000100})
	if err != nil {
		return RepackError{"repack vorbis comments block type", err}
	}

	var length = 4 + len(s.VorbisComments.VendorString) + 4
	for k, v := range s.VorbisComments.UserCommentList {
		var userComment = k + "=" + v
		length += 4 + len(userComment)
	}

	var lengthData = make([]byte, 4)
	binary.BigEndian.PutUint32(lengthData, uint32(length))
	_, err = w.Write(lengthData[1:])
	if err != nil {
		return RepackError{"repack vorbis comment data length", err}
	}

	var venderLengthData = make([]byte, 4)
	binary.LittleEndian.PutUint32(venderLengthData, uint32(len(s.VorbisComments.VendorString)))
	_, err = w.Write(venderLengthData)
	if err != nil {
		return RepackError{"repack vorbis vender length", err}
	}
	_, err = w.Write([]byte(s.VorbisComments.VendorString))
	if err != nil {
		return RepackError{"repack vorbis vender data", err}
	}
	var userCommentListLengthData = make([]byte, 4)
	binary.LittleEndian.PutUint32(userCommentListLengthData, uint32(len(s.VorbisComments.UserCommentList)))
	_, err = w.Write(userCommentListLengthData)
	if err != nil {
		return RepackError{"repack vorbis user comment list length", err}
	}
	for k, v := range s.VorbisComments.UserCommentList {
		var userComment = k + "=" + v
		var lengthData = make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthData, uint32(len(userComment)))
		_, err = w.Write(lengthData)
		if err != nil {
			return RepackError{"repack vorbis user comment length", err}
		}
		_, err = w.Write([]byte(userComment))
		if err != nil {
			return RepackError{"repack vorbis user comment data", err}
		}
	}
	return nil
}

func (s *Steam) repackMetadataBlock(w io.Writer) (err error) {
	for i, metadata := range s.MetadataBlock {
		if i == len(s.MetadataBlock)-1 {
			_, err = w.Write([]byte{metadata.BlockType | 0b1000_0000})
		} else {
			_, err = w.Write([]byte{metadata.BlockType})
		}
		if err != nil {
			return RepackError{"repack metadata block type", err}
		}
		var length = make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(metadata.Data)))
		_, err = w.Write(length[1:])
		if err != nil {
			return RepackError{"repack metadata block data length", err}
		}
		_, err = w.Write(metadata.Data)
		if err != nil {
			return RepackError{"repack metadata block data", err}
		}
	}
	return nil
}

func (s *Steam) repackFrame(w io.Writer) error {
	var _, err = w.Write(s.Frame)
	return RepackError{"repack frame", err}
}
