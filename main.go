package main

import (
	"encoding/binary"
	"path/filepath"
	"fmt"
	"image"
	"image/draw"
	_ "image/png" // register the PNG format with the image package
	_ "image/gif" // register the GIF format with the image package
	_ "image/jpeg" // register the JPG format with the image package
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
)

const TEXTURE_2D_TYPE string = "Microsoft.Xna.Framework.Content.Texture2DReader, Microsoft.Xna.Framework.Graphics, Version=4.0.0.0, Culture=neutral, PublicKeyToken=842cf8be1de50553"
const HEADER_SIZE int = 3 + 1 + 1 + 1
const COMPRESSED_FILE_SIZE int = 4
const TYPE_READER_COUNT_SIZE int = 1
const TYPE_SIZE int = 2 + len(TEXTURE_2D_TYPE) + 4
const SHARED_RESOURCE_COUNT_SIZE int = 1
const OBJECT_HEADER_SIZE int = 21

const METADATA_SIZE int = HEADER_SIZE + COMPRESSED_FILE_SIZE + TYPE_READER_COUNT_SIZE + TYPE_SIZE + SHARED_RESOURCE_COUNT_SIZE + OBJECT_HEADER_SIZE

func writeByte(outfile *os.File, b uint8) {
	binary.Write(outfile, binary.LittleEndian, b)
}

func writeInt(outfile *os.File, i uint32) {
	binary.Write(outfile, binary.LittleEndian, i)
}

func write7BitEncodedInt(outfile *os.File, i int) {
	for i >= 0x80 {
		writeByte(outfile, uint8(i & 0xff))
		i >>= 7
	}
	writeByte(outfile, uint8(i))
}

func writeChars(outfile *os.File, arr []byte) {
	binary.Write(outfile, binary.LittleEndian, arr)
}

func writeString(outfile *os.File, s string) {
	arr := []byte(s)
	write7BitEncodedInt(outfile, len(arr))
	writeChars(outfile, arr)
}

func writePixels(outfile *os.File, pixels []byte) {
	for i := 0; i < len(pixels); i+=4 {
		if pixels[i + 3] == 0 { // fully transparent
			pixels[i + 0] = 0
			pixels[i + 1] = 0
			pixels[i + 2] = 0
		}
	}
	binary.Write(outfile, binary.LittleEndian, pixels)
}

func writeData(png image.Image, outfile *os.File) error {
	write7BitEncodedInt(outfile, 1)       // type-reader-count
	writeString(outfile, TEXTURE_2D_TYPE) // type-reader-name
	writeInt(outfile, 0)                  // reader version number
	write7BitEncodedInt(outfile, 0)       // shared-resource-count
	// writing the image pixel data
	writeByte(outfile, 1) // type id + 1 (referencing the TEXTURE_2D_TYPE)
	writeInt(outfile, 0)  // surface format; 0=color
	writeInt(outfile, uint32(png.Bounds().Max.X - png.Bounds().Min.X))
	writeInt(outfile, uint32(png.Bounds().Max.Y - png.Bounds().Min.Y))
	writeInt(outfile, 1) // mip count
	writeInt(outfile, imageSize(png)) // number of bytes in the image pixel data

	switch img := png.(type) {
	case *image.NRGBA:
		writePixels(outfile, img.Pix)
	default:
		fmt.Println("Wrong image format:", reflect.TypeOf(png))
		converted := image.NewNRGBA(image.Rect(0, 0, png.Bounds().Dx(), png.Bounds().Dy()))
		draw.Draw(converted, converted.Bounds(), png, png.Bounds().Min, draw.Src)
		writePixels(outfile, converted.Pix)
	}
	return nil
}

func imageSize(png image.Image) uint32 {
	b := png.Bounds()
	return uint32(4 * (b.Max.Y - b.Min.Y) * (b.Max.X - b.Min.X))
}

func uncompressedFileSize(png image.Image) uint32 {
	return uint32(METADATA_SIZE) + imageSize(png)
}

func pngToXnb(pngFile, xnbFile string, compressed, reach bool) error {
	log.Println("Converting", pngFile, "to", xnbFile)
	infile, err := os.Open(pngFile)
	defer infile.Close()
	if err != nil {
		return err
	}
	// Decode will figure out what type of image is in the file on its own.
	// We just have to be sure all the image packages we want are imported.
	png, _, err := image.Decode(infile)
	if err != nil {
		return err
	}
	outfile, err := os.Create(xnbFile)
	defer outfile.Close()
	if err != nil {
		return err
	}
	writeChars(outfile, []byte("XNB")) // format-identifier
	writeChars(outfile, []byte("w"))   // target-platform
	writeByte(outfile, uint8(5))       // xnb-format-version
	var flagBits uint8
	if !reach {
		flagBits |= 0x01
	}
	if compressed {
		flagBits |= 0x80
	}
	writeByte(outfile, flagBits) // flag-bits; 00=reach, 01=hiprofile, 80=compressed, 00=uncompressed
	if (compressed) {
		return fmt.Errorf("Compressed files are not supported")
	} else {
		writeInt(outfile, uncompressedFileSize(png))
		return writeData(png, outfile)
	}
	return nil
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	return fileInfo.IsDir(), err
}

func pngFileNameToXnb(pngFile string) string {
	extension := filepath.Ext(pngFile)
	return pngFile[0:len(pngFile)-len(extension)] + ".xnb"
}

func pngToDirectory(pngFile, xnbDir string, compressed, reach bool) error {
	xnbFile := xnbDir + "/" + filepath.Base(pngFileNameToXnb(pngFile))
	return pngToXnb(pngFile, xnbFile, compressed, reach)
}

func pngsToDirectory(pngDir, xnbDir string, compressed, reach bool) error {
	files, err := ioutil.ReadDir(pngDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		extension := strings.ToLower(filepath.Ext(f.Name()))
		if extension == ".png" {
			pngFile := pngDir + "/" + f.Name()
			err = pngToDirectory(pngFile, xnbDir, compressed, reach)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func execute(pngFile, xnbFile string, compressed, reach bool) error {
	pngFileIsDir, err := isDirectory(pngFile)
	if err != nil {
		return err
	}
	if pngFileIsDir {
		if len(xnbFile) == 0 {
			return fmt.Errorf("Must provide xnb_file when png_file is a directory!")
		}
		return pngsToDirectory(pngFile, xnbFile, compressed, reach)
	} else {
		if len(xnbFile) == 0 {
			return pngToXnb(pngFile, pngFileNameToXnb(pngFile), compressed, reach)
		}
		xnbFileIsDir, err := isDirectory(xnbFile)
		if err != nil {
			return err
		}
		if xnbFileIsDir {
			return pngToDirectory(pngFile, xnbFile, compressed, reach)
		}
		return pngToXnb(pngFile, xnbFile, compressed, reach)
	}

}

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Save images as XNB.")
		fmt.Println("Usage:", os.Args[0], "png_file [xnb_file]")
		os.Exit(1)
	}
	pngFile := os.Args[1]
	var xnbFile string
	if len(os.Args) >= 3 {
		xnbFile = os.Args[2]
	}
	err := execute(pngFile, xnbFile, false, true)
	if err != nil {
		log.Println(err)
		os.Exit(2)
	}
}