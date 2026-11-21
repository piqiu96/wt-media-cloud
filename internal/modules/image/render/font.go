package render

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"golang.org/x/image/font/opentype"
	"io"
	"sync"
)

//go:embed fontdata/noto-cjk.zip
var fontArchive []byte
var loadedFonts sync.Map

type fontEntry struct {
	once  sync.Once
	value loadedFont
	err   error
}

type loadedFont struct {
	data []byte
	font *opentype.Font
}

func fontName(family, weight string) (string, error) {
	prefix := ""
	switch family {
	case "sans-serif", "system-ui":
		prefix = "NotoSansCJKsc"
	case "serif":
		prefix = "NotoSerifCJKsc"
	case "monospace":
		prefix = "NotoSansMonoCJKsc"
	default:
		return "", fmt.Errorf("字体不受支持：%s", family)
	}
	switch weight {
	case "400":
		return prefix + "-Regular.otf", nil
	case "700":
		return prefix + "-Bold.otf", nil
	default:
		return "", fmt.Errorf("字重不受支持")
	}
}
func loadFont(family, weight string) (loadedFont, error) {
	name, e := fontName(family, weight)
	if e != nil {
		return loadedFont{}, e
	}

	entryValue, _ := loadedFonts.LoadOrStore(name, &fontEntry{})
	entry := entryValue.(*fontEntry)
	entry.once.Do(func() {
		archive, e := zip.NewReader(bytes.NewReader(fontArchive), int64(len(fontArchive)))
		if e != nil {
			entry.err = e
			return
		}
		f, e := archive.Open(name)
		if e != nil {
			entry.err = e
			return
		}
		defer f.Close()
		data, e := io.ReadAll(f)
		if e != nil {
			entry.err = e
			return
		}
		parsed, e := opentype.Parse(data)
		if e != nil {
			entry.err = e
			return
		}
		entry.value = loadedFont{data: data, font: parsed}
	})
	return entry.value, entry.err
}

func FontData(family, weight string) ([]byte, error) {
	v, e := loadFont(family, weight)
	return v.data, e
}
