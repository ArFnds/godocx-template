package internal

import (
	"archive/zip"
	"io"
	"slices"

	_ "golang.org/x/text/encoding/charmap"
)

func ZipGetText(z *zip.ReadCloser, filename string) (string, error) {
	rc, err := z.Open(filename)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ZipClone(reader *zip.ReadCloser, writer *zip.Writer, except []string) error {
	for _, zipFile := range reader.File {
		if slices.Contains(except, zipFile.Name) {
			continue
		}
		rc, err := zipFile.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		w, err := writer.Create(zipFile.Name)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func ZipSet(z *zip.Writer, filename string, data []byte) error {
	w, err := z.Create(filename)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
