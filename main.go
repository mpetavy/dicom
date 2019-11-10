package main

import (
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"image"
	"os"
	"path/filepath"
	"regexp"

	"io/ioutil"

	"github.com/mpetavy/common"
	"github.com/mpetavy/go-dicom"
	"github.com/mpetavy/go-dicom/dicomtag"
)

var (
	file    = flag.String("f", "", "File you want to parse")
	extract = flag.Bool("x", false, "Extract PixelData")
	dim     = flag.Int("dim", 0, "Dimension of target")
	verbose = flag.Bool("v", false, "Show verbose information")
	search  = flag.String("s", "", "Tag to search for with case insensitive lookup. Supports regexp")
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

func init() {
	common.Init("1.0.3", "2017", "Tool to inspect DICOM files and export payload", "mpetavy", common.APACHE, false, nil, nil, run, 0)
}

func find(l []string, e string) bool {
	p, err := common.IndexOf(l, e)

	if err != nil {
		panic(err)
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

func fileWalker(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	// don't parse nested directories
	if fi.IsDir() {
		return nil
	}

	// not a DICOM file
	if filepath.Ext(fi.Name()) == ".dcm" {
		fmt.Printf("--------------------------------------\n")
		fmt.Printf("%s\n", fi.Name())
		processFile(path)
	}

	return err
}

func processImage(path string, dim int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	img, imgType, err := image.Decode(f)
	if err != nil {
		return err
	}

	fmt.Printf("bounds x=%d, y=%d\n", img.Bounds().Max.X, img.Bounds().Max.Y)
	fmt.Printf("%s\n", imgType)

	return nil
}

func processFile(path string) error {
	curdir, err := os.Getwd()
	if err != nil {
		return err
	}

	data, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{DropPixelData: !*extract})
	if err != nil {
		return err
	}

	if *search != "" {
		for _, elem := range data.Elements {
			tn, err := dicomtag.FindTagInfo(elem.Tag)
			if err != nil {
				common.Error(err)
				continue
			}

			bool, err := regexp.MatchString("(?i)"+*search, tn.Name)
			if err != nil {
				return err
			}

			if bool {
				fmt.Printf("%s: %s\n", tn.Name, elem.String())
			}
		}

		return nil
	}

	tags := []dicomtag.Tag{dicomtag.SOPClassUID, dicomtag.SOPInstanceUID, dicomtag.PatientName, dicomtag.TransferSyntaxUID, dicomtag.PatientID, dicomtag.Columns, dicomtag.Rows}

	n := 0
	for _, elem := range data.Elements {
		tn, err := dicomtag.FindTagInfo(elem.Tag)
		if err != nil {
			common.Error(err)
			continue
		}

		bool := *verbose
		if !bool {
			p, err := common.IndexOf(tags, elem.Tag)
			if err != nil {
				return err
			}

			bool = p >= 0
		}

		if bool {
			fmt.Printf("%-25s: %s\n", tn.Name, elem.String())
		}

		if *extract && elem.Tag == dicomtag.PixelData {
			data := elem.Value[0].(dicom.PixelDataInfo)
			for _, frame := range data.Frames {
				path := fmt.Sprintf("%s.%d.jpg", filepath.Join(curdir, filepath.Base(path)), n)
				n++
				ioutil.WriteFile(path, frame, common.DefaultFileMode)
			}
		}
	}

	return nil
}

func run() error {
	b, err := common.FileExists(*file)
	if err != nil {
		return err
	}

	if b {
		isDir, err := common.IsDirectory(*file)
		if err != nil {
			return err
		}

		if !isDir {
			err := processFile(*file)
			if err != nil {
				return err
			}
		} else {
			err := filepath.Walk(*file, fileWalker)
			if err != nil {
				return err
			}
		}
	} else {
		common.Error(errors.Wrap(&common.ErrFileNotFound{FileName: *file}, *file))
	}

	return nil
}

func main() {
	defer common.Done()

	common.Run([]string{"f"})
}
