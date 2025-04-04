package main

// dicom jsonpath, pbcopy, tee

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"github.com/mpetavy/common"
	"github.com/mpetavy/go-dicom"
	"github.com/mpetavy/go-dicom/dicomtag"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	file      = flag.String("f", "", "File you want to parse")
	recursive = flag.Bool("r", false, "Recursive directory scan")
	extract   = flag.String("x", "", "Extract PixelData to defined directory")
	search    = flag.String("s", "", "Tag to search for with case insensitive lookup. Supports regexp")
)

var (
	JPEG_LIST = []string{
		"1.2.840.10008.1.2.4.50 JPEG Baseline (Process 1)",
		"1.2.840.10008.1.2.4.51 JPEG Baseline (Processes 2 & 4)",
		"1.2.840.10008.1.2.4.52 JPEG Extended (Processes 3 & 5) Retired",
		"1.2.840.10008.1.2.4.53 JPEG Spectral Selection, Nonhierarchical (Processes 6 & 8) Retired",
		"1.2.840.10008.1.2.4.54 JPEG Spectral Selection, Nonhierarchical (Processes 7 & 9) Retired",
		"1.2.840.10008.1.2.4.55 JPEG Full Progression, Nonhierarchical (Processes 10 & 12) Retired",
		"1.2.840.10008.1.2.4.56 JPEG Full Progression, Nonhierarchical (Processes 11 & 13) Retired",
		"1.2.840.10008.1.2.4.57 JPEG Lossless, Nonhierarchical (Processes 14)",
		"1.2.840.10008.1.2.4.58 JPEG Lossless, Nonhierarchical (Processes 15) Retired",
		"1.2.840.10008.1.2.4.59 JPEG Extended, Hierarchical (Processes 16 & 18) Retired",
		"1.2.840.10008.1.2.4.60 JPEG Extended, Hierarchical (Processes 17 & 19) Retired",
		"1.2.840.10008.1.2.4.61 JPEG Spectral Selection, Hierarchical (Processes 20 & 22) Retired",
		"1.2.840.10008.1.2.4.62 JPEG Spectral Selection, Hierarchical (Processes 21 & 23) Retired",
		"1.2.840.10008.1.2.4.63 JPEG Full Progression, Hierarchical (Processes 24 & 26) Retired",
		"1.2.840.10008.1.2.4.64 JPEG Full Progression, Hierarchical (Processes 25 & 27) Retired",
		"1.2.840.10008.1.2.4.65 JPEG Lossless, Nonhierarchical (Process 28) Retired",
		"1.2.840.10008.1.2.4.66 JPEG Lossless, Nonhierarchical (Process 29) Retired",
		"1.2.840.10008.1.2.4.70 JPEG Lossless, Nonhierarchical, First- Order Prediction",
		"1.2.840.10008.1.2.4.80 JPEG-LS Lossless Image Compression",
		"1.2.840.10008.1.2.4.81 JPEG-LS Lossy (Near- Lossless) Image Compression",
	}

	JPEG_2000_LIST = []string{
		"1.2.840.10008.1.2.4.90 JPEG 2000 Image Compression (Lossless Only)",
		"1.2.840.10008.1.2.4.91 JPEG 2000 Image Compression",
		"1.2.840.10008.1.2.4.92 JPEG 2000 Part 2 Multicomponent Image Compression (Lossless Only)",
		"1.2.840.10008.1.2.4.93 JPEG 2000 Part 2 Multicomponent Image Compression",
		"1.2.840.10008.1.2.4.94 JPIP Referenced",
		"1.2.840.10008.1.2.4.95 JPIP Referenced Deflate",
	}

	MPEG_LIST = []string{
		"1.2.840.10008.1.2.4.100 MPEG2 Main Profile Main Level",
		"1.2.840.10008.1.2.4.102 MPEG-4 AVC/H.264 High Profile / Level 4.1",
		"1.2.840.10008.1.2.4.103 MPEG-4 AVC/H.264 BD-compatible High Profile / Level 4.1",
	}
)

//go:embed go.mod
var resources embed.FS

func init() {
	common.Init("", "", "", "", "Tool to inspect DICOM file header and content", "", "", "", &resources, nil, nil, run, 0)
}

func find(l []string, e string) bool {
	p := common.IndexOf(l, e)

	if p == -1 {
		panic(fmt.Errorf("cannot find %+v in %+v", e, l))
	}

	return p >= 0
}

func isJpegTransferSyntax(st string) bool {
	return find(JPEG_LIST, st)

}

func isJpeg2000TransferSyntax(st string) bool {
	return find(JPEG_2000_LIST, st)

}

func isMpegTransferSyntax(st string) bool {
	return find(MPEG_LIST, st)
}

func processFile(path string) error {
	fmt.Printf("%s\n", path)

	data, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{DropPixelData: *extract == ""})
	if common.Error(err) {
		return err
	}

	if *search != "" {
		for _, elem := range data.Elements {
			tn, err := dicomtag.FindTagInfo(elem.Tag)
			if common.Error(err) {
				common.Error(err)
				continue
			}

			b, err := regexp.MatchString("(?i)"+*search, tn.Name)
			if common.Error(err) {
				return err
			}

			if b {
				fmt.Printf("%s: %s\n", tn.Name, elem.String())
			}
		}

		return nil
	}

	standardTags := []dicomtag.Tag{dicomtag.SOPClassUID, dicomtag.SOPInstanceUID, dicomtag.PatientName, dicomtag.TransferSyntaxUID, dicomtag.PatientID, dicomtag.Columns, dicomtag.Rows}

	if *extract != "" {
		standardTags = append(standardTags, dicomtag.PixelData)
	}

	tagNames := make([]string, 0)
	for _, tagInfo := range dicomtag.AllTags() {
		tagNames = append(tagNames, tagInfo.Name)
	}

	sort.Strings(tagNames)

	for _, tagName := range tagNames {
		elem, err := data.FindElementByName(tagName)
		if common.DebugError(err) {
			continue
		}

		if elem.Tag != dicomtag.PixelData {
			if !*common.FlagNoBanner {
				fmt.Printf("%-25s: %s\n", tagName, elem.String())
			} else {
				fmt.Printf("%s\n", elem.String())
			}
		} else {
			if *extract == "" {
				continue
			}

			data := elem.Value[0].(dicom.PixelDataInfo)

			buf := bytes.Buffer{}
			for _, frame := range data.Frames {
				_, err := buf.Write(frame)
				if common.Error(err) {
					return err
				}
			}

			mt, _ := common.DetectMimeType("", buf.Bytes())

			filename := filepath.Join(*extract, fmt.Sprintf("%s.%s", filepath.Base(path), mt.Ext))

			err := os.WriteFile(filename, buf.Bytes(), common.DefaultFileMode)
			if common.Error(err) {
				return err
			}

			fmt.Printf("%-25s:  %s: %s", tagName, mt.MimeType, filename)

			img, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
			if err == nil {
				fmt.Printf("width=%d, height=%d", img.Bounds().Max.X, img.Bounds().Max.Y)
			}

			fmt.Println()
		}
	}

	fmt.Println()

	return nil
}

func run() error {
	err := common.WalkFiles(*file, *recursive, true, func(file string, f os.FileInfo) error {
		if f.IsDir() {
			return nil
		}

		common.Error(processFile(file))

		return nil
	})
	if common.Error(err) {
		return err
	}

	return nil
}

func main() {
	common.Run([]string{"f"})
}
