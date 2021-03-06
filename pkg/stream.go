package main

import (
	"fmt"
	"github.com/mholt/archiver/v3"
	"io"
	"os"
	"path/filepath"
	"sync"
)

func generateTarGzWriter(closer io.Writer) (*archiver.TarGz, error) {
	t := archiver.Tar{
		MkdirAll:               true,
		ContinueOnError:        false,
		OverwriteExisting:      true,
		ImplicitTopLevelFolder: false,
	}
	tgz := archiver.TarGz{
		Tar:              &t,
		CompressionLevel: 5,
		SingleThreaded:   false,
	}

	err := tgz.Create(closer)
	return &tgz, err
}

func generateZipWriter(writer io.Writer) (*archiver.Zip, error) {
	z := archiver.Zip{
		CompressionLevel:       5,
		OverwriteExisting:      true,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ImplicitTopLevelFolder: false,
		ContinueOnError:        false,
	}
	err := z.Create(writer)
	return &z, err
}

func generateLocalFileZipWriter(zp string) (*archiver.Zip, *os.File, error) {
	zf, err := os.Create(zp)
	if err != nil {
		return nil, nil, err
	}

	z, err := generateZipWriter(WriteFakeCloser{zf})
	if err != nil {
		return nil, nil, err
	}
	return z, zf, nil
}

func generateLocalFileTarGzWriter(tp string) (*archiver.TarGz, *os.File, error) {
	tf, err := os.Create(tp)
	if err != nil {
		return nil, nil, err
	}

	tfw, err := generateTarGzWriter(WriteFakeCloser{tf})
	if err != nil {
		return nil, nil, err
	}
	return tfw, tf, nil
}

func walkAndStream(srcPaths []string, writers []archiver.Writer, wg *sync.WaitGroup, errs chan<- error, close bool, closePipe io.WriteCloser) {
	defer wg.Done()
	if close {
		defer closePipe.Close()
		for _, writer := range writers {
			defer writer.Close()
		}
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsDir() {
			return nil
		}
		if len(path) == 0 {
			return nil
		}

		cPath := cleanPath(path)
		//fmt.Printf("%v -> %v\n", path, cPath)

		for _, writer := range writers {
			fr, err := os.Open(path)
			if err != nil {
				return err
			}
			fileInfo, err := fr.Stat()
			if err != nil {
				return err
			}
			arcFileInfo := archiver.File{FileInfo: archiver.FileInfo{
				FileInfo: fileInfo, CustomName: cPath},
				ReadCloser: fr,
			}
			err = writer.Write(arcFileInfo)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, subPath := range srcPaths {
		matches, err := filepath.Glob(subPath)
		if err != nil {
			errs <- err
			return
		}
		if matches != nil {
			for _, match := range matches {
				if err := filepath.Walk(match, walkFn); err != nil{
					errs <- err
					return
				}
				fmt.Println("path " + fmt.Sprint(match) + " streamed to " + fmt.Sprint(len(writers)) + " writer(s)")
			}
		}
	}

}
