package image

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"regexp"
	"strings"
	"sync"

	_ "golang.org/x/image/webp"
)

func IsImageUrl(url string) (bool, error) {
	resp, err := requestImage(url, http.MethodHead)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		return false, nil
	}
	return true, nil
}

func GetImageSizeFromUrl(url string) (width int, height int, err error) {
	isImage, err := IsImageUrl(url)
	if !isImage {
		return
	}
	resp, err := requestImage(url, http.MethodGet)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	img, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return
	}
	return img.Width, img.Height, nil
}

func GetImageFromUrl(url string) (mimeType string, data string, err error) {

	if strings.HasPrefix(url, "data:image/") {
		dataURLPattern := regexp.MustCompile(`data:image/([^;]+);base64,(.*)`)

		matches := dataURLPattern.FindStringSubmatch(url)
		if len(matches) == 3 && matches[2] != "" {
			mimeType = "image/" + matches[1]
			data = matches[2]
			return
		}

		err = errors.New("image base64 decode failed")
		return
	}

	isImage, err := IsImageUrl(url)
	if !isImage {
		if err == nil {
			err = errors.New("invalid image link")
		}
		return
	}
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	buffer := bytes.NewBuffer(nil)
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return
	}
	mimeType = resp.Header.Get("Content-Type")
	data = base64.StdEncoding.EncodeToString(buffer.Bytes())
	return
}

var (
	reg = regexp.MustCompile(`data:image/([^;]+);base64,`)
)

var readerPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Reader{}
	},
}

func GetImageSizeFromBase64(encoded string) (width int, height int, err error) {
	decoded, err := base64.StdEncoding.DecodeString(reg.ReplaceAllString(encoded, ""))
	if err != nil {
		return 0, 0, err
	}

	reader := readerPool.Get().(*bytes.Reader)
	defer readerPool.Put(reader)
	reader.Reset(decoded)

	img, _, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0, err
	}

	return img.Width, img.Height, nil
}

func GetImageSize(image string) (width int, height int, err error) {
	switch {
	case strings.HasPrefix(image, "data:image/"):
		return GetImageSizeFromBase64(image)
	case strings.HasPrefix(image, "http"):
		return GetImageSizeFromUrl(image)
	default:
		return 0, 0, errors.New("invalid file type, please view request interface")
	}
}
