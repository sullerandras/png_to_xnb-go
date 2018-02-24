package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/png" // register the PNG format with the image package
	_ "image/gif" // register the GIF format with the image package
	_ "image/jpeg" // register the JPG format with the image package
	"log"
	"os"
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
		binary.Write(outfile, binary.LittleEndian, img.Pix)
	default:
		converted := image.NewNRGBA(image.Rect(0, 0, png.Bounds().Dx(), png.Bounds().Dy()))
		draw.Draw(converted, converted.Bounds(), png, png.Bounds().Min, draw.Src)
		binary.Write(outfile, binary.LittleEndian, converted.Pix)
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

func main() {
	err := pngToXnb(os.Args[1], os.Args[2], false, true)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}